package tmdb

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	appconfig "github.com/cecobask/imdb-trakt-sync/internal/config"
	"github.com/cecobask/imdb-trakt-sync/internal/syncstate"
)

const (
	apiBaseURL          = "https://api.themoviedb.org/3"
	mappingCacheVersion = 1
)

// BrowserOptions is retained for compatibility with the existing syncer
// constructor. The API-backed TMDb importer does not launch a browser.
type BrowserOptions struct {
	BrowserPath string
	Headless    bool
	Trace       bool
}

type rating struct {
	IMDbID string
	Kind   string
	Value  float64
}

type findResult struct {
	ID            int `json:"id"`
	ShowID        int `json:"show_id"`
	SeasonNumber  int `json:"season_number"`
	EpisodeNumber int `json:"episode_number"`
}

type findResponse struct {
	MovieResults     []findResult `json:"movie_results"`
	TVResults        []findResult `json:"tv_results"`
	TVEpisodeResults []findResult `json:"tv_episode_results"`
}

type target struct {
	kind          string
	id            int
	showID        int
	seasonNumber  int
	episodeNumber int
}

func (t target) key() string {
	return fmt.Sprintf("%s:%d", t.kind, t.id)
}

type cachedTarget struct {
	SourceKind    string `json:"source_kind"`
	Kind          string `json:"kind"`
	ID            int    `json:"id"`
	ShowID        int    `json:"show_id,omitempty"`
	SeasonNumber  int    `json:"season_number,omitempty"`
	EpisodeNumber int    `json:"episode_number,omitempty"`
}

func (c cachedTarget) target() target {
	return target{
		kind:          c.Kind,
		id:            c.ID,
		showID:        c.ShowID,
		seasonNumber:  c.SeasonNumber,
		episodeNumber: c.EpisodeNumber,
	}
}

func newCachedTarget(sourceKind string, t target) cachedTarget {
	return cachedTarget{
		SourceKind:    sourceKind,
		Kind:          t.kind,
		ID:            t.id,
		ShowID:        t.showID,
		SeasonNumber:  t.seasonNumber,
		EpisodeNumber: t.episodeNumber,
	}
}

type mappingCache struct {
	Version  int                     `json:"version"`
	Mappings map[string]cachedTarget `json:"mappings"`
}

type accountDetails struct {
	ID int `json:"id"`
}

type ratedItem struct {
	ID     int     `json:"id"`
	Rating float64 `json:"rating"`
}

type ratedPage struct {
	Page       int         `json:"page"`
	Results    []ratedItem `json:"results"`
	TotalPages int         `json:"total_pages"`
}

type apiClient struct {
	baseURL    string
	readToken  string
	sessionID  string
	httpClient *http.Client
	logger     *slog.Logger
}

type apiStatusError struct {
	status  int
	message string
}

func (e *apiStatusError) Error() string {
	return fmt.Sprintf("tmdb api returned status %d: %s", e.status, e.message)
}

type mappingError struct {
	message string
}

func (e *mappingError) Error() string {
	return e.message
}

