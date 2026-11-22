from pathlib import Path


def replace_once(path: str, old: str, new: str) -> None:
    file_path = Path(path)
    text = file_path.read_text()
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{path}: expected one anchor, found {count}")
    file_path.write_text(text.replace(old, new, 1))


replace_once(
    "internal/config/config.go",
    '''type IMDb struct {
\tAuth         *IMDbAuthMethod `koanf:"AUTH"`
\tEmail        *string         `koanf:"EMAIL"`
\tPassword     *string         `koanf:"PASSWORD"`
\tCookieAtMain *string         `koanf:"COOKIEATMAIN"`
\tLists        *[]string       `koanf:"LISTS"`''',
    '''type IMDb struct {
\tAuth            *IMDbAuthMethod `koanf:"AUTH"`
\tEmail           *string         `koanf:"EMAIL"`
\tPassword        *string         `koanf:"PASSWORD"`
\tCookieAtMain    *string         `koanf:"COOKIEATMAIN"`
\tRatingsUserList *string         `koanf:"RATINGSUSERLIST"`
\tLists           *[]string       `koanf:"LISTS"`''',
)
replace_once(
    "internal/config/config.go",
    '''\tif c.IMDb.BrowserPath == nil {
\t\tc.IMDb.BrowserPath = pointer("")
\t}

\tif c.Trakt.Enabled == nil {''',
    '''\tif c.IMDb.BrowserPath == nil {
\t\tc.IMDb.BrowserPath = pointer("")
\t}
\tif c.IMDb.RatingsUserList == nil {
\t\tc.IMDb.RatingsUserList = pointer("")
\t}

\tif c.Trakt.Enabled == nil {''',
)

replace_once(
    "internal/syncer/syncer.go",
    '''\t"log/slog"
\t"os"
\t"time"''',
    '''\t"log/slog"
\t"os"
\t"strings"
\t"time"''',
)

old = '''\ttraktRatings := *conf.Trakt.Enabled && (*conf.Trakt.SyncRatings || *conf.Trakt.SyncHistory)
\ttmdbRatings := *conf.TMDb.Enabled && *conf.TMDb.SyncRatings
\tsourceRatings := traktRatings || tmdbRatings
\tsourceLists := (*conf.Trakt.Enabled && *conf.Trakt.SyncLists) || (*conf.TMDb.Enabled && *conf.TMDb.SyncLists)
\tsourceWatchlist := (*conf.Trakt.Enabled && *conf.Trakt.SyncWatchlist) || (*conf.TMDb.Enabled && *conf.TMDb.SyncWatchlist)

\tsourceTimeout := sourcePhaseTimeout(conf)
\tsourceCtx, sourceCancel := context.WithTimeout(ctx, sourceTimeout)
\timdbClient, err := imdb.NewAPI(sourceCtx, &conf.IMDb, log)
\tif err != nil {
\t\tsourceCancel()
\t\treturn nil, fmt.Errorf("failure initialising imdb client: %w", err)
\t}

\ttraktSync := appconfig.Sync{
\t\tMode:      conf.Trakt.SyncMode,
\t\tHistory:   conf.Trakt.SyncHistory,
\t\tRatings:   conf.Trakt.SyncRatings,
\t\tWatchlist: conf.Trakt.SyncWatchlist,
\t\tLists:     conf.Trakt.SyncLists,
\t\tTimeout:   conf.Trakt.SyncTimeout,
\t}
'''
new = '''\ttraktRatings := *conf.Trakt.Enabled && (*conf.Trakt.SyncRatings || *conf.Trakt.SyncHistory)
\ttmdbRatings := *conf.TMDb.Enabled && *conf.TMDb.SyncRatings
\tsourceRatings := traktRatings || tmdbRatings
\tsourceLists := (*conf.Trakt.Enabled && *conf.Trakt.SyncLists) || (*conf.TMDb.Enabled && *conf.TMDb.SyncLists)
\tsourceWatchlist := (*conf.Trakt.Enabled && *conf.Trakt.SyncWatchlist) || (*conf.TMDb.Enabled && *conf.TMDb.SyncWatchlist)

\tpublicRatingsRef := ""
\tif conf.IMDb.RatingsUserList != nil {
\t\tpublicRatingsRef = strings.TrimSpace(*conf.IMDb.RatingsUserList)
\t}
\tpublicRatingsConfigured := publicRatingsRef != ""

\tsourceTimeout := sourcePhaseTimeout(conf)
\tsourceCtx, sourceCancel := context.WithTimeout(ctx, sourceTimeout)
\tvar (
\t\timdbClient        imdb.API
\t\terr               error
\t\tpublicRatingsOnly bool
\t)
\tif publicRatingsConfigured && sourceRatings && !sourceLists && !sourceWatchlist {
\t\timdbClient, err = imdb.NewPublicRatingsAPI(sourceCtx, publicRatingsRef, log)
\t\tif err == nil {
\t\t\tpublicRatingsOnly = true
\t\t\tlog.Info("using anonymous public imdb ratings source")
\t\t}
\t} else {
\t\timdbClient, err = imdb.NewAPI(sourceCtx, &conf.IMDb, log)
\t\tif err != nil && publicRatingsConfigured && sourceRatings && imdb.IsWAFBlocked(err) {
\t\t\tbrowserErr := err
\t\t\timdbClient, err = imdb.NewPublicRatingsAPI(sourceCtx, publicRatingsRef, log)
\t\t\tif err == nil {
\t\t\t\tpublicRatingsOnly = true
\t\t\t\tlog.Warn(
\t\t\t\t\t"imdb browser source blocked by aws waf; using anonymous public ratings source",
\t\t\t\t\tlogger.Error(browserErr),
\t\t\t\t)
\t\t\t}
\t\t}
\t}
\tif err != nil {
\t\tsourceCancel()
\t\treturn nil, fmt.Errorf("failure initialising imdb client: %w", err)
\t}

\ttraktSync := appconfig.Sync{
\t\tMode:      conf.Trakt.SyncMode,
\t\tHistory:   conf.Trakt.SyncHistory,
\t\tRatings:   conf.Trakt.SyncRatings,
\t\tWatchlist: conf.Trakt.SyncWatchlist,
\t\tLists:     conf.Trakt.SyncLists,
\t\tTimeout:   conf.Trakt.SyncTimeout,
\t}
\ttmdbConf := conf.TMDb
\tif publicRatingsOnly {
\t\tdisabled := false
\t\tif sourceLists || sourceWatchlist {
\t\t\tlog.Warn("public imdb ratings source cannot supply watchlist or custom lists; those source features are deferred for this run")
\t\t}
\t\tsourceLists = false
\t\tsourceWatchlist = false
\t\ttraktSync.Watchlist = &disabled
\t\ttraktSync.Lists = &disabled
\t\ttmdbConf.SyncWatchlist = &disabled
\t\ttmdbConf.SyncLists = &disabled
\t}
'''
replace_once("internal/syncer/syncer.go", old, new)
replace_once(
    "internal/syncer/syncer.go",
    '''\t\tauthless:          *conf.IMDb.Auth == appconfig.IMDbAuthMethodNone,
\t\ttmdbConf:          conf.TMDb,''',
    '''\t\tauthless:          *conf.IMDb.Auth == appconfig.IMDbAuthMethodNone && !publicRatingsOnly,
\t\ttmdbConf:          tmdbConf,''',
)
replace_once(
    "internal/syncer/syncer.go",
    '''\tif *conf.Trakt.Enabled && (*conf.Trakt.SyncLists || *conf.Trakt.SyncWatchlist) {''',
    '''\tif *conf.Trakt.Enabled && (*traktSync.Lists || *traktSync.Watchlist) {''',
)

