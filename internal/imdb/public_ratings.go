package imdb

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	publicRatingsGraphQLEndpoint = "https://api.graphql.imdb.com/"
	publicRatingsPageSize        = 250
)

var (
	publicRatingsUserIDPattern    = regexp.MustCompile(`^ur\d{6,12}$`)
	publicRatingsProfileIDPattern = regexp.MustCompile(`^p\.[A-Za-z0-9]{16,40}$`)
)

const publicProfileQuery = `query Profile($id: ID!) {
  userProfile(input: { profileId: $id }) { userId }
}`

const publicRatingsQuery = `query UserRatings($userId: ID!, $first: Int!, $after: String) {
  userRatings(userId: $userId, first: $first, after: $after) {
    total
    pageInfo { endCursor hasNextPage }
    edges {
      node {
        userRating { value date }
        title {
          id
          titleText { text }
          titleType { id isEpisode isSeries }
        }
      }
    }
  }
}`

type publicRatingsClient struct {
	ctx        context.Context
	httpClient *http.Client
	endpoint   string
	logger     *slog.Logger
	userRef    string
	ratings    Items
	ratingsCSV []byte
}

type graphQLError struct {
	Message string `json:"message"`
}

type publicProfileResponse struct {
	Data struct {
		UserProfile *struct {
			UserID string `json:"userId"`
		} `json:"userProfile"`
	} `json:"data"`
	Errors []graphQLError `json:"errors"`
}

type publicRatingsConnection struct {
	Total    int `json:"total"`
	PageInfo struct {
		EndCursor   string `json:"endCursor"`
		HasNextPage bool   `json:"hasNextPage"`
	} `json:"pageInfo"`
	Edges []struct {
		Node *struct {
			UserRating struct {
				Value float64 `json:"value"`
				Date  string  `json:"date"`
			} `json:"userRating"`
			Title struct {
				ID        string `json:"id"`
				TitleText struct {
					Text string `json:"text"`
				} `json:"titleText"`
				TitleType struct {
					ID        string `json:"id"`
					IsEpisode bool   `json:"isEpisode"`
					IsSeries  bool   `json:"isSeries"`
				} `json:"titleType"`
			} `json:"title"`
		} `json:"node"`
	} `json:"edges"`
}

type publicRatingsResponse struct {
	Data struct {
		UserRatings *publicRatingsConnection `json:"userRatings"`
	} `json:"data"`
	Errors []graphQLError `json:"errors"`
}

type publicRatingRow struct {
	ID      string
	Title   string
	Kind    string
	Rating  float64
	RatedAt time.Time
}

func NewPublicRatingsAPI(ctx context.Context, ratingsUserList string, logger *slog.Logger) (API, error) {
	return newPublicRatingsAPI(ctx, ratingsUserList, logger, http.DefaultClient, publicRatingsGraphQLEndpoint)
}

