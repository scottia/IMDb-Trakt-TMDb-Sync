[![sync](https://github.com/scottia/IMDb-Trakt-TMDb-Sync/actions/workflows/runner-sync-status.yaml/badge.svg?branch=main)](https://github.com/scottia/IMDb-Trakt-TMDb-Sync/actions/workflows/runner-sync-status.yaml?query=branch%3Amain)

> **Note for forks:** The badge above is hardcoded to this repository. After forking, update the badge URLs at the top of this file, replacing `scottia/IMDb-Trakt-TMDb-Sync` with your own `{username}/{repo-name}`. If you use the private Runner workflow, also update the `repository:` value in the `Check out application source` step and the `repository_dispatch` target so Runner status is published to your fork.

# IMDb-Trakt-TMDb-Sync

<img src="./assets/logo.png" alt="IMDb to Trakt and TMDb"/>

One-way synchronization from [IMDb](https://www.imdb.com/) to:

- **[Trakt](https://trakt.tv/dashboard):** watchlist, lists, ratings, and rating-derived history.
- **[TMDb](https://www.themoviedb.org):** ratings only, through the TMDb API when the optional TMDb destination is enabled.

IMDb is the source of truth. Changes made directly on Trakt or TMDb are not written back to IMDb. Destination removals depend on `SYNC_MODE`.

> [!IMPORTANT]
> Trakt API app creation currently requires Trakt VIP access. See upstream issue [#107](https://github.com/cecobask/imdb-trakt-sync/issues/107).

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

A ready-to-copy private Runner workflow is provided at [`examples/private-runner/sync.yaml`](examples/private-runner/sync.yaml).

> [!WARNING]
> A normal fork of a public GitHub repository is also public. If you use GitHub Actions for a personal sync, create a **separate private repository** for the Runner instead of storing state and scheduled execution in a public fork.

## Configuration

Environment variables use the `ITS_` prefix. Environment variables override values loaded from `config.yaml`.

For local/container use, prefer environment variables or an untracked `.env` file for secrets. The interactive configuration writer creates/tightens `config.yaml` with mode `0600`, but secret-bearing configuration should still be treated as sensitive.

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
| `SYNC_MODE` | `dry-run` | `dry-run`, `add-only`, or `full`. |
| `SYNC_HISTORY` | `false` | Sync rating-derived Trakt history. |
| `SYNC_RATINGS` | `true` | Sync ratings to Trakt and, when enabled, TMDb. |
| `SYNC_WATCHLIST` | `true` | Sync IMDb watchlist to Trakt. |
| `SYNC_LISTS` | `true` | Sync IMDb lists to Trakt. |
| `SYNC_TIMEOUT` | `15m` | Maximum sync duration. |
| `TRAKT_CLIENTID` | empty | Trakt application client ID. |
| `TRAKT_CLIENTSECRET` | empty | Trakt application client secret. |
| `TRAKT_TOKENFILE` | `trakt-token.json` | Local Trakt OAuth token file. |
| `TMDB_ENABLED` | `false` | Enable TMDb ratings sync. |
| `TMDB_READACCESSTOKEN` | empty | TMDb API Read Access Token. |
| `TMDB_SESSIONID` | empty | Authenticated TMDb session ID. |

Two operational state settings are environment-only:

- `ITS_STATE_DIR` — directory containing persistent rating/state files. Empty disables persistent state.
- `ITS_STATE_RECONCILEINTERVAL` — periodic reconciliation interval; the private Runner example uses `168h`.

See [`.env.example`](.env.example) and [`config.yaml`](config.yaml) for sanitized examples.

## Authentication

### IMDb

Supported modes:

- `cookies` — uses the IMDb `at-main` session cookie.
- `credentials` — uses IMDb email/password and obtains the authenticated browser session as required.
- `none` — no account authentication; only functionality available from explicitly supplied/public IMDb data can run.

IMDb browser/WAF handling is managed by the application. Treat `IMDB_COOKIEATMAIN`, `IMDB_EMAIL`, and `IMDB_PASSWORD` as secrets.

### Trakt

Trakt authorization uses the OAuth device flow.

On the first run without an existing token, the application prints a verification URL and device code. Approve the code in a browser. The resulting access/refresh token pair is written to `TRAKT_TOKENFILE`.

After authorization, the Trakt transport:

1. injects the bearer access token,
2. refreshes expired credentials automatically,
3. handles refresh-token rotation, and
4. writes the current token pair back to the token file.

For the private GitHub Actions Runner, the workflow also writes the potentially rotated token back to the `TRAKT_TOKEN` repository secret so unattended scheduled runs continue working.

### TMDb

TMDb is optional and synchronizes **ratings only**. It does not modify TMDb watchlists, lists, or history.

To enable it:

```text
TMDB_ENABLED=true
TMDB_READACCESSTOKEN=<TMDb API Read Access Token>
TMDB_SESSIONID=<authenticated TMDb session ID>
```

`SYNC_RATINGS=false` disables both Trakt and TMDb ratings sync.

`SYNC_MODE` affects TMDb behavior:

- `dry-run` — report planned changes without writing.
- `add-only` — add/update ratings without removals.
- `full` — may remove TMDb ratings no longer present in IMDb during reconciliation.

Persistent state uses the IMDb ratings snapshot as the baseline. On bootstrap, the current snapshot becomes the baseline after successful destinations; TMDb writes are skipped for that bootstrap snapshot.

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

The workflow runs every 12 hours by default and also supports manual dispatch. Push-triggering is restricted to changes to the Runner workflow itself, so state commits do not recursively start another sync.

### 3. Add Runner repository secrets

Create only the secrets required by the features you use under:

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

SYNC_MODE
SYNC_HISTORY
SYNC_RATINGS
SYNC_WATCHLIST
SYNC_LISTS
SYNC_TIMEOUT

TRAKT_CLIENTID
TRAKT_CLIENTSECRET
TRAKT_TOKEN

TMDB_ENABLED
TMDB_READ_ACCESS_TOKEN
TMDB_SESSION_ID

GH_PAT
```

`IMDB_LISTS` and `IMDB_IGNOREDLISTS` are optional. When supplied as repository secrets/environment variables, use comma-separated list IDs.

The TMDb secret names are mapped by the Runner workflow as follows:

```text
TMDB_ENABLED            → ITS_TMDB_ENABLED
TMDB_READ_ACCESS_TOKEN  → ITS_TMDB_READACCESSTOKEN
TMDB_SESSION_ID         → ITS_TMDB_SESSIONID
```

### 4. Create the `GH_PAT` secret

The default `GITHUB_TOKEN` cannot update repository Actions secrets. A fine-grained personal access token is therefore used only to persist a rotated `TRAKT_TOKEN`.

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

### 5. First Trakt authorization

If `TRAKT_TOKEN` does not exist yet:

1. manually run the Runner `sync` workflow,
2. open its `Sync` step log,
3. follow the Trakt verification URL/device code,
4. approve the application before the device-flow timeout.

After authorization, the workflow persists `TRAKT_TOKEN` automatically. Subsequent scheduled runs can refresh and rotate it unattended.

### 6. Private persistent state

The Runner stores state in its own private repository:

```text
state/imdb-ratings.csv
state/sync-state.json
state/tmdb-id-map.json
```

The workflow commits state only after successful destinations. This keeps the IMDb ratings baseline and TMDb mapping cache private and durable between ephemeral GitHub-hosted runners.

## Run in Docker

1. Install [Docker](https://www.docker.com/get-started).
2. Clone this repository:

   ```bash
   git clone https://github.com/scottia/IMDb-Trakt-TMDb-Sync.git
   cd IMDb-Trakt-TMDb-Sync
   ```

3. Create a Trakt application and use `urn:ietf:wg:oauth:2.0:oob` as the redirect URI.
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

3. Create a Trakt application and use `urn:ietf:wg:oauth:2.0:oob` as the redirect URI.
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

Environment variables can be used instead of storing secrets in `config.yaml`. For example:

```text
ITS_TMDB_ENABLED=true
ITS_TMDB_READACCESSTOKEN=<token>
ITS_TMDB_SESSIONID=<session-id>
```

## Security notes

- Never commit `.env`, `trakt-token.json`, IMDb cookies, passwords, API tokens, or private state.
- Keep scheduled personal execution in a private Runner repository so Actions logs are not public.
- Prefer environment variables/`.env` over long-lived secrets in `config.yaml`.
- The interactive configuration TUI masks secret fields and writes `config.yaml` with owner-only permissions (`0600`).
- The application does not require the Trakt token architecture to be replaced by static access tokens; refresh-token rotation is handled automatically.

## License

See [`LICENSE`](LICENSE).
