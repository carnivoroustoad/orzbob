package tmux

import (
	"bufio"
	"bytes"
	"orzbob/log"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
)

const ProgramClaude = "claude"

const ProgramAider = "aider"

// TmuxSession represents a managed tmux session
type TmuxSession struct {
	// Initialized by NewTmuxSession
	//
	// The name of the tmux session and the sanitized name used for tmux commands.
	Name          string
	sanitizedName string
	program       string

	// Initialized by Start or Restore
	//
	// ptmx is a PTY is running the tmux attach command. This can be resized to change the
	// stdout dimensions of the tmux pane. On detach, we close it and set a new one.
	// This should never be nil.
	ptmx *os.File
	// monitor monitors the tmux pane content and sends signals to the UI when it's status changes
	monitor *statusMonitor

	// Initialized by Attach
	// Deinitilaized by Detach
	//
	// Channel to be closed at the very end of detaching. Used to signal callers.
	attachCh chan struct{}
	// While attached, we use some goroutines to manage the window size and stdin/stdout. This stuff
	// is used to terminate them on Detach. We don't want them to outlive the attached window.
	ctx    context.Context
	cancel func()
	wg     *sync.WaitGroup
}

const TmuxPrefix = "orzbob_"

var whiteSpaceRegex = regexp.MustCompile(`\s+`)

func toClaudeSquadTmuxName(str string) string {
	str = whiteSpaceRegex.ReplaceAllString(str, "")
	str = strings.ReplaceAll(str, ".", "_") // tmux replaces all . with _
	return fmt.Sprintf("%s%s", TmuxPrefix, str)
}

func NewTmuxSession(name string, program string) *TmuxSession {
	return &TmuxSession{
		Name:          name,
		sanitizedName: toClaudeSquadTmuxName(name),
		program:       program,
	}
}

// Start creates and starts a new tmux session, then attaches to it. Program is the command to run in
// the session (ex. claude). workdir is the git worktree directory.
func (t *TmuxSession) Start(program string, workDir string) error {
	// Check if the session already exists
	if DoesSessionExist(t.sanitizedName) {
		return fmt.Errorf("tmux session already exists: %s", t.sanitizedName)
	}

	// Create a new detached tmux session and start claude in it
	cmd := exec.Command("tmux", "new-session", "-d", "-s", t.sanitizedName, "-c", workDir, program)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		// Cleanup any partially created session if any exists.
		if DoesSessionExist(t.sanitizedName) {
			cleanupCmd := exec.Command("tmux", "kill-session", "-t", t.sanitizedName)
			if cleanupErr := cleanupCmd.Run(); cleanupErr != nil {
				err = fmt.Errorf("%v (cleanup error: %v)", err, cleanupErr)
			}
		}
		return fmt.Errorf("error starting tmux session: %w", err)
	}

	// We need to close the ptmx, but we shouldn't close it before the command above finishes.
	// So, we poll for completion before closing.
	timeout := time.After(2 * time.Second)
	// Poll with increasing intervals to reduce CPU usage
	pollInterval := time.Millisecond * 5
	for !DoesSessionExist(t.sanitizedName) {
		select {
		case <-timeout:
			// Cleanup on window size update failure
			if cleanupErr := t.Close(); cleanupErr != nil {
				err = fmt.Errorf("%v (cleanup error: %v)", err, cleanupErr)
			}
			return fmt.Errorf("timed out waiting for tmux session %s: %v", t.sanitizedName, err)
		default:
			time.Sleep(pollInterval)
			// Exponential backoff with a cap at 50ms
			if pollInterval < 50*time.Millisecond {
				pollInterval += pollInterval / 2
			}
		}
	}
	ptmx.Close()

	err = t.Restore()
	if err != nil {
		if cleanupErr := t.Close(); cleanupErr != nil {
			err = fmt.Errorf("%v (cleanup error: %v)", err, cleanupErr)
		}
		return fmt.Errorf("error restoring tmux session: %w", err)
	}

	if program == ProgramClaude || strings.HasPrefix(program, ProgramAider) {
		searchString := "Do you trust the files in this folder?"
		tapFunc := t.TapEnter
		iterations := 5
		if program != ProgramClaude {
			searchString = "Open documentation url for more info"
			tapFunc = t.TapDAndEnter
			iterations = 10 // Aider takes longer to start :/
		}
		// Deal with "do you trust the files" screen by sending an enter keystroke.
		// Initial check with shorter wait time
		time.Sleep(100 * time.Millisecond)
		var content string
		for i := 0; i < iterations; i++ {
			content, err = t.CapturePaneContent()
			if err != nil {
				log.ErrorLog.Printf("could not check 'do you trust the files screen': %v", err)
			}
			if strings.Contains(content, searchString) {
				if err := tapFunc(); err != nil {
					log.ErrorLog.Printf("could not tap enter on trust screen: %v", err)
				}
				break
			}
			// Adaptive waiting - less time for first iterations, more for later ones
			waitTime := time.Duration(100+i*20) * time.Millisecond
			if waitTime > 200*time.Millisecond {
				waitTime = 200 * time.Millisecond
			}
			time.Sleep(waitTime)
		}
	}
	return nil
}

