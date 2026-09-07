[![sync](https://github.com/scottia/IMDb-Trakt-TMDb-Sync/actions/workflows/runner-sync-status.yaml/badge.svg)](https://github.com/scottia/IMDb-Trakt-TMDb-Sync/actions/workflows/runner-sync-status.yaml)
[![quality](https://github.com/scottia/IMDb-Trakt-TMDb-Sync/actions/workflows/quality.yaml/badge.svg?branch=main)](https://github.com/scottia/IMDb-Trakt-TMDb-Sync/actions/workflows/quality.yaml?query=branch%3Amain)

> **Note for forks:** The badges above are hardcoded to this repository. After forking, replace `scottia/IMDb-Trakt-TMDb-Sync` in the badge URLs with your own `{username}/{repo-name}`. If you use the private Runner workflow, also update the source `repository:` value and `STATUS_REPOSITORY` so status is published to your fork.

# IMDb-Trakt-TMDb-Sync

<img src="./assets/logo.png" alt="IMDb to Trakt and TMDb"/>

One-way synchronization from IMDb to independently configurable Trakt and TMDb destinations.

- **Trakt:** ratings, rating-derived history, watchlist, and custom lists.
- **TMDb:** ratings and watchlist.

IMDb is the source of truth. Changes made directly on Trakt or TMDb are not written back to IMDb.

> [!IMPORTANT]
> Trakt API app creation currently requires Trakt VIP access. Manage API applications from [Trakt API Apps](https://app.trakt.tv/settings/apps).

## Destination support

| IMDb source | Trakt | TMDb |
| --- | --- | --- |
| Ratings | Supported | Supported |
| Rating-derived history | Supported | Not available through the current TMDb account API |
| Watchlist | Supported | Supported |
| Custom lists | Supported | Not currently supported with the existing TMDb authentication model |

Unsupported TMDb history/list switches are intentionally not exposed in `config.yaml`.

## Configuration

[`config.yaml`](config.yaml) is the single canonical configuration file and also acts as its own help/reference file. Every supported **non-secret** setting is documented inline with `#` comments. Credential/secret keys are intentionally not duplicated in the YAML; they are documented in [Secrets and credentials](#secrets-and-credentials).

The checked-in defaults are:

```yaml
TRAKT:
  ENABLED: true
  SYNC:
    MODE: add-only
    HISTORY: true
    RATINGS: true
    WATCHLIST: true
    LISTS: true
    TIMEOUT: 30m

TMDB:
  ENABLED: true
  SYNC:
    MODE: add-only
    RATINGS: true
    WATCHLIST: true
    TIMEOUT: 30m
```

Edit the non-secret behavior in `config.yaml`. Keep credentials out of the file and provide them through `ITS_*` environment variables or GitHub Actions secrets.

Configuration precedence is:

```text
built-in defaults
        ↓
config.yaml
        ↓
ITS_* environment variables / GitHub Actions secrets
```

Each destination has an independent sync mode:

- `dry-run` — calculate/report changes without writing.
- `add-only` — add or update data without removals. **Default.**
- `full` — reconcile to IMDb and permit removals where supported.

Each destination has its own timeout; the default is `30m`.

For TMDb watchlists, `full` mode suppresses removals if any IMDb watchlist item cannot be mapped unambiguously to TMDb.

## Secrets and credentials

Only add secrets required by the destinations you enable. For GitHub Actions, create them under:

```text
Settings → Secrets and variables → Actions
```

| Runner secret | Application variable | Required when | Purpose / how to obtain |
| --- | --- | --- | --- |
| `IMDB_COOKIEATMAIN` | `ITS_IMDB_COOKIEATMAIN` | Default IMDb cookie auth | Sign in to IMDb in a desktop browser, open Developer Tools → Application/Storage → Cookies → `https://www.imdb.com`, then copy the `at-main` cookie value. Treat it like a password. |
| `TRAKT_CLIENTID` | `ITS_TRAKT_CLIENTID` | Trakt enabled | Create a Trakt API application at [Trakt API Apps](https://app.trakt.tv/settings/apps) and copy its Client ID. |
| `TRAKT_CLIENTSECRET` | `ITS_TRAKT_CLIENTSECRET` | Trakt enabled | Copy the Client Secret from the same Trakt API application. |
| `TRAKT_TOKEN` | Seeded into `TRAKT_TOKENFILE` | Unattended Trakt runs after first authorization | Do not create manually. On the first run, follow the device-code authorization shown in the workflow log. The Runner persists the resulting/rotated OAuth token back to this secret. |
| `TMDB_READ_ACCESS_TOKEN` | `ITS_TMDB_READACCESSTOKEN` | TMDb enabled | Create/approve TMDb API access under [TMDb API settings](https://www.themoviedb.org/settings/api) and copy the API Read Access Token. |
| `TMDB_SESSION_ID` | `ITS_TMDB_SESSIONID` | TMDb enabled | Use TMDb v3 user authentication: create a request token, authorize it in a browser, then create a session ID. See [TMDb session authentication](https://developer.themoviedb.org/reference/authentication-how-do-i-generate-a-session-id). |
| `GH_PAT` | Runner-only | Trakt token rotation | Fine-grained PAT scoped only to the private Runner with **Secrets: Read and write**. Used only to update `TRAKT_TOKEN`. |
| `STATUS_PAT` | Runner-only | Optional public sync badge | Separate fine-grained PAT scoped only to the public source repository with **Contents: Read and write**. Used only for the status `repository_dispatch`. |

IMDb credential authentication is also supported for local use by setting `IMDB.AUTH: credentials` in `config.yaml` and supplying `ITS_IMDB_EMAIL` / `ITS_IMDB_PASSWORD`. Browser/CAPTCHA challenges can make that less suitable for unattended Actions.

## Private GitHub Actions Runner

Scheduled personal sync should run from a **separate private repository**. Do not store personal state, cookies, OAuth tokens, or Actions secrets in a public fork.

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

1. Create a separate private Runner repository and initialize `main`.
2. Copy `examples/private-runner/sync.yaml` to `.github/workflows/sync.yaml` in the private Runner.
3. Copy this repository's `config.yaml` to the private Runner and customize non-secret behavior there.
4. Add only the required secrets from the table above.
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

All destination switches, modes, feature choices, list IDs, tracing behavior, and timeouts belong in private `config.yaml`, not in dozens of GitHub Actions secrets.

The production example runs every 12 hours and supports manual dispatch. Push-triggering is limited to workflow/config changes so state commits do not recursively start another sync.

## Local / container use

For local execution, edit `config.yaml`, provide secrets through environment variables or an untracked `.env`, then run:

```bash
make sync
```

Interactive configuration remains available with:

```bash
make configure
```

For the container:

```bash
make package
make sync-container
```

The container uses `config.yaml` and the untracked `.env` at runtime.

## Persistent ratings state

The private Runner stores persistent source state under `state/`. `ITS_STATE_DIR` and `ITS_STATE_RECONCILEINTERVAL` remain runtime-only environment settings because their paths/cadence are Runner operational details rather than destination policy.

Ratings state is shared at the IMDb-source level. If an enabled ratings-state consumer runs in `dry-run`, the baseline is not advanced, preventing that destination from losing source deltas while another destination writes successfully.

## Public Runner-status badge

The private Runner can publish only its final `success`, `failure`, or `cancelled` result back to the public repository through `repository_dispatch`. No Runner logs, state, cookies, Trakt tokens, or TMDb credentials cross the boundary.

The public relay workflow is [`.github/workflows/runner-sync-status.yaml`](.github/workflows/runner-sync-status.yaml).

For a fork, change both the source checkout `repository:` value and `STATUS_REPOSITORY` in the Runner's `publish-status` job.

Keep `GH_PAT` and `STATUS_PAT` separate: `GH_PAT` is scoped to the private Runner for Trakt-token rotation; `STATUS_PAT` is scoped to the public source repository only for status publication.

## Development

```bash
go test ./...
make build
make lint
```

Public `quality` CI runs lint, tests, and build on `main`, `dev`, and pull requests.

## License

MIT. See [`LICENSE`](LICENSE).
