package trakt

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/scottia/IMDb-Trakt-TMDb-Sync/internal/config"
)

type authRoundTripFunc func(*http.Request) (*http.Response, error)

func (f authRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func testTraktConfig() config.Trakt {
	clientID := "test-client-id"
	clientSecret := "test-client-secret"
	tokenFile := "test-token.json"
	return config.Trakt{
		ClientID:     &clientID,
		ClientSecret: &clientSecret,
		TokenFile:    &tokenFile,
	}
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestTraktAuthAndAPIHostsAreSeparated(t *testing.T) {
	conf := testTraktConfig()
	ac := newAuthClient(conf, authRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request: %s", req.URL.Path)
		return nil, nil
	}))

	if ac.baseURL != traktAuthBaseURL {
		t.Fatalf("auth client base URL = %q, want auth base URL", ac.baseURL)
	}
	if traktAPIBaseURL == traktAuthBaseURL {
		t.Fatal("Trakt API and OAuth base URLs must remain separate")
	}

	dataClient := &client{baseURL: traktAPIBaseURL}
	if dataClient.baseURL != traktAPIBaseURL {
		t.Fatalf("data client base URL = %q, want API base URL", dataClient.baseURL)
	}
}

func TestDeviceOAuthRequestsUseAuthHost(t *testing.T) {
	conf := testTraktConfig()
	var seen []string

	transport := authRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Scheme+"://"+req.URL.Host != traktAuthBaseURL {
			t.Fatalf("OAuth request used wrong host: %s", req.URL.Host)
		}
		seen = append(seen, req.URL.Path)

		switch req.URL.Path {
		case pathAuthCodes:
			var body authCodesBody
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Fatalf("decode device code body: %v", err)
			}
			if body.ClientID != *conf.ClientID {
				t.Fatal("device code body changed client_id")
			}
			return jsonResponse(http.StatusOK, `{"device_code":"device","user_code":"user","verification_url":"https://example.invalid","expires_in":60,"interval":0}`), nil
		case pathAuthTokens:
			var body map[string]any
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Fatalf("decode device token body: %v", err)
			}
			if _, ok := body["redirect_uri"]; ok {
				t.Fatal("device token body must not contain redirect_uri")
			}
			return jsonResponse(http.StatusOK, `{"access_token":"access","refresh_token":"refresh","created_at":1,"expires_in":3600}`), nil
		default:
			t.Fatalf("unexpected OAuth path: %s", req.URL.Path)
			return nil, nil
		}
	})

	ac := newAuthClient(conf, transport)
	codes, err := ac.getAuthCodes(context.Background())
	if err != nil {
		t.Fatalf("get auth codes: %v", err)
	}
	if _, err := ac.pollAccessToken(context.Background(), codes.DeviceCode, 0, codes.ExpiresIn); err != nil {
		t.Fatalf("poll access token: %v", err)
	}

	if len(seen) != 2 || seen[0] != pathAuthCodes || seen[1] != pathAuthTokens {
		t.Fatalf("unexpected OAuth request sequence: %v", seen)
	}
}

func TestRefreshTokenUsesAuthHostAndOmitsRedirectURI(t *testing.T) {
	conf := testTraktConfig()

	transport := authRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Scheme+"://"+req.URL.Host != traktAuthBaseURL {
			t.Fatalf("refresh request used wrong host: %s", req.URL.Host)
		}
		if req.URL.Path != pathAuthToken {
			t.Fatalf("refresh request path = %q", req.URL.Path)
		}

		var body map[string]any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatalf("decode refresh body: %v", err)
		}
		if _, ok := body["redirect_uri"]; ok {
			t.Fatal("refresh body must not contain legacy redirect_uri")
		}
		if body["grant_type"] != grantTypeRefreshToken {
			t.Fatalf("refresh grant_type = %#v", body["grant_type"])
		}
		if body["refresh_token"] != "stored-refresh-token" {
			t.Fatal("refresh token field changed")
		}

		return jsonResponse(http.StatusOK, `{"access_token":"new-access","refresh_token":"new-refresh","created_at":1,"expires_in":3600}`), nil
	})

	ac := newAuthClient(conf, transport)
	token, err := ac.refreshAccessToken(context.Background(), "stored-refresh-token")
	if err != nil {
		t.Fatalf("refresh access token: %v", err)
	}
	if token.token != "new-access" || token.refreshToken != "new-refresh" {
		t.Fatal("refresh token rotation response was not preserved")
	}
	if token.expiresAt != time.Unix(3601, 0) {
		t.Fatalf("refresh expiry changed: %v", token.expiresAt)
	}
}