// Restore attaches to an existing session and restores the window size
func (t *TmuxSession) Restore() error {
	ptmx, err := pty.Start(exec.Command("tmux", "attach-session", "-t", t.sanitizedName))
	if err != nil {
		return fmt.Errorf("error opening PTY: %w", err)
	}
	t.ptmx = ptmx
	t.monitor = newStatusMonitor()
	return nil
}

type statusMonitor struct {
	// Store hashes to save memory.
	prevOutputHash []byte
	// Cache pane content and last check time to reduce calls
	cachedContent string
	lastCheck     time.Time
	cacheValidity time.Duration
}

func newStatusMonitor() *statusMonitor {
	return &statusMonitor{
		cacheValidity: 100 * time.Millisecond,
	}
}

// hash hashes the bytes directly using a faster non-cryptographic hash
func (m *statusMonitor) hash(s string) []byte {
	// Continue using sha256 for consistency but optimize the process
	h := sha256.New()
	// Use WriterString to avoid allocation of []byte from string
	io.WriteString(h, s)
	return h.Sum(nil)
}

// TapEnter sends an enter keystroke to the tmux pane.
func (t *TmuxSession) TapEnter() error {
	_, err := t.ptmx.Write([]byte{0x0D})
	if err != nil {
		return fmt.Errorf("error sending enter keystroke to PTY: %w", err)
	}
	return nil
}

// TapDAndEnter sends 'D' followed by an enter keystroke to the tmux pane.
func (t *TmuxSession) TapDAndEnter() error {
	_, err := t.ptmx.Write([]byte{0x44, 0x0D})
	if err != nil {
		return fmt.Errorf("error sending enter keystroke to PTY: %w", err)
	}
	return nil
}

func (t *TmuxSession) SendKeys(keys string) error {
	_, err := t.ptmx.Write([]byte(keys))
	return err
}

// HasUpdated checks if the tmux pane content has changed since the last tick. It also returns true if
// the tmux pane has a prompt for aider or claude code.
func (t *TmuxSession) HasUpdated() (updated bool, hasPrompt bool) {
	// Check if we can use cached content to avoid frequent captures
	if time.Since(t.monitor.lastCheck) < t.monitor.cacheValidity {
		// Reuse cached content for prompt check
		if t.program == ProgramClaude {
			hasPrompt = strings.Contains(t.monitor.cachedContent, "No, and tell Claude what to do differently")
		} else if strings.HasPrefix(t.program, ProgramAider) {
			hasPrompt = strings.Contains(t.monitor.cachedContent, "(Y)es/(N)o/(D)on't ask again")
		}
		return false, hasPrompt
	}

	content, err := t.CapturePaneContent()
	if err != nil {
		log.ErrorLog.Printf("error capturing pane content in status monitor: %v", err)
		return false, false
	}

	// Update cache and timestamp
	t.monitor.cachedContent = content
	t.monitor.lastCheck = time.Now()

	// Only set hasPrompt for claude and aider. Use these strings to check for a prompt.
	if t.program == ProgramClaude {
		hasPrompt = strings.Contains(content, "No, and tell Claude what to do differently")
	} else if strings.HasPrefix(t.program, ProgramAider) {
		hasPrompt = strings.Contains(content, "(Y)es/(N)o/(D)on't ask again")
	}

	// Only compute hash once
	newHash := t.monitor.hash(content)
	if !bytes.Equal(newHash, t.monitor.prevOutputHash) {
		t.monitor.prevOutputHash = newHash
		return true, hasPrompt
	}
	return false, hasPrompt
}

