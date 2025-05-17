//go:build windows

package tmux

import (
	"orzbob/log"
	"os"
	"time"

	"golang.org/x/term"
)

// monitorWindowSize monitors and handles window resize events while attached.
func (t *TmuxSession) monitorWindowSize() {
	everyN := log.NewEvery(60 * time.Second)
	
	// Use the current terminal height and width.
	doUpdate := func() {
		cols, rows, err := term.GetSize(int(os.Stdin.Fd()))
		if err != nil {
			if everyN.ShouldLog() {
				log.ErrorLog.Printf("failed to update window size: %v", err)
			}
			return
		}
		
		if err := t.updateWindowSize(cols, rows); err != nil {
			if everyN.ShouldLog() {
				log.ErrorLog.Printf("failed to update window size: %v", err)
			}
		}
	}

	// Do one at the start to set the initial size
	doUpdate()

	// On Windows, we'll just periodically check for window size changes
	// since SIGWINCH is not available, but use a longer interval to reduce overhead
	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()

	var lastCols, lastRows int
	lastCols, lastRows, _ = term.GetSize(int(os.Stdin.Fd()))

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		for {
			select {
			case <-t.ctx.Done():
				return
			case <-ticker.C:
				cols, rows, err := term.GetSize(int(os.Stdin.Fd()))
				if err != nil {
					continue
				}
				// Only update if size actually changed
				if cols != lastCols || rows != lastRows {
					lastCols, lastRows = cols, rows
					doUpdate()
				}
			}
		}
	}()
}
