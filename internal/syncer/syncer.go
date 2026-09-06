package syncer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	appconfig "github.com/cecobask/imdb-trakt-sync/internal/config"
	"github.com/cecobask/imdb-trakt-sync/internal/imdb"
	"github.com/cecobask/imdb-trakt-sync/internal/logger"
	"github.com/cecobask/imdb-trakt-sync/internal/syncstate"
	"github.com/cecobask/imdb-trakt-sync/internal/tmdb"
	"github.com/cecobask/imdb-trakt-sync/internal/trakt"
)

type Syncer struct {
	logger      *slog.Logger
	imdbClient  imdb.API
	traktClient trakt.API
	traktConf   appconfig.Trakt
	user        *user
	conf        appconfig.Sync
	authless    bool
	tmdbConf    appconfig.TMDb
	tmdbBrowser tmdb.BrowserOptions
	syncState   *syncstate.State
}

type user struct {
	imdbLists    map[string]imdb.List
	imdbRatings  map[string]imdb.Item
	traktLists   map[string]trakt.List
	traktRatings map[string]trakt.Item
}

func NewSyncer(ctx context.Context, conf *appconfig.Config) (*Syncer, error) {
	log := logger.NewLogger(os.Stdout)
	imdbClient, err := imdb.NewAPI(ctx, &conf.IMDb, log)
	if err != nil {
		return nil, fmt.Errorf("failure initialising imdb client: %w", err)
	}
	syncer := &Syncer{
		logger:     log,
		imdbClient: imdbClient,
		traktConf:  conf.Trakt,
		user:       &user{},
		conf:       conf.Sync,
		authless:   *conf.IMDb.Auth == appconfig.IMDbAuthMethodNone,
		tmdbConf:   conf.TMDb,
		tmdbBrowser: tmdb.BrowserOptions{
			BrowserPath: *conf.IMDb.BrowserPath,
			Headless:    *conf.IMDb.Headless,
			Trace:       *conf.IMDb.Trace,
		},
	}
	if *conf.Sync.Ratings {
		syncer.user.imdbRatings = make(map[string]imdb.Item)
		syncer.user.traktRatings = make(map[string]trakt.Item)
	}
	if *conf.Sync.Lists || *conf.Sync.Watchlist {
		syncer.user.imdbLists = make(map[string]imdb.List, len(*conf.IMDb.Lists))
		syncer.user.traktLists = make(map[string]trakt.List, len(*conf.IMDb.Lists))
		for _, lid := range *conf.IMDb.Lists {
			syncer.user.imdbLists[lid] = imdb.List{ListID: lid}
		}
	}
	return syncer, nil
}

func (s *Syncer) Sync(ctx context.Context) error {
	s.logger.Info("sync started")
	if err := s.hydrateIMDb(); err != nil {
		s.logger.Error("failure hydrating imdb source", logger.Error(err))
		return err
	}
	if err := s.prepareSyncState(); err != nil {
		s.logger.Error("failure preparing persistent sync state", logger.Error(err))
		return err
	}

	var destinationErrors []error
	if err := s.syncTraktDestination(ctx); err != nil {
		s.logger.Error("trakt destination failed", logger.Error(err))
		destinationErrors = append(destinationErrors, fmt.Errorf("trakt destination: %w", err))
	} else {
		s.logger.Info("trakt destination completed")
	}

	if err := s.syncTMDbRatings(ctx); err != nil {
		s.logger.Error("tmdb destination failed", logger.Error(err))
		destinationErrors = append(destinationErrors, fmt.Errorf("tmdb destination: %w", err))
	} else {
		s.logger.Info("tmdb destination completed")
	}

	if len(destinationErrors) > 0 {
		return fmt.Errorf("one or more destination syncs failed: %w", errors.Join(destinationErrors...))
	}

	if s.syncState != nil && *s.conf.Mode != appconfig.SyncModeDryRun {
		if err := s.syncState.Commit(); err != nil {
			return fmt.Errorf("failure advancing persistent sync state: %w", err)
		}
		s.logger.Info("persistent sync state advanced after successful destinations")
	}

	s.logger.Info("sync completed")
	return nil
}

func (s *Syncer) syncTraktDestination(ctx context.Context) error {
	if err := s.hydrateTrakt(ctx); err != nil {
		return fmt.Errorf("failure hydrating trakt client: %w", err)
	}
	if err := s.syncLists(ctx); err != nil {
		return fmt.Errorf("failure syncing lists: %w", err)
	}
	if err := s.syncRatings(ctx); err != nil {
		return fmt.Errorf("failure syncing ratings: %w", err)
	}
	if err := s.syncHistory(ctx); err != nil {
		return fmt.Errorf("failure syncing history: %w", err)
	}
	return nil
}

