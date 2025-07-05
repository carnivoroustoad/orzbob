package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestTokenManager(t *testing.T) {
	tm, err := NewTokenManager("orzbob-test")
	if err != nil {
		t.Fatalf("Failed to create token manager: %v", err)
	}

	t.Run("GenerateAndValidateToken", func(t *testing.T) {
		instanceID := "test-instance-123"
		duration := 5 * time.Minute

		// Generate token
		token, err := tm.GenerateToken(instanceID, duration)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		if token == "" {
			t.Error("Expected non-empty token")
		}

		// Validate token
		claims, err := tm.ValidateToken(token)
		if err != nil {
			t.Fatalf("Failed to validate token: %v", err)
		}

		// Check claims
		if claims.InstanceID != instanceID {
			t.Errorf("Expected instance ID %s, got %s", instanceID, claims.InstanceID)
		}

		if claims.Issuer != "orzbob-test" {
			t.Errorf("Expected issuer orzbob-test, got %s", claims.Issuer)
		}

		// Check expiration is in the future
		if time.Now().After(claims.ExpiresAt.Time) {
			t.Error("Token already expired")
		}

		// Check expiration is approximately correct
		expectedExpiry := time.Now().Add(duration)
		diff := claims.ExpiresAt.Time.Sub(expectedExpiry).Abs()
		if diff > time.Second {
			t.Errorf("Expiration time off by %v", diff)
		}
	})

	t.Run("ExpiredToken", func(t *testing.T) {
		instanceID := "test-instance-456"

		// Generate token with negative duration (already expired)
		token, err := tm.GenerateToken(instanceID, -1*time.Minute)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		// Try to validate expired token
		_, err = tm.ValidateToken(token)
		if err == nil {
			t.Error("Expected error validating expired token")
		}
	})

	t.Run("InvalidToken", func(t *testing.T) {
		// Test with invalid token
		_, err := tm.ValidateToken("invalid.token.here")
		if err == nil {
			t.Error("Expected error validating invalid token")
		}

		// Test with empty token
		_, err = tm.ValidateToken("")
		if err == nil {
			t.Error("Expected error validating empty token")
		}
	})

	t.Run("TokenFromDifferentManager", func(t *testing.T) {
		// Create another token manager with different issuer
		tm2, err := NewTokenManager("different-issuer")
		if err != nil {
			t.Fatalf("Failed to create second token manager: %v", err)
		}

		// Generate token with first manager
		token, err := tm.GenerateToken("instance-789", 5*time.Minute)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		// Try to validate with second manager (different key)
		_, err = tm2.ValidateToken(token)
		if err == nil {
			t.Error("Expected error validating token with different key")
		}
	})
}

func TestTokenManagerFromKeys(t *testing.T) {
	// Create a token manager and export keys
	tm1, err := NewTokenManager("test-issuer")
	if err != nil {
		t.Fatalf("Failed to create token manager: %v", err)
	}

	privateKeyPEM, err := tm1.GetPrivateKeyPEM()
	if err != nil {
		t.Fatalf("Failed to get private key PEM: %v", err)
	}

	publicKeyPEM, err := tm1.GetPublicKeyPEM()
	if err != nil {
		t.Fatalf("Failed to get public key PEM: %v", err)
	}

	// Create new token manager from exported keys
	tm2, err := NewTokenManagerFromKeys(privateKeyPEM, publicKeyPEM, "test-issuer")
	if err != nil {
		t.Fatalf("Failed to create token manager from keys: %v", err)
	}

	// Generate token with first manager
	token, err := tm1.GenerateToken("test-instance", 5*time.Minute)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Validate with second manager (same keys)
	claims, err := tm2.ValidateToken(token)
	if err != nil {
		t.Fatalf("Failed to validate token with reconstructed manager: %v", err)
	}

	if claims.InstanceID != "test-instance" {
		t.Errorf("Expected instance ID test-instance, got %s", claims.InstanceID)
	}
}