func (t *TmuxSession) Attach() (chan struct{}, error) {
	t.attachCh = make(chan struct{})

	t.wg = &sync.WaitGroup{}
	t.wg.Add(1)
	t.ctx, t.cancel = context.WithCancel(context.Background())

	// The first goroutine should terminate when the ptmx is closed. We use the
	// waitgroup to wait for it to finish.
	// Use buffered I/O for better performance
	go func() {
		defer t.wg.Done()
		// Use buffered I/O for better throughput
		bufWriter := bufio.NewWriterSize(os.Stdout, 4096)
		defer bufWriter.Flush()
		
		buf := make([]byte, 4096) // Larger buffer for more efficient reads
		for {
			nr, err := t.ptmx.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.ErrorLog.Printf("error reading from ptmx: %v", err)
				}
				break
			}
			if nr > 0 {
				_, err = bufWriter.Write(buf[:nr])
				if err != nil {
					log.ErrorLog.Printf("error writing to stdout: %v", err)
					break
				}
				// Flush after each write to maintain responsiveness
				err = bufWriter.Flush()
				if err != nil {
					log.ErrorLog.Printf("error flushing buffer: %v", err)
					break
				}
			}
		}
	}()

	// The 2nd one returns when you press escape to Detach. It doesn't need to be
	// in the waitgroup because is the goroutine doing the Detaching; it waits for
	// all the other ones.
	go func() {
		// Close the channel after 50ms
		timeoutCh := make(chan struct{})
		go func() {
			time.Sleep(50 * time.Millisecond)
			close(timeoutCh)
		}()

		// Read input from stdin and check for Ctrl+q
		// Use buffered I/O for better performance
		bufReader := bufio.NewReaderSize(os.Stdin, 1024)
		buf := make([]byte, 1024) // Larger buffer for more efficient reads
		
		for {
			nr, err := bufReader.Read(buf)
			if err != nil {
				if err == io.EOF {
					break
				}
				continue
			}

			// Nuke the first bytes of stdin, up to 64, to prevent tmux from reading it.
			// When we attach, there tends to be terminal control sequences like ?[?62c0;95;0c or
			// ]10;rgb:f8f8f8. The control sequences depend on the terminal (warp vs iterm). We should use regex ideally
			// but this works well for now. Log this for debugging.
			//
			// There seems to always be control characters, but I think it's possible for there not to be. The heuristic
			// here can be: if there's characters within 50ms, then assume they are control characters and nuke them.
			select {
			case <-timeoutCh:
			default:
				log.InfoLog.Printf("nuked first stdin: %s", buf[:nr])
				continue
			}

			// Check for Ctrl+q (ASCII 17)
			if nr == 1 && buf[0] == 17 {
				// Detach from the session
				t.Detach()
				return
			}

			// Forward other input to tmux
			_, _ = t.ptmx.Write(buf[:nr])
		}
	}()

	t.monitorWindowSize()
	return t.attachCh, nil
}

// Detach disconnects from the current tmux session. It panics if detaching fails. At the moment, there's no
// way to recover from a failed detach.
func (t *TmuxSession) Detach() {
	// TODO: control flow is a bit messy here. If there's an error,
	// I'm not sure if we get into a bad state. Needs testing.
	defer func() {
		close(t.attachCh)
		t.attachCh = nil
		t.cancel = nil
		t.ctx = nil
		t.wg = nil
	}()

	// Close the attached pty session.
	err := t.ptmx.Close()
	if err != nil {
		// This is a fatal error. We can't detach if we can't close the PTY. It's better to just panic and have the
		// user re-invoke the program than to ruin their terminal pane.
		msg := fmt.Sprintf("error closing attach pty session: %v", err)
		log.ErrorLog.Println(msg)
		panic(msg)
	}
	// Attach goroutines should die on EOF due to the ptmx closing. Call
	// t.Restore to set a new t.ptmx.
	if err = t.Restore(); err != nil {
		// This is a fatal error. Our invariant that a started TmuxSession always has a valid ptmx is violated.
		msg := fmt.Sprintf("error closing attach pty session: %v", err)
		log.ErrorLog.Println(msg)
		panic(msg)
	}

	// Cancel goroutines created by Attach.
	t.cancel()
	t.wg.Wait()
}

// Close terminates the tmux session and cleans up resources
func (t *TmuxSession) Close() error {
	var errs []error

	if t.ptmx != nil {
		if err := t.ptmx.Close(); err != nil {
			errs = append(errs, fmt.Errorf("error closing PTY: %w", err))
		}
		t.ptmx = nil
	}

	cmd := exec.Command("tmux", "kill-session", "-t", t.sanitizedName)
	if err := cmd.Run(); err != nil {
		errs = append(errs, fmt.Errorf("error killing tmux session: %w", err))
	}

	if len(errs) == 0 {
		return nil
	}
	if len(errs) == 1 {
		return errs[0]
	}

	errMsg := "multiple errors occurred during cleanup:"
	for _, err := range errs {
		errMsg += "\n  - " + err.Error()
	}
	return errors.New(errMsg)
}

