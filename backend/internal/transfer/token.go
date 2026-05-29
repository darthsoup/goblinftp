// backend/internal/transfer/token.go
package transfer

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
)

var (
	ErrTokenExpired = errors.New("token expired")
	ErrTokenInvalid = errors.New("token invalid")
)

// IssueToken creates a signed download token.
// Format (before outer base64): sessionID:base64url(path):expiryUnix:hexHMAC
func IssueToken(secret []byte, sessionID, path string, expiry time.Time) (string, error) {
	encodedPath := base64.RawURLEncoding.EncodeToString([]byte(path))
	expiryStr := strconv.FormatInt(expiry.Unix(), 10)
	message := sessionID + ":" + encodedPath + ":" + expiryStr
	mac := computeHMAC(secret, message)
	raw := message + ":" + mac
	return base64.RawURLEncoding.EncodeToString([]byte(raw)), nil
}

// ValidateToken verifies the HMAC, checks expiry, and returns (sessionID, path).
func ValidateToken(secret []byte, token string) (sessionID, path string, err error) {
	rawBytes, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return "", "", ErrTokenInvalid
	}
	// Exactly 4 parts: sessionID, base64url(path), expiryUnix, hexHMAC
	parts := strings.SplitN(string(rawBytes), ":", 4)
	if len(parts) != 4 {
		return "", "", ErrTokenInvalid
	}
	sessionID, encodedPath, expiryStr, gotMAC := parts[0], parts[1], parts[2], parts[3]

	message := sessionID + ":" + encodedPath + ":" + expiryStr
	expectedMAC := computeHMAC(secret, message)
	if !hmac.Equal([]byte(gotMAC), []byte(expectedMAC)) {
		return "", "", ErrTokenInvalid
	}

	expiryUnix, err := strconv.ParseInt(expiryStr, 10, 64)
	if err != nil {
		return "", "", ErrTokenInvalid
	}
	if time.Now().Unix() > expiryUnix {
		return "", "", ErrTokenExpired
	}

	pathBytes, err := base64.RawURLEncoding.DecodeString(encodedPath)
	if err != nil {
		return "", "", ErrTokenInvalid
	}
	return sessionID, string(pathBytes), nil
}

func computeHMAC(secret []byte, message string) string {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}
