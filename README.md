[![sync](https://github.com/scottia/imdb-trakt-sync/actions/workflows/sync.yaml/badge.svg?branch=main)](https://github.com/scottia/imdb-trakt-sync/actions/workflows/sync.yaml?query=branch%3Amain)
[![quality](https://github.com/scottia/imdb-trakt-sync/actions/workflows/quality.yaml/badge.svg?branch=main)](https://github.com/scottia/imdb-trakt-sync/actions/workflows/quality.yaml?query=branch%3Amain)
> **Note for forks:** The badges above are hardcoded to this repository. After forking, update the two badge URLs at the top of this file, replacing `scottia/imdb-trakt-sync` with your own `{username}/{repo-name}`.

# imdb-trakt-sync

<img src="./assets/logo.png" alt="IMDb to Trakt and TMDb"/>

Command-line application for one-way synchronization from [IMDb](https://www.imdb.com/) to two destinations:

- **[Trakt](https://trakt.tv/dashboard):** watchlist, lists, ratings, and rating-derived history.
- **[TMDb](https://www.themoviedb.org):** ratings only, using the TMDb API when the optional TMDb destination is enabled.

IMDb is the source of truth. Changes made directly on Trakt or TMDb are not written back to IMDb. Destination removals depend on `SYNC_MODE`.

> [!IMPORTANT]
> Trakt API app creation now requires Trakt VIP access. See the upstream [VIP detail / issue #107](https://github.com/cecobask/imdb-trakt-sync/issues/107).

# Configuration

<table>
    <tr>
        <th>FIELD NAME</th>
        <th>FIELD TYPE</th>
        <th>DEFAULT VALUE</th>
        <th>ALLOWED VALUES</th>
        <th>DESCRIPTION</th>
    </tr>p
    <tr>
        <td>IMDB_AUTH</td>
        <td>variable</td>
        <td>cookies</td>
        <td>
            credentials<br />
            cookies<br />
            none
        </td>
        <td>
            Authentication method to be used for IMDb:<br />
            <code>credentials</code> =&gt; IMDB_EMAIL + IMDB_PASSWORD fields required<br />
            <code>cookies</code> =&gt; IMDB_COOKIEATMAIN field required<br />
            <code>none</code> =&gt; IMDB_LISTS field required
        </td>
    </tr>
    <tr>
        <td>IMDB_EMAIL</td>
        <td>secret</td>
        <td>-</td>
        <td>-</td>
        <td>IMDb account email address. Only required when IMDB_AUTH =&gt; <code>credentials</code></td>
    </tr>
    <tr>
        <td>IMDB_PASSWORD</td>
        <td>secret</td>
        <td>-</td>
        <td>-</td>
        <td>IMDb account password. Only required when IMDB_AUTH =&gt; <code>credentials</code></td>
    </tr>
    <tr>
        <td>IMDB_COOKIEATMAIN</td>
        <td>secret</td>
        <td>-</td>
        <td>-</td>
        <td>
            Cookie value only required when IMDB_AUTH =&gt; <code>cookies</code>. Get the following cookie information from
            your browser:<br />
            <code>name: at-main | domain: .imdb.com</code>
        </td>
    </tr>
    <tr>
        <td>IMDB_LISTS</td>
        <td>variable</td>
        <td>-</td>
        <td>-</td>
        <td>
            Array of IMDb list IDs that you would like synced to Trakt. If this array is not specified or empty, all
            IMDb lists on your account will be synced to Trakt. In order to get the ID of an IMDb list, open it from a
            browser - the ID is in the URL with format <code>ls#########</code>. If provided as a GitHub secret or
            environment variable, define its values as a comma-separated list. Keep in mind the <a
                href="https://forums.trakt.tv/t/personal-list-updates/10170#limits-3">Trakt list limits</a>.
        </td>
    </tr>
    <tr>
        <td>IMDB_IGNOREDLISTS</td>
        <td>variable</td>
        <td>-</td>
        <td>-</td>
        <td>
            Array of IMDb list IDs that you do <b>NOT</b> want synced to Trakt. This is useful if you would like to
            sync all your lists, but ignore some. In order to get the ID of an IMDb list, open it from a browser - the
            ID is in the URL with format <code>ls#########</code>. If provided as a GitHub secret or environment variable,
            define its values as a comma-separated list.
        </td>
    </tr>
    <tr>
        <td>IMDB_TRACE</td>
        <td>variable</td>
        <td>false</td>
        <td>
            true<br />
            false
        </td>
        <td>Print tracing logs related to browser activity. Useful for debugging.</td>
    </tr>
    <tr>
        <td>IMDB_HEADLESS</td>
        <td>variable</td>
        <td>true</td>
        <td>
            true<br />
            false
        </td>
        <td>Whether to run the IMDb browser in headless mode. Set to false only when running locally and you need to see the browser.</td>
    </tr>
    <tr>
        <td>IMDB_BROWSERPATH</td>
        <td>variable</td>
        <td>-</td>
        <td>-</td>
        <td>
            Optional path to a preferred browser executable. If empty, the application attempts to locate a supported browser.
        </td>
    </tr>
    <tr>
        <td>SYNC_MODE</td>
        <td>variable</td>
        <td>dry-run</td>
        <td>
            full<br />
            add-only<br />
            dry-run
        </td>
        <td>
            Sync mode used by the destinations:<br />
            <code>full</code> =&gt; add/update destination data and remove destination data no longer present on IMDb where supported<br />
            <code>add-only</code> =&gt; add/update without destination removals<br />
            <code>dry-run</code> =&gt; report planned changes without writing them
        </td>
    </tr>
    <tr>
        <td>SYNC_HISTORY</td>
        <td>variable</td>
        <td>false</td>
        <td>
            true<br />
            false
        </td>
        <td>Whether to sync rating-derived history to Trakt. When IMDB_AUTH =&gt; <code>none</code>, history sync is skipped.</td>
    </tr>
    <tr>
        <td>SYNC_RATINGS</td>
        <td>variable</td>
        <td>true</td>
        <td>
            true<br />
            false
        </td>
        <td>Whether to sync ratings. When false, both Trakt ratings sync and the optional TMDb ratings destination are skipped.</td>
    </tr>
    <tr>
        <td>SYNC_WATCHLIST</td>
        <td>variable</td>
        <td>true</td>
        <td>
            true<br />
            false
        </td>
        <td>Whether to sync the IMDb watchlist to Trakt. When IMDB_AUTH =&gt; <code>none</code>, watchlist sync is skipped.</td>
    </tr>
    <tr>
        <td>SYNC_LISTS</td>
        <td>variable</td>
        <td>true</td>
        <td>
            true<br />
            false
        </td>
        <td>Whether to sync IMDb lists to Trakt.</td>
    </tr>
    <tr>
        <td>SYNC_TIMEOUT</td>
        <td>variable</td>
        <td>15m</td>
        <td>-</td>
        <td>
            Maximum duration to run the syncer. Users with large libraries might have to increase the timeout value.
            Valid time units are: ns, us (or µs), ms, s, m, h.
        </td>
    </tr>
    <tr>
        <td>TRAKT_CLIENTID</td>
        <td>secret</td>
        <td>-</td>
        <td>-</td>
        <td>Trakt app client ID.</td>
    </tr>
    <tr>
        <td>TRAKT_CLIENTSECRET</td>
        <td>secret</td>
        <td>-</td>
        <td>-</td>
        <td>Trakt app client secret.</td>
    </tr>
    <tr>
        <td>TRAKT_TOKENFILE</td>
        <td>variable</td>
        <td>trakt-token.json</td>
        <td>-</td>
        <td>
            Path used to store Trakt access/refresh tokens. Created automatically after first authorization and kept up to date afterwards.
        </td>
    </tr>
    <tr>
        <td>TMDB_ENABLED</td>
        <td>variable</td>
        <td>false</td>
        <td>
            true<br />
            false
        </td>
        <td>
            Enables the optional TMDb ratings destination. Requires authenticated IMDb ratings access, <code>SYNC_RATINGS=true</code>,
            <code>TMDB_READACCESSTOKEN</code>, and <code>TMDB_SESSIONID</code>.
        </td>
    </tr>
    <tr>
        <td>TMDB_READACCESSTOKEN</td>
        <td>secret</td>
        <td>-</td>
        <td>-</td>
        <td>TMDb API Read Access Token used to authenticate API requests. Required when <code>TMDB_ENABLED=true</code>.</td>
    </tr>
    <tr>
        <td>TMDB_SESSIONID</td>
        <td>secret</td>
        <td>-</td>
        <td>-</td>
        <td>Authenticated TMDb session ID for the account receiving rating changes. Required when <code>TMDB_ENABLED=true</code>.</td>
    </tr>
</table>

## Trakt authentication

Trakt no longer supports signing in with an email and password from third-party applications. The application authorizes using Trakt's [device code flow](https://docs.trakt.tv/reference/authentication#device-code-flow) instead.

The first time the application runs without an existing token file, it prints a verification URL and a code. Open the URL in any browser, sign in however you normally would, and enter the code. The application polls in the background and, once approved, saves the resulting tokens to `TRAKT_TOKENFILE`. When the access token expires, the refresh token is used to generate a fresh token pair.

## TMDb ratings destination

TMDb support is optional and disabled by default. It synchronizes **ratings only**; TMDb watchlists, lists, and history are not modified.

The TMDb implementation uses the API to map IMDb title IDs to TMDb targets and write ratings to the authenticated TMDb account. To enable it, provide:

- `TMDB_ENABLED=true`
- `TMDB_READACCESSTOKEN` - your TMDb API Read Access Token
- `TMDB_SESSIONID` - a valid authenticated TMDb session ID for the same account

Treat both credential values as secrets and never commit them to the repository.

`SYNC_MODE=dry-run` performs no TMDb writes. `add-only` does not remove TMDb ratings. `full` may remove destination ratings that are no longer present in the IMDb source when reconciliation identifies them.

When persistent sync state is in bootstrap mode, the current IMDb ratings snapshot becomes the baseline after successful destinations; TMDb rating writes are skipped for that bootstrap snapshot and subsequent runs process changes from the baseline.

# Usage

The application can run automatically on a custom schedule (_default: once every 12 hours_) using **GitHub Actions**, in a container, or locally. Workflow schedules can be changed in [.github/workflows/sync.yaml](.github/workflows/sync.yaml).

Configure the application for your environment using the [Configuration](#configuration) section before running it.

## Run the application using GitHub Actions

1. [Fork this repository](https://github.com/scottia/imdb-trakt-sync/fork) to your account.
2. Create a [Trakt App](https://trakt.tv/oauth/applications). Use **urn:ietf:wg:oauth:2.0:oob** as the redirect URI.
3. Configure the application in your fork: `Settings` > `Secrets and variables` > `Actions`.
   - The current workflow reads its configuration from **repository secrets**.
   - Create the IMDb/Trakt secrets referenced in [.github/workflows/sync.yaml](.github/workflows/sync.yaml) for the features you use.
   - Create a `GH_PAT` repository secret so rotated Trakt tokens can be persisted (see [Creating the GH_PAT secret](#creating-the-gh_pat-secret)).
   - To enable TMDb ratings, create these additional repository secrets:
     - `TMDB_ENABLED` = `true`
     - `TMDB_READ_ACCESS_TOKEN` = your TMDb API Read Access Token
     - `TMDB_SESSION_ID` = your authenticated TMDb session ID
4. Allow GitHub Actions on your fork: `Settings` > `Actions` > `General` > `Allow all actions and reusable workflows`.
5. Enable the **sync** workflow: `Actions` > `Workflows` > `sync` > `Enable workflow`.
6. Run the **sync** workflow manually: `Actions` > `Workflows` > `sync` > `Run workflow`.
7. From then on, GitHub Actions automatically triggers the **sync** workflow based on your schedule.

The workflow maps the TMDb repository secrets to the application environment as follows:

- `TMDB_ENABLED` -> `ITS_TMDB_ENABLED`
- `TMDB_READ_ACCESS_TOKEN` -> `ITS_TMDB_READACCESSTOKEN`
- `TMDB_SESSION_ID` -> `ITS_TMDB_SESSIONID`

### Creating the GH_PAT secret

The **sync** workflow needs to overwrite the `TRAKT_TOKEN` repository secret whenever a new Trakt token pair is generated. The default `GITHUB_TOKEN` cannot modify repository secrets, so a personal access token with that permission is required.

A fine-grained token scoped to this repository and this permission is recommended:

1. Go to [Fine-grained tokens](https://github.com/settings/personal-access-tokens/new).
2. Give the token a name (for example, `imdb-trakt-sync`) and an expiration.
3. Under `Repository access`, choose `Only select repositories` and select your `imdb-trakt-sync` fork.
4. Click `Add permissions` and select **Secrets** with `Read and write` access.
5. Generate the token and copy its value.
6. Create a repository secret named `GH_PAT` in your fork and set it to that value.

## Run the application in a Docker container

1. Install [Docker](https://www.docker.com/get-started).
2. Clone the repository: `git clone git@github.com:scottia/imdb-trakt-sync.git`.
3. Create a [Trakt App](https://trakt.tv/oauth/applications). Use **urn:ietf:wg:oauth:2.0:oob** as the redirect URI.
4. Configure the application:
   - Create a `.env` file using [.env.example](.env.example) as the starting point.
   - Populate it with your own values. Environment keys use the `ITS_` prefix.
   - If enabling TMDb ratings, also add:
     - `ITS_TMDB_ENABLED=true`
     - `ITS_TMDB_READACCESSTOKEN=<your TMDb API Read Access Token>`
     - `ITS_TMDB_SESSIONID=<your TMDb session ID>`
5. Open a terminal in the repository folder and:
   - Build a Docker image: `make package`.
   - Run the sync workflow in a Docker container: `make sync-container`.
   - On the first Trakt authorization, open the verification URL printed by the application and approve the displayed code. The resulting Trakt token is persisted to `trakt-token.json` on the host through the mounted volume.

## Run the application locally

1. Install [Git](https://git-scm.com/downloads) and [Go](https://go.dev/doc/install).
2. Clone the repository: `git clone git@github.com:scottia/imdb-trakt-sync.git`.
3. Create a [Trakt App](https://trakt.tv/oauth/applications). Use **urn:ietf:wg:oauth:2.0:oob** as the redirect URI.
4. Configure and run the application:
   - Build: `make build`.
   - Configure: `make configure`.
   - Run: `make sync`.
   - On the first Trakt authorization, approve the printed verification URL/code. The resulting token is saved to `trakt-token.json`.
   - If enabling TMDb through environment variables, set `ITS_TMDB_ENABLED`, `ITS_TMDB_READACCESSTOKEN`, and `ITS_TMDB_SESSIONID` before running.