func (s *Syncer) setupTraktLists(ctx context.Context, imdbLists imdb.Lists) (trakt.IDMetas, error) {
	traktLists, err := s.traktClient.ListsGetAllMeta(ctx)
	if err != nil {
		return nil, fmt.Errorf("failure fetching trakt lists metadata: %w", err)
	}
	traktListsMetaMap := make(map[string]trakt.IDMeta, len(traktLists))
	for _, traktList := range traktLists {
		traktListsMetaMap[*traktList.Name] = traktList.IDMeta
	}

	traktListsMeta := make(trakt.IDMetas, 0, len(imdbLists))
	for _, imdbList := range imdbLists {
		s.user.imdbLists[imdbList.ListID] = imdbList
		if *s.conf.Mode == appconfig.SyncModeDryRun {
			s.logger.Info("sync would have created trakt list", "name", imdbList.ListName)
			continue
		}
		traktListMeta, ok := traktListsMetaMap[imdbList.ListName]
		if !ok {
			newTraktListMeta, err := s.traktClient.ListCreate(ctx, imdbList.ListName)
			if err != nil {
				return nil, fmt.Errorf("failure creating trakt list %s: %w", imdbList.ListName, err)
			}
			traktListMeta = *newTraktListMeta
		}
		traktListMeta.IMDb = imdbList.ListID
		traktListsMeta = append(traktListsMeta, traktListMeta)
	}

	return traktListsMeta, nil
}

func (s *Syncer) hydrateIMDb() error {
	lids := make([]string, 0, len(s.user.imdbLists))
	for lid := range s.user.imdbLists {
		lids = append(lids, lid)
	}
	if *s.conf.Ratings {
		if err := s.imdbClient.RatingsExport(); err != nil {
			return fmt.Errorf("failure exporting imdb ratings: %w", err)
		}
	}
	if *s.conf.Lists {
		if err := s.imdbClient.ListsExport(lids...); err != nil {
			return fmt.Errorf("failure exporting imdb lists: %w", err)
		}
	}
	if *s.conf.Watchlist {
		if err := s.imdbClient.WatchlistExport(); err != nil {
			return fmt.Errorf("failure exporting imdb watchlist: %w", err)
		}
	}
	if *s.conf.Lists {
		imdbLists, err := s.imdbClient.ListsGet(lids...)
		if err != nil {
			return fmt.Errorf("failure fetching imdb lists: %w", err)
		}
		for _, imdbList := range imdbLists {
			s.user.imdbLists[imdbList.ListID] = imdbList
		}
	}
	if s.authless {
		return nil
	}
	if *s.conf.Watchlist {
		imdbWatchlist, err := s.imdbClient.WatchlistGet()
		if err != nil {
			return fmt.Errorf("failure fetching imdb watchlist: %w", err)
		}
		s.user.imdbLists[imdbWatchlist.ListID] = *imdbWatchlist
	}
	if *s.conf.Ratings {
		imdbRatings, err := s.imdbClient.RatingsGet()
		if err != nil {
			return fmt.Errorf("failure fetching imdb ratings: %w", err)
		}
		for _, imdbRating := range imdbRatings {
			s.user.imdbRatings[imdbRating.ID] = imdbRating
		}
	}
	return nil
}

func (s *Syncer) hydrateTrakt(ctx context.Context) error {
	traktClient, err := trakt.NewAPI(ctx, s.traktConf, s.logger)
	if err != nil {
		return fmt.Errorf("failure initialising trakt client: %w", err)
	}
	s.traktClient = traktClient

	if *s.conf.Lists {
		imdbLists := make(imdb.Lists, 0, len(s.user.imdbLists))
		for _, imdbList := range s.user.imdbLists {
			if !imdbList.IsWatchlist {
				imdbLists = append(imdbLists, imdbList)
			}
		}
		traktIDMetas, err := s.setupTraktLists(ctx, imdbLists)
		if err != nil {
			return fmt.Errorf("failure setting up trakt lists: %w", err)
		}
		traktLists, err := s.traktClient.ListsGet(ctx, traktIDMetas)
		if err != nil {
			return fmt.Errorf("failure hydrating trakt lists: %w", err)
		}
		for _, traktList := range traktLists {
			s.user.traktLists[traktList.IDMeta.IMDb] = traktList
		}
	}
	if s.authless {
		return nil
	}
	if *s.conf.Watchlist {
		var imdbWatchlist *imdb.List
		for _, imdbList := range s.user.imdbLists {
			if imdbList.IsWatchlist {
				watchlist := imdbList
				imdbWatchlist = &watchlist
				break
			}
		}
		if imdbWatchlist == nil {
			return fmt.Errorf("imdb watchlist was not hydrated")
		}
		traktWatchlist, err := s.traktClient.WatchlistGet(ctx)
		if err != nil {
			return fmt.Errorf("failure fetching trakt watchlist: %w", err)
		}
		s.user.traktLists[imdbWatchlist.ListID] = *traktWatchlist
	}
	if *s.conf.Ratings {
		traktRatings, err := s.traktClient.RatingsGet(ctx)
		if err != nil {
			return fmt.Errorf("failure fetching trakt ratings: %w", err)
		}
		for _, traktRating := range traktRatings {
			id, err := traktRating.GetItemID()
			if err != nil {
				return fmt.Errorf("failure fetching trakt item id: %w", err)
			}
			if id != nil {
				s.user.traktRatings[*id] = traktRating
			}
		}
	}
	return nil
}