func ImportRatings(
	ctx context.Context,
	conf *appconfig.TMDb,
	_ BrowserOptions,
	logger *slog.Logger,
	data []byte,
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
	if len(data) == 0 {
		return fmt.Errorf("imdb ratings csv must not be empty")
	}
	if state != nil && state.Bootstrap() {
		logger.Info("tmdb ratings bootstrap skipped; current imdb snapshot will become the baseline after successful destinations")
		return nil
	}

	ratings, err := parseRatingsCSV(data)
	if err != nil {
		return fmt.Errorf("failure parsing imdb ratings csv for tmdb api sync: %w", err)
	}

	fullReconciliation := state == nil || state.FullReconciliation()
	var removed map[string]syncstate.Rating
	if state != nil {
		delta := state.Delta()
		removed = delta.Remove
		if !fullReconciliation {
			upserts := delta.UpsertIDs()
			filtered := make([]rating, 0, len(upserts))
			for _, item := range ratings {
				if _, ok := upserts[item.IMDbID]; ok {
					filtered = append(filtered, item)
				}
			}
			ratings = filtered
		}
	}

	if len(ratings) == 0 && (mode != appconfig.SyncModeFull || len(removed) == 0) {
		logger.Info("no tmdb rating source changes to process")
		return nil
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

	cachePath := ""
	if state != nil {
		cachePath = state.MappingCachePath()
	}
	cache, err := loadMappingCache(cachePath)
	if err != nil {
		logger.Warn("ignoring invalid tmdb mapping cache and rebuilding it", "error", err)
		cache = newMappingCache()
	}

	var existing map[string]float64
	if fullReconciliation {
		existing, err = client.fetchExistingRatings(ctx, accountID)
		if err != nil {
			return fmt.Errorf("failure fetching existing tmdb ratings: %w", err)
		}
	}

	var (
		added      int
		updated    int
		unchanged  int
		removedOK  int
		failed     int
		cacheHits  int
		apiLookups int
	)
	for i, item := range ratings {
		if item.Value < 0.5 || item.Value > 10 {
			failed++
			logger.Warn("ignoring tmdb rating item failure", "imdb_id", item.IMDbID, "error", fmt.Sprintf("rating %.1f is outside tmdb range 0.5-10", item.Value))
			continue
		}

		resolved, fromCache, err := client.resolveWithCache(ctx, item, cache)
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
			logger.Warn("ignoring tmdb rating item failure", "imdb_id", item.IMDbID, "error", err)
			continue
		}

		if fullReconciliation {
			current, found := existing[resolved.key()]
			if found && ratingsEqual(current, item.Value) {
				unchanged++
				continue
			}
			if err := client.addRating(ctx, resolved, item.Value); err != nil {
				if isHardItemError(err) {
					return fmt.Errorf("failure writing %s rating to tmdb api: %w", item.IMDbID, err)
				}
				failed++
				logger.Warn("ignoring tmdb rating item failure", "imdb_id", item.IMDbID, "error", err)
				continue
			}
			existing[resolved.key()] = item.Value
			if found {
				updated++
			} else {
				added++
			}
		} else {
			// Incremental mode consumes only the IMDb source delta. Replaying a
			// small delta is intentionally idempotent if another destination
			// failed on the previous run and the baseline was not advanced.
			if err := client.addRating(ctx, resolved, item.Value); err != nil {
				if isHardItemError(err) {
					return fmt.Errorf("failure writing %s rating to tmdb api: %w", item.IMDbID, err)
				}
				failed++
				logger.Warn("ignoring tmdb rating item failure", "imdb_id", item.IMDbID, "error", err)
				continue
			}
			updated++
		}

		if (i+1)%50 == 0 || i+1 == len(ratings) {
			logger.Info(
				"tmdb api ratings progress",
				"processed", i+1,
				"total", len(ratings),
				"added", added,
				"updated", updated,
				"unchanged", unchanged,
				"failed", failed,
				"cache_hits", cacheHits,
				"api_lookups", apiLookups,
			)
		}
	}

	if len(removed) > 0 {
		if mode != appconfig.SyncModeFull {
			logger.Info("tmdb rating removals suppressed by sync mode", "count", len(removed), "mode", mode)
		} else {
			for imdbID, previous := range removed {
				item := rating{IMDbID: imdbID, Kind: previous.Kind, Value: previous.Value}
				resolved, fromCache, err := client.resolveWithCache(ctx, item, cache)
				if fromCache {
					cacheHits++
				} else if err == nil {
					apiLookups++
				}
				if err != nil {
					if isHardItemError(err) {
						return fmt.Errorf("failure resolving removed imdb rating %s through tmdb api: %w", imdbID, err)
					}
					failed++
					logger.Warn("ignoring tmdb removed-rating item failure", "imdb_id", imdbID, "error", err)
					continue
				}
				if err := client.deleteRating(ctx, resolved); err != nil {
					if isHardItemError(err) {
						return fmt.Errorf("failure deleting %s rating from tmdb api: %w", imdbID, err)
					}
					failed++
					logger.Warn("ignoring tmdb removed-rating item failure", "imdb_id", imdbID, "error", err)
					continue
				}
				removedOK++
			}
		}
	}

	if err := saveMappingCache(cachePath, cache); err != nil {
		return fmt.Errorf("failure saving tmdb mapping cache: %w", err)
	}

	if failed > 0 {
		logger.Warn(
			"tmdb api ratings sync completed with ignored item failures",
			"processed", len(ratings),
			"added", added,
			"updated", updated,
			"unchanged", unchanged,
			"removed", removedOK,
			"failed", failed,
			"cache_hits", cacheHits,
			"api_lookups", apiLookups,
			"full_reconciliation", fullReconciliation,
		)
		return nil
	}

	logger.Info(
		"tmdb api ratings sync completed",
		"processed", len(ratings),
		"added", added,
		"updated", updated,
		"unchanged", unchanged,
		"removed", removedOK,
		"failed", failed,
		"cache_hits", cacheHits,
		"api_lookups", apiLookups,
		"full_reconciliation", fullReconciliation,
	)
	return nil
}

