package token

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"time"

	db "backend.chesswahili.com/internal/db/sqlc"
	"github.com/google/uuid"
)

const (
	ScopeAuthentication = "authentication"
	ttl                 = 365 * 24 * time.Hour
)

func New(user_id uuid.UUID, store db.Store, scope string) (string, time.Time, error) {

	token, tokenText, err := generateToken(user_id, ttl, scope)
	if err != nil {
		return "", time.Time{}, err
	}

	err = store.CreateToken(context.Background(), *token)
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenText, token.Expiry, err
}

func generateToken(user_id uuid.UUID, ttl time.Duration, scope string) (*db.CreateTokenParams, string, error) {

	token := &db.CreateTokenParams{
		UserID: user_id,
		Expiry: time.Now().Add(ttl),
		Scope:  scope,
	}

	randomBytes := make([]byte, 16)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return nil, "", err
	}

	tokenPlaintext := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes)
	hash := sha256.Sum256([]byte(tokenPlaintext))
	token.Hash = hash[:]
	return token, tokenPlaintext, nil
}
