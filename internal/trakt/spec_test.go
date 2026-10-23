package trakt

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRatingsBodyOmitsWatchedAt(t *testing.T) {
	ratedAt := "2026-10-23T00:00:00Z"
	watchedAt := "2026-10-23T00:00:00Z"
	rating := 7.0

	items := Items{
		{
			Type: ItemTypeMovie,
			Movie: ItemSpec{
				IDMeta:    IDMeta{IMDb: "tt1234567"},
				RatedAt:   &ratedAt,
				Rating:    &rating,
				WatchedAt: &watchedAt,
			},
		},
	}

	body := items.toRatingsBody()
	if len(body.Movies) != 1 {
		t.Fatalf("expected 1 movie, got %d", len(body.Movies))
	}
	if body.Movies[0].WatchedAt != nil {
		t.Fatal("ratings body must not include watched_at")
	}
	if body.Movies[0].RatedAt == nil || *body.Movies[0].RatedAt != ratedAt {
		t.Fatalf("rated_at changed: %#v", body.Movies[0].RatedAt)
	}
	if body.Movies[0].Rating == nil || *body.Movies[0].Rating != rating {
		t.Fatalf("rating changed: %#v", body.Movies[0].Rating)
	}

	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal ratings body: %v", err)
	}
	if strings.Contains(string(raw), "watched_at") {
		t.Fatalf("ratings JSON contains watched_at: %s", raw)
	}
	if !strings.Contains(string(raw), `"rated_at":"2026-10-23T00:00:00Z"`) {
		t.Fatalf("ratings JSON missing RFC3339 rated_at: %s", raw)
	}
}