for path in ["README.md", "examples/private-runner/sync.yaml"]:
    replace_once(
        path,
        '''  ITS_IMDB_COOKIEATMAIN: ${{ secrets.IMDB_COOKIEATMAIN }}
  ITS_IMDB_COOKIEJARFILE: ${{ github.workspace }}/imdb-cookie-jar.json''',
        '''  ITS_IMDB_COOKIEATMAIN: ${{ secrets.IMDB_COOKIEATMAIN }}
  ITS_IMDB_RATINGSUSERLIST: ${{ secrets.IMDB_RATINGS_USER_LIST }}
  ITS_IMDB_COOKIEJARFILE: ${{ github.workspace }}/imdb-cookie-jar.json''',
    )

replace_once(
    "README.md",
    '''| `IMDB_COOKIE_JAR` | Seeded into `ITS_IMDB_COOKIEJARFILE` | Unattended IMDb cookie recycling after the first successful bootstrap | **Do not create manually.** If absent, the Runner bootstraps from `IMDB_COOKIEATMAIN`; after successful IMDb authentication and hydration, it persists the allowlisted IMDb cookie jar to this secret and reuses it on later runs. |''',
    '''| `IMDB_COOKIE_JAR` | Seeded into `ITS_IMDB_COOKIEJARFILE` | Unattended IMDb cookie recycling after the first successful bootstrap | **Do not create manually.** If absent, the Runner bootstraps from `IMDB_COOKIEATMAIN`; after successful IMDb authentication and hydration, it persists the allowlisted IMDb cookie jar to this secret and reuses it on later runs. |
| `IMDB_RATINGS_USER_LIST` | `ITS_IMDB_RATINGSUSERLIST` | Optional anonymous ratings source / WAF fallback | Set this to a public IMDb ratings URL or its `ur...` / `p....` user identifier. When browser acquisition is blocked by IMDb WAF, ratings and rating-derived history continue from IMDb's public ratings data without sending Amazon credentials or cookies. Watchlist and custom-list sync are deferred for that run. |''',
)
replace_once(
    "README.md",
    '''The private Runner seeds `ITS_IMDB_COOKIEJARFILE` from the `IMDB_COOKIE_JAR` repository secret when present, lets the application refresh the allowlisted jar after successful authentication/hydration, and persists a changed jar back to the same secret.''',
    '''The private Runner seeds `ITS_IMDB_COOKIEJARFILE` from the `IMDB_COOKIE_JAR` repository secret when present, lets the application refresh the allowlisted jar after successful authentication/hydration, and persists a changed jar back to the same secret.

When `IMDB_RATINGS_USER_LIST` is present, the application accepts either the full public IMDb ratings URL or the profile's `ur...` / `p....` identifier. If the browser path is stopped by AWS WAF, the application retrieves a fresh anonymous ratings snapshot, feeds it into the existing persistent CSV delta pipeline, and continues Trakt ratings/history and TMDb ratings. It never treats the previous committed CSV as fresh source data. Watchlist and custom-list destinations are skipped rather than cleared when only the public ratings source is available.''',
)
