package config

import (
	"strings"
	"testing"
)

func destinationTestConfig(t *testing.T, values map[string]interface{}) *Config {
	t.Helper()
	base := map[string]interface{}{
		"IMDB_AUTH":       string(IMDbAuthMethodNone),
		"IMDB_LISTS":      []string{},
		"IMDB_IGNOREDLISTS": []string{},
	}
	for key, value := range values {
		base[key] = value
	}
	conf, err := NewFromMap(base)
	if err != nil {
		t.Fatalf("NewFromMap() error = %v", err)
	}
	return conf
}

func TestDestinationDefaults(t *testing.T) {
	conf := destinationTestConfig(t, map[string]interface{}{})
	if conf.Trakt.Enabled == nil || !*conf.Trakt.Enabled {
		t.Fatal("TRAKT_ENABLED default = false, want true")
	}
	if conf.TMDb.Enabled == nil || *conf.TMDb.Enabled {
		t.Fatal("TMDB_ENABLED default = true, want false")
	}
	if got := *conf.Trakt.SyncMode; got != SyncModeDryRun {
		t.Fatalf("TRAKT_SYNC_MODE default = %q, want %q", got, SyncModeDryRun)
	}
	if got := *conf.TMDb.SyncMode; got != SyncModeDryRun {
		t.Fatalf("TMDB_SYNC_MODE default = %q, want %q", got, SyncModeDryRun)
	}
	if !*conf.Trakt.SyncRatings || !*conf.TMDb.SyncRatings {
		t.Fatal("ratings defaults should be enabled for both destinations")
	}
	if *conf.TMDb.SyncHistory || *conf.TMDb.SyncWatchlist || *conf.TMDb.SyncLists {
		t.Fatal("TMDb history/watchlist/lists defaults should be disabled")
	}
}

func TestValidateTMDbOnlyWithoutTraktCredentials(t *testing.T) {
	conf := destinationTestConfig(t, map[string]interface{}{
		"TRAKT_ENABLED":          false,
		"TMDB_ENABLED":           true,
		"TMDB_SYNC_RATINGS":      false,
		"TMDB_SYNC_WATCHLIST":    false,
		"TMDB_SYNC_HISTORY":      false,
		"TMDB_SYNC_LISTS":        false,
	})
	if err := conf.Validate(); err != nil {
		t.Fatalf("Validate() TMDb-only error = %v", err)
	}
}

func TestValidateTraktOnlyWithoutTMDbCredentials(t *testing.T) {
	conf := destinationTestConfig(t, map[string]interface{}{
		"TRAKT_ENABLED":      true,
		"TRAKT_CLIENTID":     "test-client-id",
		"TRAKT_CLIENTSECRET": "test-client-secret",
		"TMDB_ENABLED":       false,
	})
	if err := conf.Validate(); err != nil {
		t.Fatalf("Validate() Trakt-only error = %v", err)
	}
}

func TestValidateTMDbWatchlistRequiresSessionCredentials(t *testing.T) {
	conf := destinationTestConfig(t, map[string]interface{}{
		"TRAKT_ENABLED":       false,
		"TMDB_ENABLED":        true,
		"TMDB_SYNC_RATINGS":   false,
		"TMDB_SYNC_WATCHLIST": true,
	})
	if err := conf.Validate(); err == nil || !strings.Contains(err.Error(), "TMDB_READACCESSTOKEN") {
		t.Fatalf("Validate() error = %v, want TMDB_READACCESSTOKEN requirement", err)
	}
}

func TestValidateUnsupportedTMDbHistoryAndLists(t *testing.T) {
	t.Run("history", func(t *testing.T) {
		conf := destinationTestConfig(t, map[string]interface{}{
			"TRAKT_ENABLED":     false,
			"TMDB_ENABLED":      true,
			"TMDB_SYNC_RATINGS": false,
			"TMDB_SYNC_HISTORY": true,
		})
		if err := conf.Validate(); err == nil || !strings.Contains(err.Error(), "TMDB_SYNC_HISTORY") {
			t.Fatalf("Validate() error = %v, want TMDB_SYNC_HISTORY capability error", err)
		}
	})

	t.Run("lists", func(t *testing.T) {
		conf := destinationTestConfig(t, map[string]interface{}{
			"TRAKT_ENABLED":     false,
			"TMDB_ENABLED":      true,
			"TMDB_SYNC_RATINGS": false,
			"TMDB_SYNC_LISTS":   true,
		})
		if err := conf.Validate(); err == nil || !strings.Contains(err.Error(), "TMDB_SYNC_LISTS") {
			t.Fatalf("Validate() error = %v, want TMDB_SYNC_LISTS capability error", err)
		}
	})
}

func TestLegacySyncFallbackDoesNotEnableTMDbWatchlistOrLists(t *testing.T) {
	conf := destinationTestConfig(t, map[string]interface{}{
		"SYNC_MODE":      string(SyncModeAddOnly),
		"SYNC_RATINGS":   false,
		"SYNC_WATCHLIST": true,
		"SYNC_LISTS":     true,
	})
	if got := *conf.Trakt.SyncMode; got != SyncModeAddOnly {
		t.Fatalf("TRAKT_SYNC_MODE legacy fallback = %q, want %q", got, SyncModeAddOnly)
	}
	if *conf.Trakt.SyncRatings {
		t.Fatal("TRAKT_SYNC_RATINGS legacy fallback = true, want false")
	}
	if !*conf.Trakt.SyncWatchlist || !*conf.Trakt.SyncLists {
		t.Fatal("legacy Trakt watchlist/list fallbacks were not applied")
	}
	if *conf.TMDb.SyncWatchlist || *conf.TMDb.SyncLists {
		t.Fatal("legacy global watchlist/list settings must not implicitly enable TMDb features")
	}
}