func newPublicRatingsAPI(
	ctx context.Context,
	ratingsUserList string,
	logger *slog.Logger,
	httpClient *http.Client,
	endpoint string,
) (API, error) {
	userRef, err := parseRatingsUserList(ratingsUserList)
	if err != nil {
		return nil, err
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &publicRatingsClient{
		ctx:        ctx,
		httpClient: httpClient,
		endpoint:   endpoint,
		logger:     logger,
		userRef:    userRef,
	}, nil
}

func parseRatingsUserList(value string) (string, error) {
	value = strings.TrimSpace(value)
	if publicRatingsUserIDPattern.MatchString(value) || publicRatingsProfileIDPattern.MatchString(value) {
		return value, nil
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("invalid IMDb ratings user list: %w", err)
	}
	host := strings.ToLower(parsed.Hostname())
	if parsed.Scheme != "https" || host != "imdb.com" && host != "www.imdb.com" {
		return "", fmt.Errorf("IMDb ratings user list must be an https imdb.com URL or user id")
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 3 || parts[0] != "user" || parts[2] != "ratings" {
		return "", fmt.Errorf("IMDb ratings user list URL must use /user/<id>/ratings")
	}
	userRef := parts[1]
	if !publicRatingsUserIDPattern.MatchString(userRef) && !publicRatingsProfileIDPattern.MatchString(userRef) {
		return "", fmt.Errorf("IMDb ratings user list contains an unsupported user id")
	}
	return userRef, nil
}

func (c *publicRatingsClient) ListsExport(...string) error {
	return fmt.Errorf("public IMDb ratings source does not provide custom lists")
}

func (c *publicRatingsClient) ListsGet(...string) (Lists, error) {
	return nil, fmt.Errorf("public IMDb ratings source does not provide custom lists")
}

func (c *publicRatingsClient) WatchlistExport() error {
	return fmt.Errorf("public IMDb ratings source does not provide watchlist data")
}

func (c *publicRatingsClient) WatchlistGet() (*List, error) {
	return nil, fmt.Errorf("public IMDb ratings source does not provide watchlist data")
}

func (c *publicRatingsClient) RatingsExport() error {
	userID, err := c.resolveUserID()
	if err != nil {
		return fmt.Errorf("failure resolving public IMDb ratings user: %w", err)
	}
	summary, err := c.fetchRatings(userID, 1, "")
	if err != nil {
		return fmt.Errorf("failure reading public IMDb ratings summary: %w", err)
	}
	total := summary.Data.UserRatings.Total
	if total < 0 || total > 100000 {
		return fmt.Errorf("public IMDb ratings count %d is outside the supported range", total)
	}
	response := summary
	if total > 1 {
		response, err = c.fetchAllRatings(userID, total)
		if err != nil {
			return fmt.Errorf("failure reading public IMDb ratings: %w", err)
		}
	}
	rows, err := normalizePublicRatings(response)
	if err != nil {
		return err
	}
	if len(rows) != total {
		return fmt.Errorf("IMDb reported %d public ratings but returned %d complete rows", total, len(rows))
	}
	items, data, err := encodePublicRatings(rows)
	if err != nil {
		return err
	}
	c.ratings = items
	c.ratingsCSV = data
	c.logger.Info("downloaded public imdb ratings", "count", len(items))
	return nil
}

func (c *publicRatingsClient) RatingsGet() (Items, error) {
	if c.ratingsCSV == nil {
		return nil, fmt.Errorf("public IMDb ratings have not been downloaded")
	}
	out := make(Items, len(c.ratings))
	copy(out, c.ratings)
	return out, nil
}

func (c *publicRatingsClient) RatingsCSV() []byte {
	return bytes.Clone(c.ratingsCSV)
}

func (c *publicRatingsClient) resolveUserID() (string, error) {
	if publicRatingsUserIDPattern.MatchString(c.userRef) {
		return c.userRef, nil
	}
	var response publicProfileResponse
	if err := c.query(publicProfileQuery, map[string]any{"id": c.userRef}, &response); err != nil {
		return "", err
	}
	if response.Data.UserProfile == nil || !publicRatingsUserIDPattern.MatchString(response.Data.UserProfile.UserID) {
		return "", fmt.Errorf("IMDb did not resolve the public ratings profile to a legacy user id")
	}
	return response.Data.UserProfile.UserID, nil
}

func (c *publicRatingsClient) fetchAllRatings(userID string, total int) (*publicRatingsResponse, error) {
	aggregate := &publicRatingsResponse{}
	aggregate.Data.UserRatings = &publicRatingsConnection{
		Total: total,
		Edges: make([]struct {
			Node *struct {
				UserRating struct {
					Value float64 `json:"value"`
					Date  string  `json:"date"`
				} `json:"userRating"`
				Title struct {
					ID        string `json:"id"`
					TitleText struct {
						Text string `json:"text"`
					} `json:"titleText"`
					TitleType struct {
						ID        string `json:"id"`
						IsEpisode bool   `json:"isEpisode"`
						IsSeries  bool   `json:"isSeries"`
					} `json:"titleType"`
				} `json:"title"`
			} `json:"node"`
		}, 0, total),
	}
	after := ""
	seenCursors := make(map[string]struct{})
	for page := 1; ; page++ {
		response, err := c.fetchRatings(userID, publicRatingsPageSize, after)
		if err != nil {
			return nil, fmt.Errorf("page %d: %w", page, err)
		}
		connection := response.Data.UserRatings
		if connection.Total != total {
			return nil, fmt.Errorf("public IMDb ratings total changed during pagination from %d to %d", total, connection.Total)
		}
		aggregate.Data.UserRatings.Edges = append(aggregate.Data.UserRatings.Edges, connection.Edges...)
		if len(aggregate.Data.UserRatings.Edges) > total {
			return nil, fmt.Errorf("IMDb reported %d public ratings but pagination returned at least %d rows", total, len(aggregate.Data.UserRatings.Edges))
		}
		if !connection.PageInfo.HasNextPage {
			break
		}
		cursor := strings.TrimSpace(connection.PageInfo.EndCursor)
		if cursor == "" {
			return nil, fmt.Errorf("IMDb indicated another ratings page without an end cursor")
		}
		if _, duplicate := seenCursors[cursor]; duplicate {
			return nil, fmt.Errorf("IMDb repeated ratings pagination cursor %q", cursor)
		}
		if len(connection.Edges) == 0 {
			return nil, fmt.Errorf("IMDb indicated another ratings page after returning no rows")
		}
		seenCursors[cursor] = struct{}{}
		after = cursor
	}
	if len(aggregate.Data.UserRatings.Edges) != total {
		return nil, fmt.Errorf("IMDb reported %d public ratings but pagination returned %d rows", total, len(aggregate.Data.UserRatings.Edges))
	}
	return aggregate, nil
}

func (c *publicRatingsClient) fetchRatings(userID string, first int, after string) (*publicRatingsResponse, error) {
	variables := map[string]any{"userId": userID, "first": first}
	if after != "" {
		variables["after"] = after
	}
	var response publicRatingsResponse
	if err := c.query(publicRatingsQuery, variables, &response); err != nil {
		return nil, err
	}
	if response.Data.UserRatings == nil {
		return nil, fmt.Errorf("IMDb returned no public ratings data")
	}
	return &response, nil
}

func (c *publicRatingsClient) query(document string, variables map[string]any, destination any) error {
	payload, err := json.Marshal(map[string]any{
		"query":     document,
		"variables": variables,
	})
	if err != nil {
		return fmt.Errorf("failure encoding IMDb GraphQL request: %w", err)
	}
	request, err := http.NewRequestWithContext(c.ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failure creating IMDb GraphQL request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "Mozilla/5.0")
	request.Header.Set("x-imdb-client-name", "imdb-web-next")
	request.Header.Set("x-imdb-user-country", "US")
	request.Header.Set("x-imdb-user-language", "en-US")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("IMDb GraphQL request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("IMDb GraphQL returned status %d", response.StatusCode)
	}
	if err := json.NewDecoder(response.Body).Decode(destination); err != nil {
		return fmt.Errorf("failure decoding IMDb GraphQL response: %w", err)
	}
	switch typed := destination.(type) {
	case *publicProfileResponse:
		if len(typed.Errors) > 0 {
			return fmt.Errorf("IMDb GraphQL profile error: %s", typed.Errors[0].Message)
		}
	case *publicRatingsResponse:
		if len(typed.Errors) > 0 {
			return fmt.Errorf("IMDb GraphQL ratings error: %s", typed.Errors[0].Message)
		}
	}
	return nil
}

func normalizePublicRatings(response *publicRatingsResponse) ([]publicRatingRow, error) {
	total := response.Data.UserRatings.Total
	rows := make([]publicRatingRow, 0, total)
	seen := make(map[string]struct{}, total)
	for index, edge := range response.Data.UserRatings.Edges {
		if edge.Node == nil {
			return nil, fmt.Errorf("public IMDb ratings row %d is empty", index+1)
		}
		node := edge.Node
		id := strings.TrimSpace(node.Title.ID)
		if id == "" {
			return nil, fmt.Errorf("public IMDb ratings row %d has no title id", index+1)
		}
		if _, duplicate := seen[id]; duplicate {
			return nil, fmt.Errorf("public IMDb ratings contains duplicate title %s", id)
		}
		seen[id] = struct{}{}
		ratedAt, err := parsePublicRatingDate(node.UserRating.Date)
		if err != nil {
			return nil, fmt.Errorf("public IMDb ratings row %d has invalid date: %w", index+1, err)
		}
		rows = append(rows, publicRatingRow{
			ID:      id,
			Title:   strings.TrimSpace(node.Title.TitleText.Text),
			Kind:    publicTitleKind(node.Title.TitleType.ID, node.Title.TitleType.IsEpisode, node.Title.TitleType.IsSeries),
			Rating:  node.UserRating.Value,
			RatedAt: ratedAt,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].RatedAt.Equal(rows[j].RatedAt) {
			return rows[i].ID < rows[j].ID
		}
		return rows[i].RatedAt.After(rows[j].RatedAt)
	})
	return rows, nil
}

func encodePublicRatings(rows []publicRatingRow) (Items, []byte, error) {
	var output bytes.Buffer
	writer := csv.NewWriter(&output)
	header := []string{
		"Const",
		"Your Rating",
		"Date Rated",
		"Title",
		"Original Title",
		"URL",
		"Title Type",
		"IMDb Rating",
		"Runtime (mins)",
		"Year",
		"Genres",
		"Num Votes",
		"Release Date",
		"Directors",
	}
	if err := writer.Write(header); err != nil {
		return nil, nil, fmt.Errorf("failure writing public IMDb ratings header: %w", err)
	}
	items := make(Items, 0, len(rows))
	for _, row := range rows {
		rating := row.Rating
		title := row.Title
		if title == "" {
			title = row.ID
		}
		record := []string{
			row.ID,
			strconv.FormatFloat(row.Rating, 'f', -1, 64),
			row.RatedAt.Format(time.DateOnly),
			title,
			title,
			"https://www.imdb.com/title/" + row.ID + "/",
			row.Kind,
			"",
			"",
			"",
			"",
			"",
			"",
			"",
		}
		if err := writer.Write(record); err != nil {
			return nil, nil, fmt.Errorf("failure writing public IMDb ratings row: %w", err)
		}
		items = append(items, Item{
			ID:      row.ID,
			Kind:    row.Kind,
			Created: row.RatedAt,
			Rating:  &rating,
		})
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, nil, fmt.Errorf("failure finalizing public IMDb ratings csv: %w", err)
	}
	return items, output.Bytes(), nil
}

func parsePublicRatingDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, time.DateOnly} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported date %q", value)
}

func publicTitleKind(typeID string, isEpisode, isSeries bool) string {
	normalized := strings.ToLower(typeID)
	switch {
	case isEpisode || strings.Contains(normalized, "episode"):
		return itemTypeTvEpisode
	case strings.Contains(normalized, "mini"):
		return itemTypeTvMiniSeries
	case isSeries || strings.Contains(normalized, "series"):
		return itemTypeTvSeries
	default:
		return itemTypeMovie
	}
}
