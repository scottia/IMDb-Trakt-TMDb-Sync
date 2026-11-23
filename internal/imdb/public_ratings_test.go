package imdb

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseRatingsUserList(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "legacy id", value: "ur1234567", want: "ur1234567"},
		{name: "profile id", value: "p.abcdefghijklmnop", want: "p.abcdefghijklmnop"},
		{
			name:  "full ratings url",
			value: "https://www.imdb.com/user/ur1234567/ratings?ref_=ext_shr_lnk",
			want:  "ur1234567",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseRatingsUserList(test.value)
			if err != nil {
				t.Fatalf("parseRatingsUserList() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("parseRatingsUserList() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestParseRatingsUserListRejectsOtherURLs(t *testing.T) {
	for _, value := range []string{
		"",
		"http://www.imdb.com/user/ur1234567/ratings",
		"https://example.com/user/ur1234567/ratings",
		"https://www.imdb.com/user/ur1234567/watchlist",
	} {
		if _, err := parseRatingsUserList(value); err == nil {
			t.Fatalf("parseRatingsUserList(%q) unexpectedly succeeded", value)
		}
	}
}

func TestPublicRatingsClientExportsCanonicalSnapshot(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("x-imdb-client-name"); got != "imdb-web-next" {
			t.Fatalf("x-imdb-client-name = %q", got)
		}
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Fatal("public ratings request must not send authentication")
		}
		var payload struct {
			Variables struct {
				UserID string `json:"userId"`
				First  int    `json:"first"`
			} `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload.Variables.UserID != "ur1234567" {
			t.Fatalf("userId = %q", payload.Variables.UserID)
		}

		edges := []map[string]any{
			{
				"node": map[string]any{
					"userRating": map[string]any{"value": 8, "date": "2026-11-20"},
					"title": map[string]any{
						"id":        "tt0000001",
						"titleText": map[string]any{"text": "First"},
						"titleType": map[string]any{"id": "movie", "isEpisode": false, "isSeries": false},
					},
				},
			},
			{
				"node": map[string]any{
					"userRating": map[string]any{"value": 9, "date": "2026-11-21T12:00:00Z"},
					"title": map[string]any{
						"id":        "tt0000002",
						"titleText": map[string]any{"text": "Second"},
						"titleType": map[string]any{"id": "tvSeries", "isEpisode": false, "isSeries": true},
					},
				},
			},
		}
		if payload.Variables.First == 1 {
			edges = edges[:1]
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"userRatings": map[string]any{
					"total": 2,
					"edges": edges,
				},
			},
		})
	}))
	defer server.Close()

	clientAPI, err := newPublicRatingsAPI(
		context.Background(),
		"https://www.imdb.com/user/ur1234567/ratings",
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		server.Client(),
		server.URL,
	)
	if err != nil {
		t.Fatalf("newPublicRatingsAPI() error = %v", err)
	}
	if err := clientAPI.RatingsExport(); err != nil {
		t.Fatalf("RatingsExport() error = %v", err)
	}
	if requests != 2 {
		t.Fatalf("request count = %d, want 2", requests)
	}
	items, err := clientAPI.RatingsGet()
	if err != nil {
		t.Fatalf("RatingsGet() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("ratings count = %d, want 2", len(items))
	}
	if items[0].ID != "tt0000002" || items[0].Kind != itemTypeTvSeries {
		t.Fatalf("first item = %#v", items[0])
	}
	if items[1].ID != "tt0000001" || items[1].Kind != itemTypeMovie {
		t.Fatalf("second item = %#v", items[1])
	}

	reader := csv.NewReader(strings.NewReader(string(clientAPI.RatingsCSV())))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("csv records = %d, want 3", len(records))
	}
	if records[0][0] != "Const" || records[0][1] != "Your Rating" || records[0][2] != "Date Rated" {
		t.Fatalf("unexpected csv header: %v", records[0])
	}
	if records[1][0] != "tt0000002" || records[1][1] != "9" || records[1][6] != itemTypeTvSeries {
		t.Fatalf("unexpected first csv row: %v", records[1])
	}
}

func TestPublicRatingsClientRejectsIncompleteSnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"userRatings": map[string]any{
					"total": 2,
					"edges": []map[string]any{
						{
							"node": map[string]any{
								"userRating": map[string]any{"value": 8, "date": "2026-11-20"},
								"title": map[string]any{
									"id":        "tt0000001",
									"titleText": map[string]any{"text": "First"},
									"titleType": map[string]any{"id": "movie", "isEpisode": false, "isSeries": false},
								},
							},
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	clientAPI, err := newPublicRatingsAPI(
		context.Background(),
		"ur1234567",
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		server.Client(),
		server.URL,
	)
	if err != nil {
		t.Fatalf("newPublicRatingsAPI() error = %v", err)
	}
	if err := clientAPI.RatingsExport(); err == nil || !strings.Contains(err.Error(), "reported 2") {
		t.Fatalf("RatingsExport() error = %v, want incomplete snapshot error", err)
	}
}

func TestIsWAFBlocked(t *testing.T) {
	if IsWAFBlocked(nil) {
		t.Fatal("nil error classified as WAF")
	}
	err := errors.New("failure handling waf challenge: aws waf challenge remained after solver token and reload")
	if !IsWAFBlocked(err) {
		t.Fatal("known WAF error was not classified")
	}
	if IsWAFBlocked(errors.New("failure authenticating with the provided cookies")) {
		t.Fatal("unrelated authentication error classified as WAF")
	}
}
