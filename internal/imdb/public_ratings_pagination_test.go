package imdb

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPublicRatingsClientUsesCursorPagination(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var payload struct {
			Variables struct {
				UserID string  `json:"userId"`
				First  int     `json:"first"`
				After  *string `json:"after"`
			} `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		userRatings := map[string]any{"total": 3}
		switch requests {
		case 1:
			if payload.Variables.First != 1 || payload.Variables.After != nil {
				t.Fatalf("summary variables = first:%d after:%v", payload.Variables.First, payload.Variables.After)
			}
			userRatings["pageInfo"] = map[string]any{"endCursor": "summary", "hasNextPage": true}
			userRatings["edges"] = []map[string]any{paginationEdge("tt0000001", "2026-11-20")}
		case 2:
			if payload.Variables.First != publicRatingsPageSize || payload.Variables.After != nil {
				t.Fatalf("page 1 variables = first:%d after:%v", payload.Variables.First, payload.Variables.After)
			}
			userRatings["pageInfo"] = map[string]any{"endCursor": "cursor-1", "hasNextPage": true}
			userRatings["edges"] = []map[string]any{
				paginationEdge("tt0000001", "2026-11-20"),
				paginationEdge("tt0000002", "2026-11-21"),
			}
		case 3:
			if payload.Variables.First != publicRatingsPageSize || payload.Variables.After == nil || *payload.Variables.After != "cursor-1" {
				t.Fatalf("page 2 variables = first:%d after:%v", payload.Variables.First, payload.Variables.After)
			}
			userRatings["pageInfo"] = map[string]any{"endCursor": "cursor-2", "hasNextPage": false}
			userRatings["edges"] = []map[string]any{paginationEdge("tt0000003", "2026-11-22")}
		default:
			t.Fatalf("unexpected request %d", requests)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"userRatings": userRatings},
		})
	}))
	defer server.Close()

	api, err := newPublicRatingsAPI(
		context.Background(),
		"ur1234567",
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		server.Client(),
		server.URL,
	)
	if err != nil {
		t.Fatalf("newPublicRatingsAPI() error = %v", err)
	}
	if err := api.RatingsExport(); err != nil {
		t.Fatalf("RatingsExport() error = %v", err)
	}
	items, err := api.RatingsGet()
	if err != nil {
		t.Fatalf("RatingsGet() error = %v", err)
	}
	if requests != 3 || len(items) != 3 {
		t.Fatalf("requests=%d items=%d, want 3/3", requests, len(items))
	}
}

func paginationEdge(id, date string) map[string]any {
	return map[string]any{
		"node": map[string]any{
			"userRating": map[string]any{"value": 8, "date": date},
			"title": map[string]any{
				"id":        id,
				"titleText": map[string]any{"text": id},
				"titleType": map[string]any{"id": "movie", "isEpisode": false, "isSeries": false},
			},
		},
	}
}