// SetDetachedSize set the width and height of the session while detached. This makes the
// tmux output conform to the specified shape.
func (t *TmuxSession) SetDetachedSize(width, height int) error {
	return t.updateWindowSize(width, height)
}

// updateWindowSize updates the window size of the PTY.
func (t *TmuxSession) updateWindowSize(cols, rows int) error {
	return pty.Setsize(t.ptmx, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
		X:    0,
		Y:    0,
	})
}

// sessionExistenceCache provides a short-lived cache for session existence checks
var sessionExistenceCache = struct {
	mutex       sync.Mutex
	exists      map[string]bool
	timestamps  map[string]time.Time
	cachePeriod time.Duration
}{
	exists:      make(map[string]bool),
	timestamps:  make(map[string]time.Time),
	cachePeriod: 500 * time.Millisecond,
}

// DoesSessionExist checks if a tmux session exists with caching for better performance
func DoesSessionExist(name string) bool {
	// Check cache first
	sessionExistenceCache.mutex.Lock()
	defer sessionExistenceCache.mutex.Unlock()

	if ts, ok := sessionExistenceCache.timestamps[name]; ok {
		if time.Since(ts) < sessionExistenceCache.cachePeriod {
			return sessionExistenceCache.exists[name]
		}
	}

	// Cache miss or expired, check for real
	// Using "-t name" does a prefix match, which is wrong. `-t=` does an exact match.
	existsCmd := exec.Command("tmux", "has-session", fmt.Sprintf("-t=%s", name))
	exists := existsCmd.Run() == nil

	// Update cache
	sessionExistenceCache.exists[name] = exists
	sessionExistenceCache.timestamps[name] = time.Now()

	return exists
}

func (t *TmuxSession) DoesSessionExist() bool {
	return DoesSessionExist(t.sanitizedName)
}

// paneContentCache provides a short-lived cache for pane content
var paneContentCache = struct {
	mutex       sync.Mutex
	content     map[string]string
	timestamps  map[string]time.Time
	cachePeriod time.Duration
}{
	content:     make(map[string]string),
	timestamps:  make(map[string]time.Time),
	cachePeriod: 100 * time.Millisecond,
}

// CapturePaneContent captures the content of the tmux pane with caching
func (t *TmuxSession) CapturePaneContent() (string, error) {
	// Check cache first
	paneContentCache.mutex.Lock()
	if ts, ok := paneContentCache.timestamps[t.sanitizedName]; ok {
		if time.Since(ts) < paneContentCache.cachePeriod {
			content := paneContentCache.content[t.sanitizedName]
			paneContentCache.mutex.Unlock()
			return content, nil
		}
	}
	paneContentCache.mutex.Unlock()

	// Cache miss or expired, capture for real
	// Add -e flag to preserve escape sequences (ANSI color codes)
	cmd := exec.Command("tmux", "capture-pane", "-p", "-e", "-J", "-t", t.sanitizedName)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("error capturing pane content: %v", err)
	}
	
	// Update cache
	content := string(output)
	paneContentCache.mutex.Lock()
	paneContentCache.content[t.sanitizedName] = content
	paneContentCache.timestamps[t.sanitizedName] = time.Now()
	paneContentCache.mutex.Unlock()
	
	return content, nil
}

// CapturePaneContentWithOptions captures the pane content with additional options
// start and end specify the starting and ending line numbers (use "-" for the start/end of history)
func (t *TmuxSession) CapturePaneContentWithOptions(start, end string) (string, error) {
	// Add -e flag to preserve escape sequences (ANSI color codes)
	cmd := exec.Command("tmux", "capture-pane", "-p", "-e", "-J", "-S", start, "-E", end, "-t", t.sanitizedName)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to capture tmux pane content with options: %v", err)
	}
	return string(output), nil
}

// CleanupSessions kills all tmux sessions that start with "session-"
func CleanupSessions() error {
	// First try to list sessions
	cmd := exec.Command("tmux", "ls")
	output, err := cmd.Output()

	// If there's an error and it's because no server is running, that's fine
	// Exit code 1 typically means no sessions exist
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil // No sessions to clean up
		}
		return fmt.Errorf("failed to list tmux sessions: %v", err)
	}

	re := regexp.MustCompile(fmt.Sprintf(`%s.*:`, TmuxPrefix))
	matches := re.FindAllString(string(output), -1)
	for i, match := range matches {
		matches[i] = match[:strings.Index(match, ":")]
	}

	for _, match := range matches {
		log.InfoLog.Printf("cleaning up session: %s", match)
		cmd := exec.Command("tmux", "kill-session", "-t", match)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to kill tmux session %s: %v", match, err)
		}
	}
	return nil
}
