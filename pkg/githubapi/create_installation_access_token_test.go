package githubapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestCreateInstallationAccessToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-jwt" {
			t.Errorf("Expected Authorization header 'Bearer test-jwt', got %s", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Accept") != "application/vnd.github.v3+json" {
			t.Errorf("Expected Accept header 'application/vnd.github.v3+json', got %s", r.Header.Get("Accept"))
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(AccessTokenResponse{
			Token:     "test-access-token",
			ExpiresAt: time.Now().Add(time.Hour),
		})
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		BaseURL: server.URL,
	})

	resp, err := client.CreateInstallationAccessToken("test-installation-id", "test-jwt")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.Token != "test-access-token" {
		t.Errorf("Expected token 'test-access-token', got %s", resp.Token)
	}
}

func TestCreateInstallationAccessTokenError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Unauthorized"))
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		BaseURL: server.URL,
	})

	_, err := client.CreateInstallationAccessToken("test-installation-id", "invalid-jwt")

	if err == nil {
		t.Fatal("Expected an error, got nil")
	}
	expectedError := "unexpected status code: 401, body: Unauthorized"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestCreateInstallationAccessTokenLive(t *testing.T) {
	jwt := os.Getenv("GITHUB_APP_JWT")
	installationID := os.Getenv("GITHUB_INSTALLATION_ID")
	baseURL := os.Getenv("GITHUB_API_BASE_URL")

	if jwt == "" || installationID == "" {
		t.Skip("Skipping live API test. Set GITHUB_APP_JWT and GITHUB_INSTALLATION_ID environment variables to run this test.")
	}

	config := ClientConfig{}
	if baseURL != "" {
		config.BaseURL = baseURL
	}
	client := NewClient(config)

	resp, err := client.CreateInstallationAccessToken(installationID, jwt)
	if err != nil {
		t.Fatalf("Error creating installation access token: %v", err)
	}

	if resp.Token == "" {
		t.Error("Received empty token from API")
	}
	fmt.Printf("Response: %+v\n", resp)
}
