package syncer

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cecobask/imdb-trakt-sync/internal/syncstate"
)

const defaultStateReconcileInterval = 7 * 24 * time.Hour

func (s *Syncer) prepareSyncState() error {
	if !*s.conf.Ratings {
		return nil
	}
	dir := strings.TrimSpace(os.Getenv("ITS_STATE_DIR"))
	if dir == "" {
		s.logger.Info("sync state disabled; ratings destinations will use full reconciliation")
		return nil
	}
	interval := defaultStateReconcileInterval
	if raw := strings.TrimSpace(os.Getenv("ITS_STATE_RECONCILEINTERVAL")); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			return fmt.Errorf("invalid ITS_STATE_RECONCILEINTERVAL %q: %w", raw, err)
		}
		interval = parsed
	}
	state, err := syncstate.Load(dir, s.imdbClient.RatingsCSV(), interval, time.Now())
	if err != nil {
		return fmt.Errorf("failure preparing sync state: %w", err)
	}
	s.syncState = state
	if state == nil {
		return nil
	}
	if state.Bootstrap() {
		s.logger.Info(
			"sync state bootstrap prepared; historical ratings will not be replayed",
			"ratings", state.CurrentCount(),
		)
		return nil
	}
	delta := state.Delta()
	if state.FullReconciliation() {
		s.logger.Info(
			"periodic full ratings reconciliation due",
			"ratings", state.CurrentCount(),
			"source_add", len(delta.Add),
			"source_update", len(delta.Update),
			"source_remove", len(delta.Remove),
		)
		return nil
	}
	s.logger.Info(
		"imdb ratings source delta prepared",
		"ratings", state.CurrentCount(),
		"add", len(delta.Add),
		"update", len(delta.Update),
		"remove", len(delta.Remove),
	)
	return nil
}

func (s *Syncer) ratingDiff() diff {
	full := itemsDifference(s.user.imdbRatings, s.user.traktRatings)
	if s.syncState == nil || s.syncState.FullReconciliation() {
		return full
	}
	if s.syncState.Bootstrap() {
		return newDiff()
	}
	delta := s.syncState.Delta()
	return filterRatingDiff(full, delta.UpsertIDs(), delta.RemoveIDs())
}

func filterRatingDiff(input diff, upsertIDs, removeIDs map[string]struct{}) diff {
	out := newDiff()
	for _, item := range input.Add {
		id, err := item.GetItemID()
		if err != nil || id == nil {
			continue
		}
		if _, ok := upsertIDs[*id]; ok {
			out.Add = append(out.Add, item)
		}
	}
	for _, item := range input.Remove {
		id, err := item.GetItemID()
		if err != nil || id == nil {
			continue
		}
		if _, ok := removeIDs[*id]; ok {
			out.Remove = append(out.Remove, item)
		}
	}
	out.Sort()
	return out
}
