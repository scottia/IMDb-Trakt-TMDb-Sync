[![sync](https://github.com/scottia/IMDb-Trakt-TMDb-Sync/actions/workflows/runner-sync-status.yaml/badge.svg)](https://github.com/scottia/IMDb-Trakt-TMDb-Sync/actions/workflows/runner-sync-status.yaml)
[![quality](https://github.com/scottia/IMDb-Trakt-TMDb-Sync/actions/workflows/quality.yaml/badge.svg?branch=main)](https://github.com/scottia/IMDb-Trakt-TMDb-Sync/actions/workflows/quality.yaml?query=branch%3Amain)

> **Note for forks:** The badges above are hardcoded to this repository. After forking, update the badge URLs at the top of this file, replacing `scottia/IMDb-Trakt-TMDb-Sync` with your own `{username}/{repo-name}`. If you use the private Runner workflow, also update the `repository:` value in the `Check out application source` step and `STATUS_REPOSITORY` in the `publish-status` job so Runner status is published to your fork.

# IMDb-Trakt-TMDb-Sync

<img src="./assets/logo.png" alt="IMDb to Trakt and TMDb"/>

One-way synchronization from [IMDb](https://www.imdb.com/) to independently configurable destinations:

- **[Trakt](https://trakt.tv/):** ratings, rating-derived history, watchlist, and custom lists.
- **[TMDb](https://www.themoviedb.org/):** ratings and watchlist through the TMDb API.

IMDb is the source of truth. Changes made directly on Trakt or TMDb are not written back to IMDb.

Trakt and TMDb are independently enabled and each has its own sync mode and timeout. The current defaults are intentionally opinionated: **both destinations enabled**, **all supported sync features enabled**, **`add-only` mode**, and **`30m` timeout**. Disable a destination or feature in `config.yaml` when you do not want it.

The **sync** badge reflects the latest private Runner result through a status-only `repository_dispatch` relay. The **quality** badge reflects public source CI.

> [!IMPORTANT]
> Trakt API app creation currently requires Trakt VIP access. See upstream issue [#107](https://github.com/cecobask/imdb-trakt-sync/issues/107).

## Destination support

| IMDb source | Trakt | TMDb |
| --- | --- | --- |
| Ratings | Supported | Supported |
| Rating-derived history | Supported | Not available through the current TMDb account API |
| Watchlist | Supported | Supported |
| Custom lists | Supported | Not currently supported with the existing TMDb authentication model |

`TMDB_SYNC_HISTORY` and `TMDB_SYNC_LISTS` are intentionally omitted from the public example configuration. They are not advertised as usable switches until the required TMDb capabilities/authentication are available.

## Configuration

The canonical configuration reference is [`example.config.yaml`](example.config.yaml).

Copy it to an untracked `config.yaml` and edit the behavior you want:

```bash
cp example.config.yaml config.yaml
```

`config.yaml` is ignored by Git. For a private Runner, keep its `config.yaml` in the **private Runner repository**, not in this public source repository.

The configuration hierarchy is:

```text
built-in defaults
        ↓
config.yaml
        ↓
ITS_* environment variables / GitHub Actions secrets
```

Environment variables override YAML values. This keeps ordinary behavior readable in one YAML file while credentials remain outside the file.

A minimal configuration using the project defaults can be as small as:

```yaml
TRAKT:
  ENABLED: true
  SYNC:
    MODE: add-only
    TIMEOUT: 30m

TMDB:
  ENABLED: true
  SYNC:
    MODE: add-only
    TIMEOUT: 30m
```

Supported Trakt feature switches (`HISTORY`, `RATINGS`, `WATCHLIST`, `LISTS`) default to `true`. Supported TMDb feature switches (`RATINGS`, `WATCHLIST`) also default to `true`. See [`example.config.yaml`](example.config.yaml) for the complete configuration surface, including IMDb source settings and optional browser/list controls.

### Sync modes

Each destination has its own `SYNC.MODE`:

- `dry-run` — calculate/report planned changes without writing.
- `add-only` — add or update destination data without removals. **Default.**
- `full` — reconcile the destination to IMDb and permit removals where the feature supports them.

For TMDb watchlists, `full` mode suppresses removals if any IMDb watchlist item could not be mapped unambiguously to TMDb.

Each destination also has its own `SYNC.TIMEOUT`; the default is `30m`. The shared IMDb source phase uses the longer timeout of the enabled destinations.

### Local use

After copying `example.config.yaml` to `config.yaml`, provide secrets through environment variables or an untracked `.env` file and run:

```bash
make sync
```

The interactive configurator can also bootstrap a missing `config.yaml` from `example.config.yaml`:

```bash
make configure
```

The configurator writes `config.yaml` with mode `0600`. Even so, environment variables are preferred for credentials.

## Required secrets and credentials

Only secrets required by the destinations/authentication you actually enable need to be configured.

| Runner secret | Application environment variable | Required when | Purpose / how to obtain |
| --- | --- | --- | --- |
| `IMDB_COOKIEATMAIN` | `ITS_IMDB_COOKIEATMAIN` | Default `IMDB.AUTH: cookies` | Sign in to IMDb in a desktop browser, open Developer Tools → Application/Storage → Cookies → `https://www.imdb.com`, and copy the `at-main` cookie value. Treat it like a password. |
| `TRAKT_CLIENTID` | `ITS_TRAKT_CLIENTID` | Trakt enabled | Create a Trakt API application under [Trakt API Apps](https://app.trakt.tv/settings/apps) and copy its Client ID. |
| `TRAKT_CLIENTSECRET` | `ITS_TRAKT_CLIENTSECRET` | Trakt enabled | From the same Trakt API application, copy its Client Secret. |
| `TRAKT_TOKEN` | Seeded into `ITS_TRAKT_TOKENFILE` | Unattended Trakt runs after first authorization | Do not invent this value manually. On the first Trakt run, follow the device-code authorization shown in the Actions log. The resulting OAuth token is stored in `trakt-token.json` and the Runner persists the rotated value back to this secret. |
| `TMDB_READ_ACCESS_TOKEN` | `ITS_TMDB_READACCESSTOKEN` | TMDb enabled | Create/approve TMDb API access under [TMDb API settings](https://www.themoviedb.org/settings/api) and copy the **API Read Access Token**. |
| `TMDB_SESSION_ID` | `ITS_TMDB_SESSIONID` | TMDb enabled | Generate an authenticated v3 user session using TMDb's official flow: create request token → authorize it in the browser → create session ID. See [How do I generate a session id?](https://developer.themoviedb.org/reference/authentication-how-do-i-generate-a-session-id). |
| `GH_PAT` | Runner-only | Unattended Trakt token rotation | Fine-grained PAT scoped only to the private Runner repository with **Secrets: Read and write**. Used only to update `TRAKT_TOKEN`. |
| `STATUS_PAT` | Runner-only | Optional public sync badge | Separate fine-grained PAT scoped only to the public source repository with **Contents: Read and write**. Used only for `repository_dispatch` status relay. |

IMDb credential authentication is also supported for local use with `IMDB.AUTH: credentials`, `ITS_IMDB_EMAIL`, and `ITS_IMDB_PASSWORD`, but browser/CAPTCHA challenges can make it less suitable for unattended GitHub Actions.

### Trakt OAuth behavior

Trakt uses OAuth device authorization. On first use, the application prints a verification URL and device code. After approval it stores the access/refresh token pair in `TRAKT_TOKENFILE`. Refresh tokens rotate, so unattended Runner use should retain `GH_PAT` so the newest token can be written back to the private repository secret.

### TMDb session behavior

TMDb ratings and watchlist writes use both the API Read Access Token and an authenticated v3 `session_id`. The read token authenticates the application; the session ID identifies the TMDb user account receiving rating/watchlist changes.

## Private GitHub Actions Runner

Scheduled personal sync should run from a **separate private repository**. A normal fork of this public repository is also public, so do not put personal state, cookies, tokens, or Actions secrets in a public fork.

Recommended layout:

```text
PUBLIC SOURCE REPOSITORY
IMDb-Trakt-TMDb-Sync
        ↓ checkout

PRIVATE RUNNER REPOSITORY
├── .github/workflows/sync.yaml
├── config.yaml
├── repository secrets
└── state/
    ├── imdb-ratings.csv
    ├── sync-state.json
    └── tmdb-id-map.json
```

A ready-to-copy workflow is provided at [`examples/private-runner/sync.yaml`](examples/private-runner/sync.yaml).

### Runner setup

1. Create a separate **private** Runner repository and initialize its `main` branch.
2. Copy [`examples/private-runner/sync.yaml`](examples/private-runner/sync.yaml) to `.github/workflows/sync.yaml` in that private repository.
3. Copy [`example.config.yaml`](example.config.yaml) to `config.yaml` in the private Runner and customize non-secret behavior there.
4. Add only the required repository secrets from the table above under **Settings → Secrets and variables → Actions**.
5. Manually dispatch the first run. If Trakt has no `TRAKT_TOKEN` yet, authorize the displayed device code.

The Runner workflow passes only sensitive credentials to the application:

```yaml
env:
  ITS_IMDB_COOKIEATMAIN: ${{ secrets.IMDB_COOKIEATMAIN }}

  ITS_TRAKT_CLIENTID: ${{ secrets.TRAKT_CLIENTID }}
  ITS_TRAKT_CLIENTSECRET: ${{ secrets.TRAKT_CLIENTSECRET }}
  ITS_TRAKT_TOKENFILE: ${{ github.workspace }}/trakt-token.json

  ITS_TMDB_READACCESSTOKEN: ${{ secrets.TMDB_READ_ACCESS_TOKEN }}
  ITS_TMDB_SESSIONID: ${{ secrets.TMDB_SESSION_ID }}
```

All destination switches, modes, feature choices, list IDs, tracing behavior, and timeouts belong in private `config.yaml` instead of dozens of GitHub Actions secrets.

The provided production workflow runs every 12 hours and supports manual dispatch. Push-triggering is limited to changes to the workflow or private `config.yaml`, so state commits do not recursively start another sync.

## Persistent ratings state

The private Runner stores persistent source state under `state/`. `ITS_STATE_DIR` and `ITS_STATE_RECONCILEINTERVAL` remain runtime-only environment settings because their paths/cadence are Runner operational details rather than destination configuration.

Ratings state is shared at the IMDb-source level. If an enabled ratings-state consumer runs in `dry-run`, the baseline is not advanced, preventing that dry-run destination from losing source deltas while another destination writes successfully.

## Public Runner-status badge

The private Runner can publish only its final `success`, `failure`, or `cancelled` result back to the public repository through `repository_dispatch`. No Runner logs, state, IMDb cookies, Trakt tokens, TMDb credentials, or other secret values cross the boundary.

The public relay workflow is [`.github/workflows/runner-sync-status.yaml`](.github/workflows/runner-sync-status.yaml).

For a fork, change both:

- the `repository:` value in `Check out application source`, and
- `STATUS_REPOSITORY` in the Runner `publish-status` job.

Keep `GH_PAT` and `STATUS_PAT` separate: the first is scoped to the private Runner for Trakt-token rotation; the second is scoped to the public source repository only for status publication.

## Development

```bash
go test ./...
make build
make lint
```

Public `quality` CI runs lint, tests, and build on `main`, `dev`, and pull requests.

## License

MIT. See [`LICENSE`](LICENSE).
