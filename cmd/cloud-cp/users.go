package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

var (
	userStore     = make(map[string]*User)
	userStoreMu   sync.RWMutex
	userStoreFile = "/tmp/orzbob-users.json"
)

func init() {
	// Load users from file on startup
	loadUserStore()
}

func loadUserStore() {
	file, err := os.Open(userStoreFile)
	if err != nil {
		return // File doesn't exist yet
	}
	defer file.Close()
	
	userStoreMu.Lock()
	defer userStoreMu.Unlock()
	
	_ = json.NewDecoder(file).Decode(&userStore)
}


// saveUserStoreUnlocked saves the user store without acquiring locks
// Must be called while holding at least a read lock
func saveUserStoreUnlocked() {
	file, err := os.Create(userStoreFile)
	if err != nil {
		return
	}
	defer file.Close()
	
	_ = json.NewEncoder(file).Encode(userStore)
}

func getUserByID(id string) (*User, error) {
	userStoreMu.RLock()
	defer userStoreMu.RUnlock()
	
	user, ok := userStore[id]
	if !ok {
		return nil, fmt.Errorf("user not found")
	}
	
	return user, nil
}

func getOrCreateUser(githubID int64, login, email string) (*User, error) {
	userID := fmt.Sprintf("user-%d", githubID)
	
	userStoreMu.Lock()
	defer userStoreMu.Unlock()
	
	// Check if user exists
	if user, ok := userStore[userID]; ok {
		// Update login/email if changed
		user.Login = login
		user.Email = email
		saveUserStoreUnlocked() // Use unlocked version since we hold the write lock
		return user, nil
	}
	
	// Create new user
	user := &User{
		ID:       userID,
		GitHubID: githubID,
		Login:    login,
		Email:    email,
		OrgID:    fmt.Sprintf("gh-%d", githubID),
		Plan:     "free",
		Created:  time.Now(),
	}
	
	userStore[userID] = user
	saveUserStoreUnlocked() // Use unlocked version since we hold the write lock
	
	return user, nil
}