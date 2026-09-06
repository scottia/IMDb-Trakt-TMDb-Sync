package tmdb

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	appconfig "github.com/cecobask/imdb-trakt-sync/internal/config"
)

const apiBaseURL = "https://api.themoviedb.org/3"

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

type apiClient struct {
	baseURL    string
	readToken  string
	sessionID  string
	httpClient *http.Client
	logger     *slog.Logger
}

func ImportRatings(
	ctx context.Context,
	conf *appconfig.TMDb,
	_ BrowserOptions,
	logger *slog.Logger,
	data []byte,
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

	ratings, err := parseRatingsCSV(data)
	if err != nil {
		return fmt.Errorf("failure parsing imdb ratings csv for tmdb api sync: %w", err)
	}
	if len(ratings) == 0 {
		logger.Info("no imdb ratings to sync to tmdb api")
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
	if err := client.validateSession(ctx); err != nil {
		return fmt.Errorf("failure validating tmdb api session: %w", err)
	}

	var failures []string
	succeeded := 0
	for i, item := range ratings {
		if item.Value < 0.5 || item.Value > 10 {
			failures = append(failures, fmt.Sprintf("%s: rating %.1f is outside tmdb range 0.5-10", item.IMDbID, item.Value))
			continue
		}

		resolved, err := client.findByIMDbID(ctx, item)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", item.IMDbID, err))
			continue
		}
		if err := client.addRating(ctx, resolved, item.Value); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", item.IMDbID, err))
			continue
		}
		succeeded++

		if (i+1)%50 == 0 || i+1 == len(ratings) {
			logger.Info(
				"tmdb api ratings progress",
				"processed", i+1,
				"total", len(ratings),
				"succeeded", succeeded,
				"failed", len(failures),
			)
		}
	}

	if len(failures) > 0 {
		for _, failure := range failures {
			logger.Warn("tmdb api rating failed", "item", failure)
		}
		return fmt.Errorf(
			"tmdb api ratings sync completed with %d failures out of %d",
			len(failures),
			len(ratings),
		)
	}

	logger.Info("synced imdb ratings to tmdb api", "count", succeeded)
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

func (c *apiClient) validateSession(ctx context.Context) error {
	values := url.Values{}
	values.Set("session_id", c.sessionID)
	_, err := c.do(ctx, http.MethodGet, "/account?"+values.Encode(), nil, http.StatusOK)
	if err != nil {
		return err
	}
	return nil
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
	return fmt.Errorf(
		"no unambiguous exact tmdb mapping for imdb id (title type %q; movie=%d tv=%d episode=%d)",
		item.Kind,
		len(result.MovieResults),
		len(result.TVResults),
		len(result.TVEpisodeResults),
	)
}

func (c *apiClient) addRating(ctx context.Context, item target, value float64) error {
	var path string
	switch item.kind {
	case "movie":
		path = fmt.Sprintf("/movie/%d/rating", item.id)
	case "tv":
		path = fmt.Sprintf("/tv/%d/rating", item.id)
	case "episode":
		if item.showID == 0 {
			return fmt.Errorf("tmdb episode mapping did not include show_id")
		}
		path = fmt.Sprintf(
			"/tv/%d/season/%d/episode/%d/rating",
			item.showID,
			item.seasonNumber,
			item.episodeNumber,
		)
	default:
		return fmt.Errorf("unsupported tmdb target kind %q", item.kind)
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
		return nil, fmt.Errorf("tmdb api returned status %d: %s", resp.StatusCode, message)
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
