package config

import (
	"strings"
	"testing"
	"time"
)

func destinationTestConfig(t *testing.T, values map[string]interface{}) *Config {
	t.Helper()
	base := map[string]interface{}{
		"IMDB_AUTH":         string(IMDbAuthMethodNone),
		"IMDB_LISTS":        []string{},
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
	if conf.TMDb.Enabled == nil || !*conf.TMDb.Enabled {
		t.Fatal("TMDB_ENABLED default = false, want true")
	}
	if got := *conf.Trakt.Sync.Mode; got != SyncModeAddOnly {
		t.Fatalf("TRAKT_SYNC_MODE default = %q, want %q", got, SyncModeAddOnly)
	}
	if got := *conf.TMDb.Sync.Mode; got != SyncModeAddOnly {
		t.Fatalf("TMDB_SYNC_MODE default = %q, want %q", got, SyncModeAddOnly)
	}
	if !*conf.Trakt.Sync.History || !*conf.Trakt.Sync.Ratings || !*conf.Trakt.Sync.Watchlist || !*conf.Trakt.Sync.Lists {
		t.Fatal("all supported Trakt sync feature defaults should be enabled")
	}
	if !*conf.TMDb.Sync.Ratings || !*conf.TMDb.Sync.Watchlist {
		t.Fatal("all supported TMDb sync feature defaults should be enabled")
	}
	if *conf.TMDb.Sync.History || *conf.TMDb.Sync.Lists {
		t.Fatal("unsupported TMDb history/list defaults should remain disabled")
	}
	if got := *conf.Trakt.Sync.Timeout; got != 30*time.Minute {
		t.Fatalf("TRAKT_SYNC_TIMEOUT default = %v, want 30m", got)
	}
	if got := *conf.TMDb.Sync.Timeout; got != 30*time.Minute {
		t.Fatalf("TMDB_SYNC_TIMEOUT default = %v, want 30m", got)
	}
}

func TestValidateTMDbOnlyWithoutTraktCredentials(t *testing.T) {
	conf := destinationTestConfig(t, map[string]interface{}{
		"TRAKT_ENABLED":       false,
		"TMDB_ENABLED":        true,
		"TMDB_SYNC_RATINGS":   false,
		"TMDB_SYNC_WATCHLIST": false,
		"TMDB_SYNC_HISTORY":   false,
		"TMDB_SYNC_LISTS":     false,
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
	if got := *conf.Trakt.Sync.Mode; got != SyncModeAddOnly {
		t.Fatalf("TRAKT_SYNC_MODE legacy fallback = %q, want %q", got, SyncModeAddOnly)
	}
	if *conf.Trakt.Sync.Ratings {
		t.Fatal("TRAKT_SYNC_RATINGS legacy fallback = true, want false")
	}
	if !*conf.Trakt.Sync.Watchlist || !*conf.Trakt.Sync.Lists {
		t.Fatal("legacy Trakt watchlist/list fallbacks were not applied")
	}
	if *conf.TMDb.Sync.Lists {
		t.Fatal("legacy global list setting must not implicitly enable TMDb lists")
	}
	if !*conf.TMDb.Sync.Watchlist {
		t.Fatal("TMDb watchlist default should remain enabled")
	}
}
