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
| `IMDB_COOKIEATMAIN` | `ITS_IMDB_COOKIEATMAIN` | Default IMDb cookie auth / bootstrap fallback | Sign in to IMDb in a desktop browser, open Developer Tools → Application/Storage → Cookies → `https://www.imdb.com`, then copy the `at-main` cookie value. Treat it like a password. |
| `IMDB_COOKIE_JAR` | Seeded into `ITS_IMDB_COOKIEJARFILE` | Unattended IMDb cookie recycling after the first successful bootstrap | **Do not create manually.** If absent, the Runner bootstraps from `IMDB_COOKIEATMAIN`; after successful IMDb authentication and hydration, it persists the allowlisted IMDb cookie jar to this secret and reuses it on later runs. |
| `TRAKT_CLIENTID` | `ITS_TRAKT_CLIENTID` | Trakt enabled | Create a Trakt API application at [Trakt API Apps](https://app.trakt.tv/settings/apps) and copy its Client ID. |
| `TRAKT_CLIENTSECRET` | `ITS_TRAKT_CLIENTSECRET` | Trakt enabled | Copy the Client Secret from the same Trakt API application. |
| `TRAKT_TOKEN` | Seeded into `TRAKT_TOKENFILE` | Unattended Trakt runs after first authorization | Do not create manually. On the first run, follow the device-code authorization shown in the workflow log. Normal refreshes use Trakt's OAuth token endpoint and the Runner persists the replacement access/refresh token pair back to this secret. |
| `TMDB_READ_ACCESS_TOKEN` | `ITS_TMDB_READACCESSTOKEN` | TMDb enabled | Create/approve TMDb API access under [TMDb API settings](https://www.themoviedb.org/settings/api) and copy the API Read Access Token. |
| `TMDB_SESSION_ID` | `ITS_TMDB_SESSIONID` | TMDb enabled | Use TMDb v3 user authentication: create a request token, authorize it in a browser, then create a session ID. See [TMDb session authentication](https://developer.themoviedb.org/reference/authentication-how-do-i-generate-a-session-id). |
| `GH_PAT` | Runner-only | Trakt token rotation and IMDb cookie-jar persistence | Fine-grained PAT scoped only to the private Runner with **Secrets: Read and write**. Used only to update Runner-managed authentication secrets. |
| `STATUS_PAT` | Runner-only | Optional public sync badge | Separate fine-grained PAT scoped only to the public source repository with **Contents: Read and write**. Used only for the status `repository_dispatch`. |

IMDb credential authentication is also supported for local use by setting `IMDB.AUTH: credentials` in `config.yaml` and supplying `ITS_IMDB_EMAIL` / `ITS_IMDB_PASSWORD`. Browser/CAPTCHA challenges can make that less suitable for unattended Actions.

### IMDb cookie-session recycling

The Runner does not require a hand-built `IMDB_COOKIE_JAR`. The first successful run is self-bootstrapping:

```text
existing IMDB_COOKIEATMAIN secret
        ↓
seed Chrome
        ↓
authenticate successfully
        ↓
hydrate the IMDb client successfully
        ↓
capture current allowlisted IMDb cookies
        ↓
write imdb-cookie-jar.json
        ↓
Runner creates/updates IMDB_COOKIE_JAR
        ↓
next run seeds Chrome from the persisted jar
```

The application writes the jar only after both IMDb authentication and client hydration succeed. The Runner compares the resulting file with the seeded copy and updates `IMDB_COOKIE_JAR` only when the jar actually changed. If the persisted jar is missing or cannot be loaded, the Runner falls back to the existing `IMDB_COOKIEATMAIN` bootstrap path.

Only allowlisted IMDb-domain authentication cookies are retained. Amazon-domain cookies, the general Chrome profile, and the generated `aws-waf-token` are not persisted. Cookie values are never intentionally written to logs. The jar preserves session continuity across ephemeral Actions runners; it does not create an IMDb refresh-token API, so IMDb can still require a fresh interactive cookie bootstrap if it invalidates the session server-side.

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

A ready-to-copy production workflow is provided at [`examples/private-runner/sync.yaml`](examples/private-runner/sync.yaml).

### Runner setup

1. Create a separate private Runner repository and initialize `main`.
2. Copy `examples/private-runner/sync.yaml` to `.github/workflows/sync.yaml` in the private Runner.
3. Copy this repository's `config.yaml` to the private Runner and customize non-secret behavior there.
4. Add only the required bootstrap/static secrets from the table above. Do not manually create `TRAKT_TOKEN` or `IMDB_COOKIE_JAR`; those are Runner-managed after their respective bootstrap flows.
5. Manually dispatch the first run. If Trakt has no `TRAKT_TOKEN` yet, authorize the displayed device code.

The production Runner workflow passes sensitive credentials and ephemeral credential-file paths to the application:

```yaml
env:
  ITS_IMDB_COOKIEATMAIN: ${{ secrets.IMDB_COOKIEATMAIN }}
  ITS_IMDB_COOKIEJARFILE: ${{ github.workspace }}/imdb-cookie-jar.json

  ITS_TRAKT_CLIENTID: ${{ secrets.TRAKT_CLIENTID }}
  ITS_TRAKT_CLIENTSECRET: ${{ secrets.TRAKT_CLIENTSECRET }}
  ITS_TRAKT_TOKENFILE: ${{ github.workspace }}/trakt-token.json

  ITS_TMDB_READACCESSTOKEN: ${{ secrets.TMDB_READ_ACCESS_TOKEN }}
  ITS_TMDB_SESSIONID: ${{ secrets.TMDB_SESSION_ID }}
```

The private Runner seeds `ITS_IMDB_COOKIEJARFILE` from the `IMDB_COOKIE_JAR` repository secret when present, lets the application refresh the allowlisted jar after successful authentication/hydration, and persists a changed jar back to the same secret.

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

Local cookie-auth users may set `ITS_IMDB_COOKIEJARFILE` to a protected local path to retain the same allowlisted IMDb session cookies between runs. `ITS_IMDB_COOKIEATMAIN` remains the bootstrap/fallback credential.

## Persistent ratings state

The private Runner stores persistent source state under `state/`. `ITS_STATE_DIR` and `ITS_STATE_RECONCILEINTERVAL` remain runtime-only environment settings because their paths/cadence are Runner operational details rather than destination policy.

Ratings state is shared at the IMDb-source level. If an enabled ratings-state consumer runs in `dry-run`, the baseline is not advanced, preventing that destination from losing source deltas while another destination writes successfully.

## Public Runner-status badge

The private Runner can publish only its final `success`, `failure`, or `cancelled` result back to the public repository through `repository_dispatch`. No Runner logs, state, cookies, Trakt tokens, or TMDb credentials cross the boundary.

The public relay workflow is [`.github/workflows/runner-sync-status.yaml`](.github/workflows/runner-sync-status.yaml).

For a fork, change both the source checkout `repository:` value and `STATUS_REPOSITORY` in the Runner's `publish-status` job.

Keep `GH_PAT` and `STATUS_PAT` separate: `GH_PAT` is scoped to the private Runner for Runner-managed authentication-secret updates; `STATUS_PAT` is scoped to the public source repository only for status publication.

## Development

```bash
go test ./...
make build
make lint
```

Public `quality` CI runs lint, tests, and build on `main`, `dev`, and pull requests.

For this repository's private Runner pairing, scheduled production runs select Runner `main` and application source `main`. A manual Runner dispatch with the workflow branch set to `dev` selects Runner `dev` and application source `dev`, which remains the path for validating future staged changes without changing the scheduled production branch.

## License

MIT. See [`LICENSE`](LICENSE).
