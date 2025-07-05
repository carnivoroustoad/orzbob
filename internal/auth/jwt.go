package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenManager handles JWT token generation and validation
type TokenManager struct {
	privateKey *ecdsa.PrivateKey
	publicKey  *ecdsa.PublicKey
	issuer     string
}

// Claims represents the JWT claims for instance attachment
type Claims struct {
	jwt.RegisteredClaims
	InstanceID string `json:"instance_id,omitempty"`
	UserID     string `json:"user_id,omitempty"`
	Tier       string `json:"tier,omitempty"`
	Type       string `json:"type"` // "instance" or "user"
}

// NewTokenManager creates a new token manager with a generated ES256 key pair
func NewTokenManager(issuer string) (*TokenManager, error) {
	// Generate ECDSA P-256 key pair
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	return &TokenManager{
		privateKey: privateKey,
		publicKey:  &privateKey.PublicKey,
		issuer:     issuer,
	}, nil
}

// NewTokenManagerFromKeys creates a token manager from existing PEM-encoded keys
func NewTokenManagerFromKeys(privateKeyPEM, publicKeyPEM []byte, issuer string) (*TokenManager, error) {
	// Parse private key
	block, _ := pem.Decode(privateKeyPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block containing private key")
	}

	privateKey, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse EC private key: %w", err)
	}

	// Parse public key if provided, otherwise use the one from private key
	var publicKey *ecdsa.PublicKey
	if len(publicKeyPEM) > 0 {
		block, _ = pem.Decode(publicKeyPEM)
		if block == nil {
			return nil, fmt.Errorf("failed to parse PEM block containing public key")
		}

		pubInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse public key: %w", err)
		}

		var ok bool
		publicKey, ok = pubInterface.(*ecdsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("not an ECDSA public key")
		}
	} else {
		publicKey = &privateKey.PublicKey
	}

	return &TokenManager{
		privateKey: privateKey,
		publicKey:  publicKey,
		issuer:     issuer,
	}, nil
}

// GenerateToken creates a new JWT token for instance attachment
func (tm *TokenManager) GenerateToken(instanceID string, duration time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    tm.issuer,
			Subject:   instanceID,
			ExpiresAt: jwt.NewNumericDate(now.Add(duration)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        fmt.Sprintf("%d", now.UnixNano()),
		},
		InstanceID: instanceID,
		Type:       "instance",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	signedToken, err := token.SignedString(tm.privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return signedToken, nil
}

// GenerateUserToken creates a JWT token for API authentication
func (tm *TokenManager) GenerateUserToken(userID string, duration time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    tm.issuer,
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(now.Add(duration)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        fmt.Sprintf("user-%d", now.UnixNano()),
		},
		UserID: userID,
		Type:   "user",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	return token.SignedString(tm.privateKey)
}

// ValidateToken validates a JWT token and returns the claims
func (tm *TokenManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing algorithm
		if _, ok := token.Method.(*jwt.SigningMethodECDSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return tm.publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, fmt.Errorf("invalid claims type")
	}

	// Additional validation
	if claims.Issuer != tm.issuer {
		return nil, fmt.Errorf("invalid issuer")
	}

	return claims, nil
}

// GetPrivateKeyPEM returns the private key in PEM format
func (tm *TokenManager) GetPrivateKeyPEM() ([]byte, error) {
	x509Encoded, err := x509.MarshalECPrivateKey(tm.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal private key: %w", err)
	}

	pemEncoded := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: x509Encoded,
	})

	return pemEncoded, nil
}

// GetPublicKeyPEM returns the public key in PEM format
func (tm *TokenManager) GetPublicKeyPEM() ([]byte, error) {
	x509EncodedPub, err := x509.MarshalPKIXPublicKey(tm.publicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal public key: %w", err)
	}

	pemEncodedPub := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: x509EncodedPub,
	})

	return pemEncodedPub, nil
}