func (s *Syncer) syncLists(ctx context.Context) error {
	if !*s.conf.Watchlist {
		s.logger.Info("skipping watchlist sync")
	}
	if !*s.conf.Lists {
		s.logger.Info("skipping lists sync")
	}
	if !*s.conf.Watchlist && !*s.conf.Lists {
		return nil
	}
	for _, imdbList := range s.user.imdbLists {
		diff := listDiff(imdbList, s.user.traktLists[imdbList.ListID])
		if imdbList.IsWatchlist {
			if len(diff.Add) > 0 {
				if *s.conf.Mode == appconfig.SyncModeDryRun {
					s.logger.Info("sync would have added trakt watchlist items", "count", len(diff.Add))
					continue
				}
				if err := s.traktClient.WatchlistItemsAdd(ctx, diff.Add); err != nil {
					return fmt.Errorf("failure adding items to trakt watchlist: %w", err)
				}
			} else {
				s.logger.Info("no trakt watchlist items to add")
			}
			if len(diff.Remove) > 0 {
				if *s.conf.Mode == appconfig.SyncModeDryRun || *s.conf.Mode == appconfig.SyncModeAddOnly {
					s.logger.Info("sync would have removed trakt watchlist items", "count", len(diff.Remove))
					continue
				}
				if err := s.traktClient.WatchlistItemsRemove(ctx, diff.Remove); err != nil {
					return fmt.Errorf("failure removing items from trakt watchlist: %w", err)
				}
			} else {
				s.logger.Info("no trakt watchlist items to remove")
			}
			continue
		}
		traktListSlug := s.user.traktLists[imdbList.ListID].IDMeta.Slug
		if len(diff.Add) > 0 {
			if *s.conf.Mode == appconfig.SyncModeDryRun {
				s.logger.Info("sync would have added trakt list items", "count", len(diff.Add), "name", imdbList.ListName)
				continue
			}
			if err := s.traktClient.ListItemsAdd(ctx, traktListSlug, imdbList.ListName, diff.Add); err != nil {
				return fmt.Errorf("failure adding items to trakt list %s: %w", imdbList.ListName, err)
			}
		} else {
			s.logger.Info("no trakt list items to add", "name", imdbList.ListName)
		}
		if len(diff.Remove) > 0 {
			if *s.conf.Mode == appconfig.SyncModeDryRun || *s.conf.Mode == appconfig.SyncModeAddOnly {
				s.logger.Info("sync would have deleted trakt list items", "count", len(diff.Remove), "name", imdbList.ListName)
				continue
			}
			if err := s.traktClient.ListItemsRemove(ctx, traktListSlug, imdbList.ListName, diff.Remove); err != nil {
				return fmt.Errorf("failure removing trakt list items from %s: %w", imdbList.ListName, err)
			}
		} else {
			s.logger.Info("no trakt list items to remove", "name", imdbList.ListName)
		}
	}
	return nil
}

func (s *Syncer) syncRatings(ctx context.Context) error {
	if s.authless {
		s.logger.Info("skipping ratings sync since no imdb auth was provided")
		return nil
	}
	if !*s.conf.Ratings {
		s.logger.Info("skipping ratings sync")
		return nil
	}
	diff := s.ratingDiff()
	if len(diff.Add) > 0 {
		if *s.conf.Mode == appconfig.SyncModeDryRun {
			s.logger.Info("sync would have added trakt ratings", "count", len(diff.Add))
		} else {
			if err := s.traktClient.RatingsAdd(ctx, diff.Add); err != nil {
				return fmt.Errorf("failure adding trakt ratings: %w", err)
			}
		}
	} else {
		s.logger.Info("no trakt ratings to add")
	}
	if len(diff.Remove) > 0 {
		if *s.conf.Mode == appconfig.SyncModeDryRun || *s.conf.Mode == appconfig.SyncModeAddOnly {
			s.logger.Info("sync would have deleted trakt ratings", "count", len(diff.Remove))
		} else {
			if err := s.traktClient.RatingsRemove(ctx, diff.Remove); err != nil {
				return fmt.Errorf("failure removing trakt ratings: %w", err)
			}
		}
	} else {
		s.logger.Info("no trakt ratings to remove")
	}
	return nil
}

