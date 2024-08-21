package githubappjwt

import (
	"crypto/rsa"
	"github.com/cockroachdb/errors"
	"github.com/golang-jwt/jwt/v5"
	"time"
)

// Generate generates a JWT for a GitHub App
func Generate(appID string, privateKey *rsa.PrivateKey, expiration ...time.Duration) (string, error) {
	exp := 10 * time.Minute // デフォルト値を10分に設定

	if len(expiration) > 0 {
		exp = expiration[0]
	}

	claims := jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(exp)),
		Issuer:    appID,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signedToken, err := token.SignedString(privateKey)
	if err != nil {
		return "", errors.Errorf("error signing token: %v", err)
	}
	return signedToken, nil
}
