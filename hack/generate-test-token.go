package main

import (
	"fmt"
	"orzbob/internal/auth"
	"time"
)

func main() {
	// Create token manager
	tm, err := auth.NewTokenManager("orzbob-cloud")
	if err != nil {
		panic(err)
	}

	// Generate a user token
	userID := "user-69773422" // Your GitHub user ID
	token, err := tm.GenerateUserToken(userID, 90*24*time.Hour)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Generated JWT token:\n%s\n", token)
	fmt.Printf("\nSave this to ~/.config/orzbob/token.json as the api_token\n")
}