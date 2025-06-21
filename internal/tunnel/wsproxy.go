package tunnel

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins for now (configure properly for production)
		return true
	},
}

// Message types for WebSocket communication
const (
	MessageTypeData  = "data"
	MessageTypeResize = "resize"
	MessageTypePing   = "ping"
	MessageTypePong   = "pong"
)

// Message represents a WebSocket message
type Message struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
	Cols int    `json:"cols,omitempty"`
	Rows int    `json:"rows,omitempty"`
}

// WSProxy handles WebSocket connections for terminal sessions
type WSProxy struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

// Session represents an active WebSocket session
type Session struct {
	ID       string
	conn     *websocket.Conn
	instance string
	mu       sync.Mutex
	done     chan struct{}
}

// NewWSProxy creates a new WebSocket proxy
func NewWSProxy() *WSProxy {
	return &WSProxy{
		sessions: make(map[string]*Session),
	}
}

// HandleAttach handles WebSocket connections for instance attachment
func (p *WSProxy) HandleAttach(instanceID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("Failed to upgrade connection: %v", err)
			return
		}
		defer conn.Close()

		sessionID := fmt.Sprintf("%s-%d", instanceID, time.Now().Unix())
		session := &Session{
			ID:       sessionID,
			conn:     conn,
			instance: instanceID,
			done:     make(chan struct{}),
		}

		p.mu.Lock()
		p.sessions[sessionID] = session
		p.mu.Unlock()

		defer func() {
			p.mu.Lock()
			delete(p.sessions, sessionID)
			p.mu.Unlock()
			close(session.done)
		}()

		log.Printf("WebSocket session %s started for instance %s", sessionID, instanceID)

		// Start echo handler
		session.handleEcho()

		log.Printf("WebSocket session %s ended", sessionID)
	}
}

// handleEcho implements tmux attachment for the session
func (s *Session) handleEcho() {
	// For control plane, we still use echo mode
	// The actual tmux attachment happens in the runner pod
	s.handleEchoMode()
}

// handleEchoMode implements a simple echo server for testing
func (s *Session) handleEchoMode() {
	// Set up ping/pong to keep connection alive
	s.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	s.conn.SetPongHandler(func(string) error {
		s.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Start ping ticker
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ticker.C:
				s.mu.Lock()
				if err := s.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					s.mu.Unlock()
					return
				}
				s.mu.Unlock()
			case <-s.done:
				return
			}
		}
	}()

	// Echo loop
	for {
		messageType, data, err := s.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Echo the message back
		s.mu.Lock()
		if err := s.conn.WriteMessage(messageType, data); err != nil {
			s.mu.Unlock()
			log.Printf("Failed to write message: %v", err)
			break
		}
		s.mu.Unlock()

		// Reset read deadline
		s.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	}
}

// Client represents a WebSocket client for connecting to the proxy
type Client struct {
	conn   *websocket.Conn
	done   chan struct{}
	mu     sync.Mutex
}

// NewClient creates a new WebSocket client
func NewClient(url string) (*Client, error) {
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	client := &Client{
		conn: conn,
		done: make(chan struct{}),
	}

	// Set up ping/pong
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	return client, nil
}

// NewClientWithToken creates a new WebSocket client with JWT token
func NewClientWithToken(url string, token string) (*Client, error) {
	// Add token to URL query parameters
	if token != "" {
		separator := "?"
		if strings.Contains(url, "?") {
			separator = "&"
		}
		url = fmt.Sprintf("%s%stoken=%s", url, separator, token)
	}
	
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	client := &Client{
		conn: conn,
		done: make(chan struct{}),
	}

	// Set up ping/pong
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	return client, nil
}

// Close closes the client connection
func (c *Client) Close() error {
	close(c.done)
	return c.conn.Close()
}

// Start starts the client I/O handling
func (c *Client) Start(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) error {
	// Start ping ticker
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ticker.C:
				c.mu.Lock()
				if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					c.mu.Unlock()
					return
				}
				c.mu.Unlock()
			case <-c.done:
				return
			}
		}
	}()

	// Start reader goroutine (WebSocket -> stdout)
	errCh := make(chan error, 2)
	go func() {
		for {
			messageType, data, err := c.conn.ReadMessage()
			if err != nil {
				errCh <- err
				return
			}

			if messageType == websocket.TextMessage || messageType == websocket.BinaryMessage {
				if _, err := stdout.Write(data); err != nil {
					errCh <- err
					return
				}
			}
		}
	}()

	// Start writer goroutine (stdin -> WebSocket)
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stdin.Read(buf)
			if err != nil {
				if err != io.EOF {
					errCh <- err
				}
				return
			}

			c.mu.Lock()
			if err := c.conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
				c.mu.Unlock()
				errCh <- err
				return
			}
			c.mu.Unlock()
		}
	}()

	// Wait for error or context cancellation
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		return nil
	}
}