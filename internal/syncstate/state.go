package syncstate

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	baselineFile = "imdb-ratings.csv"
	manifestFile = "sync-state.json"
	mappingFile  = "tmdb-id-map.json"
	stateVersion = 1
)

type Rating struct {
	IMDbID    string  `json:"imdb_id"`
	Kind      string  `json:"kind"`
	Value     float64 `json:"value"`
	DateRated string  `json:"date_rated,omitempty"`
}

type Delta struct {
	Add    map[string]Rating
	Update map[string]Rating
	Remove map[string]Rating
}

func (d Delta) Empty() bool {
	return len(d.Add) == 0 && len(d.Update) == 0 && len(d.Remove) == 0
}

func (d Delta) UpsertIDs() map[string]struct{} {
	ids := make(map[string]struct{}, len(d.Add)+len(d.Update))
	for id := range d.Add {
		ids[id] = struct{}{}
	}
	for id := range d.Update {
		ids[id] = struct{}{}
	}
	return ids
}

func (d Delta) RemoveIDs() map[string]struct{} {
	ids := make(map[string]struct{}, len(d.Remove))
	for id := range d.Remove {
		ids[id] = struct{}{}
	}
	return ids
}

type manifest struct {
	Version          int       `json:"version"`
	BootstrappedAt   time.Time `json:"bootstrapped_at,omitempty"`
	LastReconciledAt time.Time `json:"last_reconciled_at,omitempty"`
	UpdatedAt        time.Time `json:"updated_at"`
	RatingCount      int       `json:"rating_count"`
}

type State struct {
	dir               string
	freshCSV          []byte
	current           map[string]Rating
	delta             Delta
	manifest          manifest
	now               time.Time
	bootstrap         bool
	fullReconciliation bool
}

func Load(dir string, freshCSV []byte, reconcileInterval time.Duration, now time.Time) (*State, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, nil
	}
	if len(freshCSV) == 0 {
		return nil, fmt.Errorf("cannot prepare sync state from an empty imdb ratings csv")
	}
	current, err := parseRatingsCSV(freshCSV)
	if err != nil {
		return nil, fmt.Errorf("failure parsing current imdb ratings snapshot: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("failure creating sync state directory: %w", err)
	}

	s := &State{
		dir:      dir,
		freshCSV: bytes.Clone(freshCSV),
		current:  current,
		delta: Delta{
			Add:    make(map[string]Rating),
			Update: make(map[string]Rating),
			Remove: make(map[string]Rating),
		},
		manifest: manifest{Version: stateVersion},
		now:      now.UTC(),
	}

	if data, err := os.ReadFile(filepath.Join(dir, manifestFile)); err == nil {
		if err := json.Unmarshal(data, &s.manifest); err != nil {
			return nil, fmt.Errorf("failure decoding sync state manifest: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("failure reading sync state manifest: %w", err)
	}

	baselineData, err := os.ReadFile(filepath.Join(dir, baselineFile))
	if os.IsNotExist(err) {
		s.bootstrap = true
		// Bootstrap deliberately produces an empty source delta. The current
		// IMDb snapshot becomes the baseline only after both destinations get
		// their chance and no hard destination error is returned.
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failure reading saved imdb ratings snapshot: %w", err)
	}

	baseline, err := parseRatingsCSV(baselineData)
	if err != nil {
		return nil, fmt.Errorf("failure parsing saved imdb ratings snapshot: %w", err)
	}
	for id, currentRating := range current {
		previous, found := baseline[id]
		if !found {
			s.delta.Add[id] = currentRating
			continue
		}
		if previous.Value != currentRating.Value || previous.Kind != currentRating.Kind {
			s.delta.Update[id] = currentRating
		}
	}
	for id, previous := range baseline {
		if _, found := current[id]; !found {
			s.delta.Remove[id] = previous
		}
	}

	if reconcileInterval > 0 && (s.manifest.LastReconciledAt.IsZero() || !s.now.Before(s.manifest.LastReconciledAt.Add(reconcileInterval))) {
		s.fullReconciliation = true
	}
	return s, nil
}

func (s *State) Bootstrap() bool {
	return s != nil && s.bootstrap
}

func (s *State) FullReconciliation() bool {
	return s != nil && s.fullReconciliation
}

func (s *State) Delta() Delta {
	if s == nil {
		return Delta{}
	}
	return s.delta
}

func (s *State) CurrentCount() int {
	if s == nil {
		return 0
	}
	return len(s.current)
}

func (s *State) MappingCachePath() string {
	if s == nil {
		return ""
	}
	return filepath.Join(s.dir, mappingFile)
}

func (s *State) Commit() error {
	if s == nil {
		return nil
	}
	if err := atomicWrite(filepath.Join(s.dir, baselineFile), s.freshCSV, 0o644); err != nil {
		return fmt.Errorf("failure writing imdb ratings baseline: %w", err)
	}
	if s.manifest.Version == 0 {
		s.manifest.Version = stateVersion
	}
	if s.bootstrap && s.manifest.BootstrappedAt.IsZero() {
		s.manifest.BootstrappedAt = s.now
		// A bootstrap follows a known-good pre-state sync and intentionally
		// avoids treating the entire historical library as a new delta. Start
		// the reconciliation clock here so the next run stays incremental.
		s.manifest.LastReconciledAt = s.now
	}
	if s.fullReconciliation {
		s.manifest.LastReconciledAt = s.now
	}
	s.manifest.UpdatedAt = s.now
	s.manifest.RatingCount = len(s.current)
	data, err := json.MarshalIndent(s.manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failure encoding sync state manifest: %w", err)
	}
	data = append(data, '\n')
	if err := atomicWrite(filepath.Join(s.dir, manifestFile), data, 0o644); err != nil {
		return fmt.Errorf("failure writing sync state manifest: %w", err)
	}
	return nil
}

func parseRatingsCSV(data []byte) (map[string]Rating, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failure reading csv records: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("expected a csv header row")
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
	dateIdx, hasDate := header["Date Rated"]

	maxIdx := constIdx
	if ratingIdx > maxIdx {
		maxIdx = ratingIdx
	}
	if kindIdx > maxIdx {
		maxIdx = kindIdx
	}
	if hasDate && dateIdx > maxIdx {
		maxIdx = dateIdx
	}

	out := make(map[string]Rating, len(records)-1)
	for rowNum, record := range records[1:] {
		if len(record) <= maxIdx {
			return nil, fmt.Errorf("csv row %d has too few columns", rowNum+2)
		}
		id := strings.TrimSpace(record[constIdx])
		if id == "" {
			return nil, fmt.Errorf("csv row %d has empty Const value", rowNum+2)
		}
		value, err := strconv.ParseFloat(strings.TrimSpace(record[ratingIdx]), 64)
		if err != nil {
			return nil, fmt.Errorf("csv row %d has invalid rating: %w", rowNum+2, err)
		}
		rating := Rating{
			IMDbID: id,
			Kind:   strings.TrimSpace(record[kindIdx]),
			Value:  value,
		}
		if hasDate {
			rating.DateRated = strings.TrimSpace(record[dateIdx])
		}
		out[id] = rating
	}
	return out, nil
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