func parseRatingsCSV(data []byte) ([]rating, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failure reading csv records: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("expected csv records to contain a header row")
	}

	header := make(map[string]int, len(records[0]))
	for i, name := range records[0] {
		header[strings.TrimSpace(name)] = i
	}

	constIdx, ok := header["Const"]
	if !ok {
		return nil, fmt.Errorf("missing Const column")
	}
	ratingIdx, ok := header["Your Rating"]
	if !ok {
		return nil, fmt.Errorf("missing Your Rating column")
	}
	kindIdx, ok := header["Title Type"]
	if !ok {
		return nil, fmt.Errorf("missing Title Type column")
	}

	maxIdx := constIdx
	if ratingIdx > maxIdx {
		maxIdx = ratingIdx
	}
	if kindIdx > maxIdx {
		maxIdx = kindIdx
	}

	ratings := make([]rating, 0, len(records)-1)
	for rowNum, record := range records[1:] {
		if len(record) <= maxIdx {
			return nil, fmt.Errorf("csv row %d has too few columns", rowNum+2)
		}
		imdbID := strings.TrimSpace(record[constIdx])
		if imdbID == "" {
			return nil, fmt.Errorf("csv row %d has empty Const value", rowNum+2)
		}
		value, err := strconv.ParseFloat(strings.TrimSpace(record[ratingIdx]), 64)
		if err != nil {
			return nil, fmt.Errorf("csv row %d has invalid rating: %w", rowNum+2, err)
		}
		ratings = append(ratings, rating{
			IMDbID: imdbID,
			Kind:   strings.TrimSpace(record[kindIdx]),
			Value:  value,
		})
	}
	return ratings, nil
}

func newMappingCache() *mappingCache {
	return &mappingCache{
		Version:  mappingCacheVersion,
		Mappings: make(map[string]cachedTarget),
	}
}

func loadMappingCache(path string) (*mappingCache, error) {
	if strings.TrimSpace(path) == "" {
		return newMappingCache(), nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return newMappingCache(), nil
	}
	if err != nil {
		return nil, err
	}
	var cache mappingCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	if cache.Version != mappingCacheVersion {
		return nil, fmt.Errorf("unsupported tmdb mapping cache version %d", cache.Version)
	}
	if cache.Mappings == nil {
		cache.Mappings = make(map[string]cachedTarget)
	}
	return &cache, nil
}

