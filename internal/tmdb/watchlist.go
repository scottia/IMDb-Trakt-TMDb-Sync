package tmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	appconfig "github.com/cecobask/imdb-trakt-sync/internal/config"
	"github.com/cecobask/imdb-trakt-sync/internal/syncstate"
)

// SourceItem is the minimal IMDb identity needed for TMDb destination syncs.
type SourceItem struct {
	IMDbID string
	Kind   string
}

type watchlistItem struct {
	ID int `json:"id"`
}

type watchlistPage struct {
	Page       int             `json:"page"`
	Results    []watchlistItem `json:"results"`
	TotalPages int             `json:"total_pages"`
}

type watchlistRequest struct {
	MediaType string `json:"media_type"`
	MediaID   int    `json:"media_id"`
	Watchlist bool   `json:"watchlist"`
}

func SyncWatchlist(
	ctx context.Context,
	conf *appconfig.TMDb,
	logger *slog.Logger,
	items []SourceItem,
	state *syncstate.State,
	mode appconfig.SyncMode,
) error {
	if conf == nil || conf.Enabled == nil || !*conf.Enabled {
		return nil
	}
	if conf.ReadAccessToken == nil || strings.TrimSpace(*conf.ReadAccessToken) == "" {
		return fmt.Errorf("tmdb read access token must not be empty")
	}
	if conf.SessionID == nil || strings.TrimSpace(*conf.SessionID) == "" {
		return fmt.Errorf("tmdb session id must not be empty")
	}

	client := &apiClient{
		baseURL:   apiBaseURL,
		readToken: strings.TrimSpace(*conf.ReadAccessToken),
		sessionID: strings.TrimSpace(*conf.SessionID),
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
		logger: logger,
	}

	accountID, err := client.validateSession(ctx)
	if err != nil {
		return fmt.Errorf("failure validating tmdb api session: %w", err)
	}

	existing, err := client.fetchExistingWatchlist(ctx, accountID)
	if err != nil {
		return err
	}

	cachePath := ""
	if state != nil {
		cachePath = state.MappingCachePath()
	}
	cache, err := loadMappingCache(cachePath)
	if err != nil {
		logger.Warn("ignoring invalid tmdb mapping cache and rebuilding it", "error", err)
		cache = newMappingCache()
	}

	desired := make(map[string]target, len(items))
	failed := 0
	cacheHits := 0
	apiLookups := 0
	for _, item := range items {
		if strings.TrimSpace(item.IMDbID) == "" {
			failed++
			logger.Warn("ignoring tmdb watchlist item failure", "error", "empty imdb id")
			continue
		}
		resolved, fromCache, err := client.resolveWithCache(ctx, rating{IMDbID: item.IMDbID, Kind: item.Kind}, cache)
		if fromCache {
			cacheHits++
		} else if err == nil {
			apiLookups++
		}
		if err != nil {
			if isHardItemError(err) {
				return fmt.Errorf("failure resolving %s through tmdb api: %w", item.IMDbID, err)
			}
			failed++
			logger.Warn("ignoring tmdb watchlist item failure", "imdb_id", item.IMDbID, "error", err)
			continue
		}
		if resolved.kind != "movie" && resolved.kind != "tv" {
			failed++
			logger.Warn(
				"ignoring tmdb watchlist item failure",
				"imdb_id", item.IMDbID,
				"error", fmt.Sprintf("tmdb watchlist does not support target kind %q", resolved.kind),
			)
			continue
		}
		desired[resolved.key()] = resolved
	}

	toAdd := make([]target, 0)
	for key, item := range desired {
		if _, ok := existing[key]; !ok {
			toAdd = append(toAdd, item)
		}
	}
	toRemove := make([]target, 0)
	for key, item := range existing {
		if _, ok := desired[key]; !ok {
			toRemove = append(toRemove, item)
		}
	}

	if mode == appconfig.SyncModeDryRun {
		logger.Info(
			"sync would reconcile tmdb watchlist",
			"source", len(items),
			"resolved", len(desired),
			"add", len(toAdd),
			"remove", len(toRemove),
			"failed", failed,
			"cache_hits", cacheHits,
			"api_lookups", apiLookups,
		)
		return nil
	}

	added := 0
	for _, item := range toAdd {
		if err := client.setWatchlist(ctx, accountID, item, true); err != nil {
			if isHardItemError(err) {
				return fmt.Errorf("failure adding %s to tmdb watchlist: %w", item.key(), err)
			}
			failed++
			logger.Warn("ignoring tmdb watchlist add failure", "item", item.key(), "error", err)
			continue
		}
		added++
	}

	removed := 0
	if mode == appconfig.SyncModeFull {
		for _, item := range toRemove {
			if err := client.setWatchlist(ctx, accountID, item, false); err != nil {
				if isHardItemError(err) {
					return fmt.Errorf("failure removing %s from tmdb watchlist: %w", item.key(), err)
				}
				failed++
				logger.Warn("ignoring tmdb watchlist remove failure", "item", item.key(), "error", err)
				continue
			}
			removed++
		}
	} else if len(toRemove) > 0 {
		logger.Info("tmdb watchlist removals suppressed by sync mode", "count", len(toRemove), "mode", mode)
	}

	if err := saveMappingCache(cachePath, cache); err != nil {
		return fmt.Errorf("failure saving tmdb mapping cache: %w", err)
	}

	logger.Info(
		"tmdb api watchlist sync completed",
		"source", len(items),
		"resolved", len(desired),
		"added", added,
		"removed", removed,
		"unchanged", len(desired)-len(toAdd),
		"failed", failed,
		"cache_hits", cacheHits,
		"api_lookups", apiLookups,
	)
	return nil
}