func TestShortLivedToken(t *testing.T) {
	tm, err := NewTokenManager("orzbob-test")
	if err != nil {
		t.Fatalf("Failed to create token manager: %v", err)
	}

	// Generate token with 2 second duration
	token, err := tm.GenerateToken("short-lived", 2*time.Second)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Validate immediately (should work)
	_, err = tm.ValidateToken(token)
	if err != nil {
		t.Errorf("Token should be valid immediately after generation: %v", err)
	}

	// Wait for expiration
	time.Sleep(2100 * time.Millisecond)

	// Validate again (should fail)
	_, err = tm.ValidateToken(token)
	if err == nil {
		t.Error("Expected error validating expired token")
	}
}

func TestExpiredTokenReturns401(t *testing.T) {
	// Create token manager
	tm, err := NewTokenManager("test-issuer")
	if err != nil {
		t.Fatalf("Failed to create token manager: %v", err)
	}

	// Generate an expired token
	token, err := tm.GenerateToken("test-instance", -1*time.Hour) // Already expired
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Create a test server that validates tokens
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract token from query params
		queryToken := r.URL.Query().Get("token")
		if queryToken == "" {
			http.Error(w, "No token provided", http.StatusUnauthorized)
			return
		}

		// Validate the token
		claims, err := tm.ValidateToken(queryToken)
		if err != nil {
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		// Check instance ID matches
		instanceID := strings.TrimPrefix(r.URL.Path, "/v1/instances/")
		instanceID = strings.TrimSuffix(instanceID, "/attach")
		if claims.InstanceID != instanceID {
			http.Error(w, "Token not valid for this instance", http.StatusForbidden)
			return
		}

		// If we get here, upgrade to WebSocket (which won't happen with expired token)
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
	}))
	defer server.Close()

	// Try to connect with expired token
	wsURL := strings.Replace(server.URL, "http://", "ws://", 1) + "/v1/instances/test-instance/attach?token=" + token
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)

	// We expect an error due to 401
	if err == nil {
		t.Fatal("Expected connection to fail with expired token, but it succeeded")
	}

	// Check that we got a 401 response
	if resp != nil && resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("Expected 401 Unauthorized, got %d", resp.StatusCode)
	}
}

func TestValidTokenAllowsConnection(t *testing.T) {
	// Create token manager
	tm, err := NewTokenManager("test-issuer")
	if err != nil {
		t.Fatalf("Failed to create token manager: %v", err)
	}

	// Generate a valid token
	token, err := tm.GenerateToken("test-instance", 5*time.Minute)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Create a test server that validates tokens
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract token from query params
		queryToken := r.URL.Query().Get("token")
		if queryToken == "" {
			http.Error(w, "No token provided", http.StatusUnauthorized)
			return
		}

		// Validate the token
		claims, err := tm.ValidateToken(queryToken)
		if err != nil {
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		// Check instance ID matches
		instanceID := strings.TrimPrefix(r.URL.Path, "/v1/instances/")
		instanceID = strings.TrimSuffix(instanceID, "/attach")
		if claims.InstanceID != instanceID {
			http.Error(w, "Token not valid for this instance", http.StatusForbidden)
			return
		}

		// Upgrade to WebSocket
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Send a test message
		err = conn.WriteMessage(websocket.TextMessage, []byte("Connected!"))
		if err != nil {
			return
		}
	}))
	defer server.Close()

	// Connect with valid token
	wsURL := strings.Replace(server.URL, "http://", "ws://", 1) + "/v1/instances/test-instance/attach?token=" + token
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)

	// Connection should succeed
	if err != nil {
		t.Fatalf("Failed to connect with valid token: %v (status: %v)", err, resp.StatusCode)
	}
	defer conn.Close()

	// Read the test message
	messageType, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read message: %v", err)
	}

	if messageType != websocket.TextMessage || string(data) != "Connected!" {
		t.Fatalf("Unexpected message: %s", string(data))
	}
}