func saveMappingCache(path string, cache *mappingCache) error {
	if strings.TrimSpace(path) == "" || cache == nil {
		return nil
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (c *apiClient) validateSession(ctx context.Context) (int, error) {
	values := url.Values{}
	values.Set("session_id", c.sessionID)
	body, err := c.do(ctx, http.MethodGet, "/account?"+values.Encode(), nil, http.StatusOK)
	if err != nil {
		return 0, err
	}
	var account accountDetails
	if err := json.Unmarshal(body, &account); err != nil {
		return 0, fmt.Errorf("failure decoding tmdb account response: %w", err)
	}
	if account.ID == 0 {
		return 0, fmt.Errorf("tmdb account response did not contain a valid account id")
	}
	return account.ID, nil
}

func (c *apiClient) fetchExistingRatings(ctx context.Context, accountID int) (map[string]float64, error) {
	type endpoint struct {
		kind string
		path string
	}
	endpoints := []endpoint{
		{kind: "movie", path: fmt.Sprintf("/account/%d/rated/movies", accountID)},
		{kind: "tv", path: fmt.Sprintf("/account/%d/rated/tv", accountID)},
		{kind: "episode", path: fmt.Sprintf("/account/%d/rated/tv/episodes", accountID)},
	}

	existing := make(map[string]float64)
	for _, endpoint := range endpoints {
		page := 1
		for {
			values := url.Values{}
			values.Set("session_id", c.sessionID)
			values.Set("page", strconv.Itoa(page))

			body, err := c.do(ctx, http.MethodGet, endpoint.path+"?"+values.Encode(), nil, http.StatusOK)
			if err != nil {
				return nil, fmt.Errorf("failure fetching %s ratings page %d: %w", endpoint.kind, page, err)
			}
			var result ratedPage
			if err := json.Unmarshal(body, &result); err != nil {
				return nil, fmt.Errorf("failure decoding %s ratings page %d: %w", endpoint.kind, page, err)
			}
			for _, item := range result.Results {
				existing[fmt.Sprintf("%s:%d", endpoint.kind, item.ID)] = item.Rating
			}

			if result.TotalPages <= 1 || page >= result.TotalPages {
				break
			}
			page++
		}
	}

	c.logger.Info("loaded existing tmdb ratings", "count", len(existing))
	return existing, nil
}

func (c *apiClient) resolveWithCache(ctx context.Context, item rating, cache *mappingCache) (target, bool, error) {
	if cache != nil {
		if cached, ok := cache.Mappings[item.IMDbID]; ok && cached.SourceKind == item.Kind && cached.ID != 0 {
			return cached.target(), true, nil
		}
	}
	resolved, err := c.findByIMDbID(ctx, item)
	if err != nil {
		return target{}, false, err
	}
	if cache != nil {
		cache.Mappings[item.IMDbID] = newCachedTarget(item.Kind, resolved)
	}
	return resolved, false, nil
}

func (c *apiClient) findByIMDbID(ctx context.Context, item rating) (target, error) {
	values := url.Values{}
	values.Set("external_source", "imdb_id")

	body, err := c.do(
		ctx,
		http.MethodGet,
		"/find/"+url.PathEscape(item.IMDbID)+"?"+values.Encode(),
		nil,
		http.StatusOK,
	)
	if err != nil {
		return target{}, fmt.Errorf("failure resolving imdb id through tmdb find api: %w", err)
	}

	var result findResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return target{}, fmt.Errorf("failure decoding tmdb find response: %w", err)
	}

	switch item.Kind {
	case "Movie":
		if len(result.MovieResults) != 1 {
			return target{}, resolutionError(item, result)
		}
		return target{kind: "movie", id: result.MovieResults[0].ID}, nil
	case "TV Series", "TV Mini Series":
		if len(result.TVResults) != 1 {
			return target{}, resolutionError(item, result)
		}
		return target{kind: "tv", id: result.TVResults[0].ID}, nil
	case "TV Episode":
		if len(result.TVEpisodeResults) != 1 {
			return target{}, resolutionError(item, result)
		}
		episode := result.TVEpisodeResults[0]
		return target{
			kind:          "episode",
			id:            episode.ID,
			showID:        episode.ShowID,
			seasonNumber:  episode.SeasonNumber,
			episodeNumber: episode.EpisodeNumber,
		}, nil
	default:
		return resolveSingleExactResult(item, result)
	}
}

func resolveSingleExactResult(item rating, result findResponse) (target, error) {
	total := len(result.MovieResults) + len(result.TVResults) + len(result.TVEpisodeResults)
	if total != 1 {
		return target{}, resolutionError(item, result)
	}
	if len(result.MovieResults) == 1 {
		return target{kind: "movie", id: result.MovieResults[0].ID}, nil
	}
	if len(result.TVResults) == 1 {
		return target{kind: "tv", id: result.TVResults[0].ID}, nil
	}
	episode := result.TVEpisodeResults[0]
	return target{
		kind:          "episode",
		id:            episode.ID,
		showID:        episode.ShowID,
		seasonNumber:  episode.SeasonNumber,
		episodeNumber: episode.EpisodeNumber,
	}, nil
}

func resolutionError(item rating, result findResponse) error {
	return &mappingError{message: fmt.Sprintf(
		"no unambiguous exact tmdb mapping for imdb id (title type %q; movie=%d tv=%d episode=%d)",
		item.Kind,
		len(result.MovieResults),
		len(result.TVResults),
		len(result.TVEpisodeResults),
	)}
}

func ratingsEqual(a, b float64) bool {
	return math.Abs(a-b) < 0.001
}

func isHardItemError(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var mappingErr *mappingError
	if errors.As(err, &mappingErr) {
		return false
	}
	var statusErr *apiStatusError
	if errors.As(err, &statusErr) {
		return statusErr.status == http.StatusUnauthorized ||
			statusErr.status == http.StatusForbidden ||
			statusErr.status == http.StatusTooManyRequests ||
			statusErr.status >= 500
	}
	return true
}

func (c *apiClient) ratingPath(item target) (string, error) {
	switch item.kind {
	case "movie":
		return fmt.Sprintf("/movie/%d/rating", item.id), nil
	case "tv":
		return fmt.Sprintf("/tv/%d/rating", item.id), nil
	case "episode":
		if item.showID == 0 {
			return "", &mappingError{message: "tmdb episode mapping did not include show_id"}
		}
		return fmt.Sprintf(
			"/tv/%d/season/%d/episode/%d/rating",
			item.showID,
			item.seasonNumber,
			item.episodeNumber,
		), nil
	default:
		return "", &mappingError{message: fmt.Sprintf("unsupported tmdb target kind %q", item.kind)}
	}
}

func (c *apiClient) addRating(ctx context.Context, item target, value float64) error {
	path, err := c.ratingPath(item)
	if err != nil {
		return err
	}
	values := url.Values{}
	values.Set("session_id", c.sessionID)
	payload, err := json.Marshal(map[string]float64{"value": value})
	if err != nil {
		return fmt.Errorf("failure encoding tmdb rating request: %w", err)
	}

	_, err = c.do(
		ctx,
		http.MethodPost,
		path+"?"+values.Encode(),
		payload,
		http.StatusOK,
		http.StatusCreated,
	)
	if err != nil {
		return fmt.Errorf("failure writing tmdb rating: %w", err)
	}
	return nil
}

func (c *apiClient) deleteRating(ctx context.Context, item target) error {
	path, err := c.ratingPath(item)
	if err != nil {
		return err
	}
	values := url.Values{}
	values.Set("session_id", c.sessionID)
	_, err = c.do(ctx, http.MethodDelete, path+"?"+values.Encode(), nil, http.StatusOK, http.StatusNoContent)
	if err != nil {
		return fmt.Errorf("failure deleting tmdb rating: %w", err)
	}
	return nil
}

func (c *apiClient) do(
	ctx context.Context,
	method string,
	path string,
	payload []byte,
	expected ...int,
) ([]byte, error) {
	for attempt := 0; attempt < 3; attempt++ {
		var body io.Reader = http.NoBody
		if payload != nil {
			body = bytes.NewReader(payload)
		}

		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
		if err != nil {
			return nil, fmt.Errorf("failure creating tmdb request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.readToken)
		req.Header.Set("Accept", "application/json")
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if attempt < 2 {
				if err := waitForRetry(ctx, time.Duration(attempt+1)*time.Second); err != nil {
					return nil, err
				}
				continue
			}
			return nil, fmt.Errorf("failure sending tmdb request: %w", err)
		}

		responseBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("failure reading tmdb response: %w", readErr)
		}

		if statusAllowed(resp.StatusCode, expected) {
			return responseBody, nil
		}

		if (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500) && attempt < 2 {
			delay := retryDelay(resp, attempt)
			if err := waitForRetry(ctx, delay); err != nil {
				return nil, err
			}
			continue
		}

		message := strings.TrimSpace(string(responseBody))
		if len(message) > 500 {
			message = message[:500] + "..."
		}
		return nil, &apiStatusError{status: resp.StatusCode, message: message}
	}
	return nil, fmt.Errorf("tmdb request exhausted retries")
}

func statusAllowed(status int, expected []int) bool {
	for _, value := range expected {
		if status == value {
			return true
		}
	}
	return false
}

func retryDelay(resp *http.Response, attempt int) time.Duration {
	if value := strings.TrimSpace(resp.Header.Get("Retry-After")); value != "" {
		if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}
	return time.Duration(attempt+1) * time.Second
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
