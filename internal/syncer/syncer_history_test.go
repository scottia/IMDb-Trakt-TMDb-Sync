package syncer

import (
	"context"
	"io"
	"log/slog"
	"testing"

	appconfig "github.com/scottia/IMDb-Trakt-TMDb-Sync/internal/config"
	"github.com/scottia/IMDb-Trakt-TMDb-Sync/internal/imdb"
	"github.com/scottia/IMDb-Trakt-TMDb-Sync/internal/trakt"
)

type historyRemovalSpy struct {
	historyGetCalls    int
	historyRemoveCalls int
	historyGetResult   trakt.Items
}

var _ trakt.API = (*historyRemovalSpy)(nil)

func (s *historyRemovalSpy) HistoryAdd(context.Context, trakt.Items) error { return nil }
func (s *historyRemovalSpy) HistoryGet(context.Context, string, string) (trakt.Items, error) {
	s.historyGetCalls++
	return s.historyGetResult, nil
}
func (s *historyRemovalSpy) HistoryRemove(context.Context, trakt.Items) error {
	s.historyRemoveCalls++
	return nil
}
func (s *historyRemovalSpy) ListCreate(context.Context, string) (*trakt.IDMeta, error) {
	return &trakt.IDMeta{}, nil
}
func (s *historyRemovalSpy) ListGet(context.Context, string) (*trakt.List, error) {
	return &trakt.List{}, nil
}
func (s *historyRemovalSpy) ListGetMeta(context.Context, string) (*trakt.List, error) {
	return &trakt.List{}, nil
}
func (s *historyRemovalSpy) ListItemsAdd(context.Context, string, string, trakt.Items) error {
	return nil
}
func (s *historyRemovalSpy) ListItemsRemove(context.Context, string, string, trakt.Items) error {
	return nil
}
func (s *historyRemovalSpy) ListsGet(context.Context, trakt.IDMetas) (trakt.Lists, error) {
	return nil, nil
}
func (s *historyRemovalSpy) ListsGetAllMeta(context.Context) (trakt.Lists, error) {
	return nil, nil
}
func (s *historyRemovalSpy) RatingsAdd(context.Context, trakt.Items) error { return nil }
func (s *historyRemovalSpy) RatingsGet(context.Context) (trakt.Items, error) {
	return nil, nil
}
func (s *historyRemovalSpy) RatingsRemove(context.Context, trakt.Items) error { return nil }
func (s *historyRemovalSpy) WatchlistGet(context.Context) (*trakt.List, error) {
	return &trakt.List{}, nil
}
func (s *historyRemovalSpy) WatchlistItemsAdd(context.Context, trakt.Items) error {
	return nil
}
func (s *historyRemovalSpy) WatchlistItemsRemove(context.Context, trakt.Items) error {
	return nil
}

func newHistoryRemovalTestSyncer(mode appconfig.SyncMode, client trakt.API) *Syncer {
	history := true
	candidate := trakt.Item{
		Type: trakt.ItemTypeMovie,
		Movie: trakt.ItemSpec{
			IDMeta: trakt.IDMeta{IMDb: "tt1234567"},
		},
	}
	return &Syncer{
		logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		traktClient: client,
		conf: appconfig.Sync{
			Mode:    &mode,
			History: &history,
		},
		user: &user{
			imdbRatings:  map[string]imdb.Item{},
			traktRatings: map[string]trakt.Item{"tt1234567": candidate},
		},
	}
}

func TestSyncHistoryAddOnlySkipsRemovalLookups(t *testing.T) {
	client := &historyRemovalSpy{}
	syncer := newHistoryRemovalTestSyncer(appconfig.SyncModeAddOnly, client)

	if err := syncer.syncHistory(context.Background()); err != nil {
		t.Fatalf("syncHistory() error = %v", err)
	}
	if client.historyGetCalls != 0 {
		t.Fatalf("HistoryGet calls = %d, want 0", client.historyGetCalls)
	}
	if client.historyRemoveCalls != 0 {
		t.Fatalf("HistoryRemove calls = %d, want 0", client.historyRemoveCalls)
	}
}

func TestSyncHistoryFullModePreservesRemovalChecks(t *testing.T) {
	candidate := trakt.Item{
		Type: trakt.ItemTypeMovie,
		Movie: trakt.ItemSpec{
			IDMeta: trakt.IDMeta{IMDb: "tt1234567"},
		},
	}
	client := &historyRemovalSpy{historyGetResult: trakt.Items{candidate}}
	syncer := newHistoryRemovalTestSyncer(appconfig.SyncModeFull, client)

	if err := syncer.syncHistory(context.Background()); err != nil {
		t.Fatalf("syncHistory() error = %v", err)
	}
	if client.historyGetCalls != 1 {
		t.Fatalf("HistoryGet calls = %d, want 1", client.historyGetCalls)
	}
	if client.historyRemoveCalls != 1 {
		t.Fatalf("HistoryRemove calls = %d, want 1", client.historyRemoveCalls)
	}
}
