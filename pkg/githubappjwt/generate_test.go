package githubappjwt

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testPrivateKeyPEM = `
-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAux2IyykhqksBG9rFh4OjtIkRK93YAImMSDDnfsE4IASuq3uM
JoOxvY1PCrlnE3vJdMlLYWSuY7irMnJiBbE7wt+AeDT4xEEhy0rxhu+NXUqK8K/K
cy7y3Ki0TzBTmQ+SS3ob5jAoQSOkZ61AjFEE3Up8/Rss9pIYwqSKBrTG3QkxUXc3
2BWz9jfBoXGOc1YoT+/wuDmquFJj1ufOienwONzBQ5beXeepn5wNuegWIUsss0Zp
L1zp9uL4hVZHTrcfOpp3gq4Z/diD44uRUQ5O5WOSy8yUPHsjFJXvQQTmXaiIF8wj
HObifkayJ2LijJ2m/szIot+nV5ZXY0HuKHixwwIDAQABAoIBAFk3a9HyeqrHuG+f
kC9dBOE/uYBA9ozLCKgjKT22wxwBH4eEEP8MK+NFTTq/y/XuP8//aoG1j7DcjEQx
ZatxJh10k7y9BSAOLh7QTPkZnz2sHTNFnjHtYL71cYOQd0uzsP1r64GF1Ku6YtlM
Mkq1Fqysp4vHOVkXr9aevXEVIPyiZLNpC22bbMQAkE9BP7qL4YvOJCgert4h6dUE
3D1w3u6sFdHLFOYcgH4eD0aKArMNNW3z7BTGg0+GyXIfOgV8uWPjsD6noBNEaTvl
zH8pPrjbkLMzwseNrJelp7+MN9ObFulbZSViHeiGb/qby9U6ubPeStRHzw6HC7Uo
btzekYECgYEA3qflRIMvzhgLYmfrrirjoXnt+smyz6COKjoKW1LTI21D98WBfqne
v11qkYBdNzUXAWBpSdoobMLdblVjQtIGtMUUuIriwK11QMsWfBKEpO4DW4lqTLTW
+XEcSIMczSeWw/zdn3CG9sxUXJcXPVTgCSIszCEQZTsdZ7+5y1sxvcUCgYEA1yMZ
oNPb2dAra0kb1yROHk1LGpU83obqkPyQftzMYEO1EEsV+XNmlVzQgdLUl+ay/E4z
B2eqhrj8SmjEYm7e09e5wEJyF/JdyQ6iXjcmdBM2MRvPSg8LKGpORMg6v/JodIDD
I5XE9Ook/h/c9fRxViwgEARYFC5hY4uCmP4w8ecCgYBa/+G7I6bJI5ibin+PemX4
XB4AbqkPJL6V0YzkEDDM/N5XiLhJLWIlcieY+g6e/qq9XEsL7QaylN3tNybPa4lk
Hlw+pDzSpNIUPiydXvApfEGRCtOQMCTgY/M8S6Hc0z5SManefR4cBhzAjtvnrCW4
deg7MZRC22tEON7Vlxr4RQKBgGaY0KIIJvKK+gniBarmH3MH/WciALNGuBqIuAgo
GDdYUsMAa+xYgnV8m9stxkDivjzgtikz4Pj6wyZhLDadFRsF6AmuJmcRKHS3y+sO
dgIpH1DwKDzzS6jseYMH0iyz1+ind2hDBnieKSIf4+pPtrUXufqpd6+4Jq2oXJHF
t2XFAoGBAK+snfSXVmsx6kbAtPJvEw6K6smccIYCzcLEduGHTWjqsrPl6gvHQ2YE
gtrBrE1fNh6QTD/+lAhp4ygfHbykBGqTmuWYWr6MRlnTnWadHV6rwReCERdJtygg
AZJ8k0egSszC2U+CGcR1chiWWCxvF14OgmsdnxNJZHJ1owCsABQi
-----END RSA PRIVATE KEY-----
`

func TestGenerateJwtWithPEM(t *testing.T) {
	// Parse the PEM encoded private key
	block, _ := pem.Decode([]byte(testPrivateKeyPEM))
	if block == nil {
		t.Fatal("Failed to parse PEM block containing the private key")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse private key: %v", err)
	}

	// Test parameters
	appID := "975222"

	// Test cases
	testCases := []struct {
		name       string
		expiration time.Duration
		maxExpTime time.Duration
	}{
		{"Default 10 minutes", 0, 11 * time.Minute},
		{"Custom 5 minutes", 5 * time.Minute, 6 * time.Minute},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Generate JWT
			var token string
			var err error
			if tc.expiration == 0 {
				token, err = Generate(appID, privateKey)
			} else {
				token, err = Generate(appID, privateKey, tc.expiration)
			}
			if err != nil {
				t.Fatalf("Failed to generate JWT: %v", err)
			}

			// Parse and validate the token
			parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				return &privateKey.PublicKey, nil
			})

			if err != nil {
				t.Fatalf("Failed to parse JWT: %v", err)
			}

			if !parsedToken.Valid {
				t.Errorf("Token is not valid")
			}

			// Check claims
			claims, ok := parsedToken.Claims.(jwt.MapClaims)
			if !ok {
				t.Fatalf("Failed to parse claims")
			}

			// Check issuer
			if issuer, ok := claims["iss"].(string); !ok || issuer != appID {
				t.Errorf("Incorrect issuer. Expected %s, got %v", appID, issuer)
			}

			// Check issued at time
			if iat, ok := claims["iat"].(float64); !ok || int64(iat) > time.Now().Unix() {
				t.Errorf("Incorrect issued at time")
			}

			// Check expiration time
			if exp, ok := claims["exp"].(float64); !ok || int64(exp) <= time.Now().Unix() || int64(exp) > time.Now().Add(tc.maxExpTime).Unix() {
				t.Errorf("Incorrect expiration time")
			}

			// Print claims
			fmt.Printf("Claims for %s: %v\n", tc.name, claims)

			// Print token
			fmt.Printf("Token for %s: %v\n", tc.name, token)
		})
	}
}