func (c *apiClient) fetchExistingWatchlist(ctx context.Context, accountID int) (map[string]target, error) {
	type endpoint struct {
		kind string
		path string
	}
	endpoints := []endpoint{
		{kind: "movie", path: fmt.Sprintf("/account/%d/watchlist/movies", accountID)},
		{kind: "tv", path: fmt.Sprintf("/account/%d/watchlist/tv", accountID)},
	}

	existing := make(map[string]target)
	for _, endpoint := range endpoints {
		page := 1
		for {
			values := url.Values{}
			values.Set("session_id", c.sessionID)
			values.Set("page", strconv.Itoa(page))

			body, err := c.do(ctx, http.MethodGet, endpoint.path+"?"+values.Encode(), nil, http.StatusOK)
			if err != nil {
				return nil, fmt.Errorf("failure fetching tmdb %s watchlist page %d: %w", endpoint.kind, page, err)
			}
			var result watchlistPage
			if err := json.Unmarshal(body, &result); err != nil {
				return nil, fmt.Errorf("failure decoding tmdb %s watchlist page %d: %w", endpoint.kind, page, err)
			}
			for _, item := range result.Results {
				resolved := target{kind: endpoint.kind, id: item.ID}
				existing[resolved.key()] = resolved
			}
			if result.TotalPages <= 1 || page >= result.TotalPages {
				break
			}
			page++
		}
	}

	c.logger.Info("loaded existing tmdb watchlist", "count", len(existing))
	return existing, nil
}

func (c *apiClient) setWatchlist(ctx context.Context, accountID int, item target, value bool) error {
	if item.kind != "movie" && item.kind != "tv" {
		return &mappingError{message: fmt.Sprintf("unsupported tmdb watchlist target kind %q", item.kind)}
	}
	values := url.Values{}
	values.Set("session_id", c.sessionID)
	payload, err := json.Marshal(watchlistRequest{
		MediaType: item.kind,
		MediaID:   item.id,
		Watchlist: value,
	})
	if err != nil {
		return fmt.Errorf("failure encoding tmdb watchlist request: %w", err)
	}
	_, err = c.do(
		ctx,
		http.MethodPost,
		fmt.Sprintf("/account/%d/watchlist?%s", accountID, values.Encode()),
		payload,
		http.StatusOK,
		http.StatusCreated,
	)
	if err != nil {
		return fmt.Errorf("failure writing tmdb watchlist: %w", err)
	}
	return nil
}
