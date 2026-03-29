package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
)

type CookieAuth struct {
	secret   []byte
	maxAge   time.Duration
	clockNow func() time.Time
}

func NewCookieAuth(password string, maxAge time.Duration) *CookieAuth {
	derived := sha256.Sum256([]byte(password))
	return &CookieAuth{
		secret:   derived[:],
		maxAge:   maxAge,
		clockNow: time.Now,
	}
}

func (a *CookieAuth) Issue() (string, error) {
	expiryUnix := a.clockNow().Add(a.maxAge).Unix()
	expiryText := strconv.FormatInt(expiryUnix, 10)

	mac := hmac.New(sha256.New, a.secret)
	if _, err := mac.Write([]byte(expiryText)); err != nil {
		return "", err
	}
	signature := hex.EncodeToString(mac.Sum(nil))
	payload := expiryText + "." + signature
	return base64.RawURLEncoding.EncodeToString([]byte(payload)), nil
}

func (a *CookieAuth) Valid(token string) (bool, error) {
	if token == "" {
		return false, nil
	}

	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return false, nil
	}

	parts := strings.SplitN(string(decoded), ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false, nil
	}

	expiryUnix, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return false, nil
	}
	if a.clockNow().Unix() >= expiryUnix {
		return false, nil
	}

	providedSig, err := hex.DecodeString(parts[1])
	if err != nil {
		return false, nil
	}

	mac := hmac.New(sha256.New, a.secret)
	if _, err := mac.Write([]byte(parts[0])); err != nil {
		return false, err
	}
	expectedSig := mac.Sum(nil)
	if !hmac.Equal(providedSig, expectedSig) {
		return false, nil
	}

	return true, nil
}
