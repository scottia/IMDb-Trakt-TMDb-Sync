[![sync](https://github.com/scottia/IMDb-Trakt-TMDb-Sync/actions/workflows/runner-sync-status.yaml/badge.svg)](https://github.com/scottia/IMDb-Trakt-TMDb-Sync/actions/workflows/runner-sync-status.yaml)
[![quality](https://github.com/scottia/IMDb-Trakt-TMDb-Sync/actions/workflows/quality.yaml/badge.svg?branch=main)](https://github.com/scottia/IMDb-Trakt-TMDb-Sync/actions/workflows/quality.yaml?query=branch%3Amain)

> **Note for forks:** The badges above are hardcoded to this repository. After forking, update the badge URLs at the top of this file, replacing `scottia/IMDb-Trakt-TMDb-Sync` with your own `{username}/{repo-name}`. If you use the private Runner workflow, also update the `repository:` value in the `Check out application source` step and `STATUS_REPOSITORY` in the `publish-status` job so Runner status is published to your fork.

# IMDb-Trakt-TMDb-Sync

<img src="./assets/logo.png" alt="IMDb to Trakt and TMDb"/>

One-way synchronization from [IMDb](https://www.imdb.com/) to independently configurable destinations:

- **[Trakt](https://trakt.tv/dashboard):** ratings, rating-derived history, watchlist, and custom lists.
- **[TMDb](https://www.themoviedb.org):** ratings and watchlist through the TMDb API.

IMDb is the source of truth. Changes made directly on Trakt or TMDb are not written back to IMDb. Trakt and TMDb have separate enable switches, feature switches, sync modes, and timeouts, so users may run Trakt only, TMDb only, or both.

The **sync** badge reflects the latest private Runner result through a status-only `repository_dispatch` relay. The **quality** badge reflects lint/build CI for the public source repository.

> [!IMPORTANT]
> Trakt API app creation currently requires Trakt VIP access. See upstream issue [#107](https://github.com/cecobask/imdb-trakt-sync/issues/107).

## Destination support

| IMDb source | Trakt | TMDb |
| --- | --- | --- |
| Ratings | Supported | Supported |
| Rating-derived history | Supported | Not available through the current TMDb account API |
| Watchlist | Supported | Supported |
| Custom lists | Supported | Reserved, but currently blocked because mixed movie/TV list writes require TMDb v4 user-access-token support |

`TMDB_SYNC_HISTORY` and `TMDB_SYNC_LISTS` are included in the destination-scoped configuration model so the interface remains symmetrical. Setting either to `true` currently fails configuration validation with an explicit capability/authentication message rather than silently pretending the destination supports it.

## Privacy model

This repository is intended to contain **application source code only**.

Do not store personal IMDb exports, persistent rating state, Trakt tokens, TMDb credentials, IMDb cookies, or account-specific GitHub Actions logs in a public repository.

For scheduled GitHub Actions use, the recommended architecture is:

```text
PUBLIC SOURCE REPOSITORY
IMDb-Trakt-TMDb-Sync
        ↓ checkout

PRIVATE RUNNER REPOSITORY
├── .github/workflows/sync.yaml
├── repository secrets
├── private Actions logs
└── state/
    ├── imdb-ratings.csv
    ├── sync-state.json
    └── tmdb-id-map.json
```

The private Runner may publish only its final `success`, `failure`, or `cancelled` result back to the public source repository. The relay does not publish Runner logs, state, cookies, tokens, or other secret values.

A ready-to-copy private Runner workflow is provided at [`examples/private-runner/sync.yaml`](examples/private-runner/sync.yaml).

> [!WARNING]
> A normal fork of a public GitHub repository is also public. If you use GitHub Actions for a personal sync, create a **separate private repository** for the Runner instead of storing state and scheduled execution in a public fork.

## Configuration

Environment variables use the `ITS_` prefix. Environment variables override values loaded from `config.yaml`.

For local/container use, prefer environment variables or an untracked `.env` file for secrets. The interactive configuration writer creates/tightens `config.yaml` with mode `0600`, but secret-bearing configuration should still be treated as sensitive.

### IMDb source

| Configuration key | Default | Purpose |
| --- | --- | --- |
| `IMDB_AUTH` | `cookies` | IMDb authentication mode: `credentials`, `cookies`, or `none`. |
| `IMDB_EMAIL` | empty | IMDb account email when `IMDB_AUTH=credentials`. |
| `IMDB_PASSWORD` | empty | IMDb account password when `IMDB_AUTH=credentials`. |
| `IMDB_COOKIEATMAIN` | empty | IMDb `at-main` cookie when `IMDB_AUTH=cookies`. |
| `IMDB_LISTS` | empty | Optional IMDb list IDs to sync. Empty means all available lists. |
| `IMDB_IGNOREDLISTS` | empty | Optional IMDb list IDs to exclude. |
| `IMDB_TRACE` | `false` | Enable browser tracing logs. |
| `IMDB_HEADLESS` | `true` | Run the IMDb browser headlessly. |
| `IMDB_BROWSERPATH` | empty | Optional browser executable path. |

### Trakt destination

| Configuration key | Default | Purpose |
| --- | --- | --- |
| `TRAKT_ENABLED` | `true` | Enable or disable the Trakt destination as a whole. |
| `TRAKT_CLIENTID` | empty | Trakt application client ID; required when Trakt is enabled. |
| `TRAKT_CLIENTSECRET` | empty | Trakt application client secret; required when Trakt is enabled. |
| `TRAKT_TOKENFILE` | `trakt-token.json` | Local Trakt OAuth token file. |
| `TRAKT_SYNC_MODE` | `dry-run` | Trakt destination mode: `dry-run`, `add-only`, or `full`. |
| `TRAKT_SYNC_RATINGS` | `true` | Sync IMDb ratings to Trakt. |
| `TRAKT_SYNC_HISTORY` | `false` | Sync rating-derived Trakt history. |
| `TRAKT_SYNC_WATCHLIST` | `true` | Sync IMDb watchlist to Trakt. |
| `TRAKT_SYNC_LISTS` | `true` | Sync configured IMDb custom lists to Trakt. |
| `TRAKT_SYNC_TIMEOUT` | `15m` | Maximum Trakt destination duration. |

### TMDb destination

| Configuration key | Default | Purpose |
| --- | --- | --- |
| `TMDB_ENABLED` | `false` | Enable or disable the TMDb destination as a whole. |
| `TMDB_READACCESSTOKEN` | empty | TMDb API Read Access Token. |
| `TMDB_SESSIONID` | empty | Authenticated TMDb v3 session ID. |
| `TMDB_SYNC_MODE` | `dry-run` | TMDb destination mode: `dry-run`, `add-only`, or `full`. |
| `TMDB_SYNC_RATINGS` | `true` | Sync IMDb ratings to TMDb. |
| `TMDB_SYNC_HISTORY` | `false` | Reserved for rating-derived TMDb history; `true` is currently rejected because TMDb exposes no equivalent account history API. |
| `TMDB_SYNC_WATCHLIST` | `false` | Sync IMDb watchlist to TMDb. |
| `TMDB_SYNC_LISTS` | `false` | Reserved for IMDb custom-list sync; `true` is currently rejected until TMDb v4 user-access-token list support is implemented. |
| `TMDB_SYNC_TIMEOUT` | `15m` | Maximum TMDb destination duration. |

A destination sync mode applies to every supported enabled feature for that destination:

- `dry-run` — calculate/report planned destination changes without writing.
- `add-only` — add or update destination data without removals.
- `full` — reconcile destination data to IMDb and permit removals where the feature supports them.

For TMDb watchlists, `full` mode has an additional safety guard: removals are suppressed whenever any IMDb watchlist item could not be mapped unambiguously to TMDb.

The shared IMDb source phase is capped by the longer timeout of the enabled destinations. After source hydration, Trakt and TMDb each receive their own destination timeout.

### Legacy `SYNC_*` migration

Legacy `SYNC_MODE`, `SYNC_HISTORY`, `SYNC_RATINGS`, `SYNC_WATCHLIST`, `SYNC_LISTS`, and `SYNC_TIMEOUT` values are accepted as fallback inputs for migration where applicable. New configurations and Runner secrets should use the destination-scoped `TRAKT_SYNC_*` and `TMDB_SYNC_*` names. The sanitized examples no longer define the global `SYNC_*` block.

Two operational state settings remain environment-only:

- `ITS_STATE_DIR` — directory containing persistent rating/state files. Empty disables persistent state.
- `ITS_STATE_RECONCILEINTERVAL` — periodic ratings reconciliation interval; the private Runner example uses `168h`.

Ratings state is shared at the IMDb-source level. If an enabled ratings-state consumer is running in `dry-run`, the baseline is not advanced, preventing that dry-run destination from losing source deltas while another destination writes successfully.

See [`.env.example`](.env.example) and [`config.yaml`](config.yaml) for sanitized examples.

## Authentication

### IMDb

Supported modes:

- `cookies` — uses the IMDb `at-main` session cookie.
- `credentials` — uses IMDb email/password and obtains the authenticated browser session as required.
- `none` — no account authentication; only functionality available from explicitly supplied/public IMDb data can run.

IMDb browser/WAF handling is managed by the application. Treat `IMDB_COOKIEATMAIN`, `IMDB_EMAIL`, and `IMDB_PASSWORD` as secrets.

### Trakt

Set `TRAKT_ENABLED=false` for a TMDb-only installation. When enabled, Trakt authorization uses the OAuth device flow.

On the first run without an existing token, the application prints a verification URL and device code. Approve the code in a browser. The resulting access/refresh token pair is written to `TRAKT_TOKENFILE`.

After authorization, the Trakt transport:

1. injects the bearer access token,
2. refreshes expired credentials automatically,
3. handles refresh-token rotation, and
4. writes the current token pair back to the token file.

For the private GitHub Actions Runner, the workflow also writes the potentially rotated token back to the `TRAKT_TOKEN` repository secret so unattended scheduled runs continue working. Token seeding/persistence is skipped when `TRAKT_ENABLED=false`.

### TMDb

Set `TMDB_ENABLED=true` to enable the TMDb destination. Ratings and watchlist sync use the TMDb API Read Access Token plus an authenticated v3 session ID:

```text
TMDB_ENABLED=true
TMDB_READACCESSTOKEN=<TMDb API Read Access Token>
TMDB_SESSIONID=<authenticated TMDb session ID>
TMDB_SYNC_RATINGS=true
TMDB_SYNC_WATCHLIST=true
TMDB_SYNC_MODE=add-only
```

Ratings and watchlist can be enabled independently. `TMDB_SYNC_HISTORY=true` is currently rejected because TMDb has no Trakt-like account watch-history API. `TMDB_SYNC_LISTS=true` is currently rejected because mixed movie/TV custom-list writes require a TMDb v4 user access token, which is not part of the current TMDb authentication model.

Persistent ratings state uses the IMDb ratings snapshot as the baseline. On bootstrap, the current ratings snapshot becomes the baseline after successful writable ratings destinations; historical ratings are not replayed as a new delta.

## Run with a private GitHub Actions Runner

### 1. Create the repositories

Use this repository as the public/read-only application source.

Create a **separate private repository** for scheduled execution, for example:

```text
my-IMDb-Trakt-TMDb-Sync-Runner
```

Initialize the private repository with a `README.md` so it has a `main` branch.

### 2. Install the Runner workflow

Copy:

```text
examples/private-runner/sync.yaml
```

from this repository to:

```text
.github/workflows/sync.yaml
```

in the private Runner repository.

The example checks out `scottia/IMDb-Trakt-TMDb-Sync@main`. If you maintain your own source fork, change the `repository:` value in the `Check out application source` step.

The example's `publish-status` job also targets `scottia/IMDb-Trakt-TMDb-Sync` through `STATUS_REPOSITORY`. Change that value to your own `{username}/{repo-name}` if you want your Runner to drive the sync badge in your fork.

The workflow runs every 12 hours by default and also supports manual dispatch. Push-triggering is restricted to changes to the Runner workflow itself, so state commits do not recursively start another sync.

### 3. Add Runner repository secrets

Create only the secrets required by the destinations/features you use under:

```text
Settings → Secrets and variables → Actions
```

The workflow recognizes:

```text
IMDB_AUTH
IMDB_EMAIL
IMDB_PASSWORD
IMDB_COOKIEATMAIN
IMDB_LISTS
IMDB_IGNOREDLISTS
IMDB_TRACE
IMDB_HEADLESS

TRAKT_ENABLED
TRAKT_CLIENTID
TRAKT_CLIENTSECRET
TRAKT_TOKEN
TRAKT_SYNC_MODE
TRAKT_SYNC_HISTORY
TRAKT_SYNC_RATINGS
TRAKT_SYNC_WATCHLIST
TRAKT_SYNC_LISTS
TRAKT_SYNC_TIMEOUT

TMDB_ENABLED
TMDB_READ_ACCESS_TOKEN
TMDB_SESSION_ID
TMDB_SYNC_MODE
TMDB_SYNC_HISTORY
TMDB_SYNC_RATINGS
TMDB_SYNC_WATCHLIST
TMDB_SYNC_LISTS
TMDB_SYNC_TIMEOUT

GH_PAT
STATUS_PAT
```

`IMDB_LISTS` and `IMDB_IGNOREDLISTS` are optional. When supplied as repository secrets/environment variables, use comma-separated list IDs.

If the destination-control secrets are omitted, the example supplies the documented defaults. `STATUS_PAT` is optional unless you want the public Runner-status badge; the example safely skips status publication when it is not configured.

The Runner maps repository secret names to application environment variables, for example:

```text
TRAKT_ENABLED           → ITS_TRAKT_ENABLED
TRAKT_SYNC_RATINGS      → ITS_TRAKT_SYNC_RATINGS
TRAKT_SYNC_MODE         → ITS_TRAKT_SYNC_MODE
TRAKT_SYNC_TIMEOUT      → ITS_TRAKT_SYNC_TIMEOUT

TMDB_ENABLED            → ITS_TMDB_ENABLED
TMDB_READ_ACCESS_TOKEN  → ITS_TMDB_READACCESSTOKEN
TMDB_SESSION_ID         → ITS_TMDB_SESSIONID
TMDB_SYNC_RATINGS       → ITS_TMDB_SYNC_RATINGS
TMDB_SYNC_WATCHLIST     → ITS_TMDB_SYNC_WATCHLIST
TMDB_SYNC_MODE          → ITS_TMDB_SYNC_MODE
TMDB_SYNC_TIMEOUT       → ITS_TMDB_SYNC_TIMEOUT
```

### 4. Create the `GH_PAT` secret

The default `GITHUB_TOKEN` cannot update repository Actions secrets. A fine-grained personal access token is therefore used only to persist a rotated `TRAKT_TOKEN` when Trakt is enabled.

Create a fine-grained token with:

```text
Repository access:
  Only select repositories
    ✓ <your private Runner repository>

Repository permissions:
  Secrets: Read and write
```

GitHub automatically grants read access to repository metadata.

Store that token in the private Runner repository as:

```text
GH_PAT
```

If the application source repository is private rather than public, the same token (or another token used for checkout) must also have appropriate Contents access to the private source repository.

### 5. Optional public Runner status badge

This repository includes [`.github/workflows/runner-sync-status.yaml`](.github/workflows/runner-sync-status.yaml), a status-only relay used by the sync badge at the top of this README.

To publish the private Runner result to your public source fork:

1. Create a **separate fine-grained personal access token** scoped only to your public source repository.
2. Grant that token `Contents: Read and write` repository permission.
3. Store it in the **private Runner repository** as the Actions secret `STATUS_PAT`.
4. Set `STATUS_REPOSITORY` in the Runner workflow to your public `{username}/{repo-name}`.
5. Keep `runner-sync-status.yaml` on the public repository's default branch.

After the private `sync` job finishes, the separate `publish-status` job sends only the final `success`, `failure`, or `cancelled` result. The public relay converts that result into the badge status. No Runner state, logs, cookies, Trakt tokens, TMDb credentials, or other secret values are sent to the public repository.

Keep `GH_PAT` and `STATUS_PAT` separate: `GH_PAT` is scoped to the private Runner for Trakt-token rotation, while `STATUS_PAT` is scoped to the public source repository only for the status relay.

### 6. First Trakt authorization

Skip this section when `TRAKT_ENABLED=false`.

If `TRAKT_TOKEN` does not exist yet:

1. manually run the Runner `sync` workflow,
2. open its `Sync` step log,
3. follow the Trakt verification URL/device code,
4. approve the application before the device-flow timeout.

After authorization, the workflow persists `TRAKT_TOKEN` automatically. Subsequent scheduled runs can refresh and rotate it unattended.

### 7. Private persistent state

The Runner stores state in its own private repository:

```text
state/imdb-ratings.csv
state/sync-state.json
state/tmdb-id-map.json
```

The workflow commits state only after successful destinations and only when the ratings baseline is eligible to advance. This keeps the IMDb ratings baseline and TMDb mapping cache private and durable between ephemeral GitHub-hosted runners.

## Run in Docker

1. Install [Docker](https://www.docker.com/get-started).
2. Clone this repository:

   ```bash
   git clone https://github.com/scottia/IMDb-Trakt-TMDb-Sync.git
   cd IMDb-Trakt-TMDb-Sync
   ```

3. Configure at least one destination. If using Trakt, create a Trakt application and use `urn:ietf:wg:oauth:2.0:oob` as the redirect URI.
4. Copy `.env.example` to `.env` and supply your own values.
5. Build and run:

   ```bash
   make package
   make sync-container
   ```

The local `trakt-token.json` is intentionally excluded from Git.

## Run locally

1. Install [Git](https://git-scm.com/downloads) and [Go](https://go.dev/doc/install).
2. Clone the repository:

   ```bash
   git clone https://github.com/scottia/IMDb-Trakt-TMDb-Sync.git
   cd IMDb-Trakt-TMDb-Sync
   ```

3. Configure at least one destination. If using Trakt, create a Trakt application and use `urn:ietf:wg:oauth:2.0:oob` as the redirect URI.
4. Build:

   ```bash
   make build
   ```

5. Configure interactively:

   ```bash
   make configure
   ```

6. Run:

   ```bash
   make sync
   ```

Environment variables can be used instead of storing secrets in `config.yaml`. For example, a TMDb-only ratings/watchlist setup can use:

```text
ITS_TRAKT_ENABLED=false
ITS_TMDB_ENABLED=true
ITS_TMDB_READACCESSTOKEN=<token>
ITS_TMDB_SESSIONID=<session-id>
ITS_TMDB_SYNC_RATINGS=true
ITS_TMDB_SYNC_WATCHLIST=true
ITS_TMDB_SYNC_MODE=add-only
```

## Security notes

- Never commit `.env`, `trakt-token.json`, IMDb cookies, passwords, API tokens, or private state.
- Keep scheduled personal execution in a private Runner repository so Actions logs are not public.
- Keep `GH_PAT` scoped to the private Runner and `STATUS_PAT` scoped only to the public source repository.
- Prefer environment variables/`.env` over long-lived secrets in `config.yaml`.
- The interactive configuration TUI masks secret fields and writes `config.yaml` with owner-only permissions (`0600`).
- The application does not require the Trakt token architecture to be replaced by static access tokens; refresh-token rotation is handled automatically.

## License

See [`LICENSE`](LICENSE).