func (s *Syncer) syncTMDbRatings(ctx context.Context) error {
	if s.tmdbConf.Enabled == nil || !*s.tmdbConf.Enabled {
		s.logger.Info("skipping tmdb ratings sync")
		return nil
	}
	if s.authless {
		s.logger.Info("skipping tmdb ratings sync since no imdb auth was provided")
		return nil
	}
	if !*s.conf.Ratings {
		s.logger.Info("skipping tmdb ratings sync since ratings sync is disabled")
		return nil
	}

	data := s.imdbClient.RatingsCSV()
	if len(data) == 0 {
		s.logger.Info("skipping tmdb ratings sync since no imdb ratings csv was downloaded")
		return nil
	}
	if *s.conf.Mode == appconfig.SyncModeDryRun {
		if s.syncState != nil {
			delta := s.syncState.Delta()
			s.logger.Info(
				"sync would process imdb ratings source delta for tmdb",
				"bootstrap", s.syncState.Bootstrap(),
				"full_reconciliation", s.syncState.FullReconciliation(),
				"add", len(delta.Add),
				"update", len(delta.Update),
				"remove", len(delta.Remove),
			)
		} else {
			s.logger.Info("sync would compare imdb ratings against tmdb api", "count", len(s.user.imdbRatings), "bytes", len(data))
		}
		return nil
	}

	if err := tmdb.ImportRatings(ctx, &s.tmdbConf, s.tmdbBrowser, s.logger, data, s.syncState, *s.conf.Mode); err != nil {
		return fmt.Errorf("failure syncing imdb ratings to tmdb: %w", err)
	}
	return nil
}

func (s *Syncer) syncHistory(ctx context.Context) error {
	if s.authless {
		s.logger.Info("skipping history sync since no imdb auth was provided")
		return nil
	}
	if !*s.conf.History {
		s.logger.Info("skipping history sync")
		return nil
	}
	diff := s.ratingDiff()
	if len(diff.Add) > 0 {
		var historyToAdd trakt.Items
		for i := range diff.Add {
			traktItemID, err := diff.Add[i].GetItemID()
			if err != nil {
				return fmt.Errorf("failure fetching trakt item id: %w", err)
			}
			history, err := s.traktClient.HistoryGet(ctx, diff.Add[i].Type, *traktItemID)
			if err != nil {
				return fmt.Errorf("failure fetching trakt history for %s %s: %w", diff.Add[i].Type, *traktItemID, err)
			}
			if len(history) > 0 {
				continue
			}
			historyToAdd = append(historyToAdd, diff.Add[i])
		}
		if len(historyToAdd) > 0 {
			if *s.conf.Mode == appconfig.SyncModeDryRun {
				s.logger.Info("sync would have added trakt history", "count", len(historyToAdd))
			} else {
				if err := s.traktClient.HistoryAdd(ctx, historyToAdd); err != nil {
					return fmt.Errorf("failure adding trakt history: %w", err)
				}
			}
		}
	} else {
		s.logger.Info("no history to add to trakt")
	}
	if len(diff.Remove) > 0 {
		var historyToRemove trakt.Items
		for i := range diff.Remove {
			traktItemID, err := diff.Remove[i].GetItemID()
			if err != nil {
				return fmt.Errorf("failure fetching trakt item id: %w", err)
			}
			history, err := s.traktClient.HistoryGet(ctx, diff.Remove[i].Type, *traktItemID)
			if err != nil {
				return fmt.Errorf("failure fetching trakt history for %s %s: %w", diff.Remove[i].Type, *traktItemID, err)
			}
			if len(history) == 0 {
				continue
			}
			historyToRemove = append(historyToRemove, diff.Remove[i])
		}
		if len(historyToRemove) > 0 {
			if *s.conf.Mode == appconfig.SyncModeDryRun || *s.conf.Mode == appconfig.SyncModeAddOnly {
				s.logger.Info("sync would have deleted trakt history", "count", len(historyToRemove))
			} else {
				if err := s.traktClient.HistoryRemove(ctx, historyToRemove); err != nil {
					return fmt.Errorf("failure removing trakt history: %w", err)
				}
			}
		}
	} else {
		s.logger.Info("no trakt history to remove")
	}
	return nil
}
