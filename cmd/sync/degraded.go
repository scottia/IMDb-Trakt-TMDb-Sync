package sync

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/scottia/IMDb-Trakt-TMDb-Sync/internal/logger"
)

const cachedIMDbRatingsFile = "imdb-ratings.csv"

var (
	errIMDbSourceUnavailable = errors.New("imdb source unavailable")
	imdbWAFBlockedMessages  = []string{
		"aws waf challenge remained after solver token and reload",
		"browser-native aws waf challenge did not clear",
	}
)

func handleCachedWAFSourceUnavailable(sourceErr error) (bool, error) {
	if !isIMDbWAFBlocked(sourceErr) {
		return false, nil
	}

	count, err := cachedIMDbRatingsCount(strings.TrimSpace(os.Getenv("ITS_STATE_DIR")))
	if err != nil {
		return true, fmt.Errorf("%w due to aws waf; cached baseline validation failed: %v", errIMDbSourceUnavailable, err)
	}

	log := logger.NewLogger(os.Stdout)
	log.Warn(
		"sync deferred; imdb source blocked by aws waf; no fresh source snapshot or destination sync was performed",
		"status", "degraded_source_unavailable",
		"source_snapshot", "unavailable",
		"destinations_attempted", false,
		"titles_synchronized", 0,
		"cached_ratings", count,
		logger.Error(sourceErr),
	)

	return true, fmt.Errorf(
		"%w due to aws waf; no sync performed; cached baseline preserved (%d ratings): %v",
		errIMDbSourceUnavailable,
		count,
		sourceErr,
	)
}

func isIMDbWAFBlocked(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, marker := range imdbWAFBlockedMessages {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func cachedIMDbRatingsCount(stateDir string) (int, error) {
	if stateDir == "" {
		return 0, fmt.Errorf("ITS_STATE_DIR is empty")
	}

	path := filepath.Join(stateDir, cachedIMDbRatingsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("failure reading cached imdb ratings baseline %s: %w", path, err)
	}

	reader := csv.NewReader(strings.NewReader(string(data)))
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return 0, fmt.Errorf("failure reading cached imdb ratings csv: %w", err)
	}
	if len(records) == 0 {
		return 0, fmt.Errorf("cached imdb ratings csv has no header row")
	}

	header := make(map[string]int, len(records[0]))
	for index, name := range records[0] {
		header[strings.TrimSpace(name)] = index
	}
	constIndex, ok := header["Const"]
	if !ok {
		return 0, fmt.Errorf("cached imdb ratings csv is missing Const column")
	}
	if _, ok := header["Your Rating"]; !ok {
		return 0, fmt.Errorf("cached imdb ratings csv is missing Your Rating column")
	}
	if _, ok := header["Title Type"]; !ok {
		return 0, fmt.Errorf("cached imdb ratings csv is missing Title Type column")
	}

	count := 0
	for rowNumber, record := range records[1:] {
		if len(record) <= constIndex {
			return 0, fmt.Errorf("cached imdb ratings csv row %d has too few columns", rowNumber+2)
		}
		if strings.TrimSpace(record[constIndex]) != "" {
			count++
		}
	}
	return count, nil
}
