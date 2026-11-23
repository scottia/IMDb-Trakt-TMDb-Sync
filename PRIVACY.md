# IMDb-Trakt-TMDb-Sync Privacy Policy

**Effective date:** November 22, 2026  
**Publisher:** Psycotix  
**Application:** IMDb-Trakt-TMDb-Sync (IMDb Traky Sync)

## Scope

This Privacy Policy applies to the IMDb-Trakt-TMDb-Sync application published by Psycotix.

IMDb-Trakt-TMDb-Sync is an open-source utility that synchronizes selected IMDb account data to independently configured Trakt and TMDb accounts. It may be run locally, in a container, or through a private GitHub Actions Runner controlled by the user.

## Information the application accesses

Depending on the features enabled by the user, IMDb-Trakt-TMDb-Sync may access:

- a public IMDb ratings-page URL or IMDb profile identifier;
- IMDb title identifiers, personal ratings, rating dates, title types, and titles;
- an IMDb Watchlist or custom lists when authenticated browser access is configured;
- Trakt ratings, rating-derived history, Watchlist items, custom lists, and account identifiers;
- TMDb ratings, Watchlist items, and title identifiers;
- application configuration and destination settings;
- OAuth tokens, API credentials, browser cookies, and authorization files supplied by the user;
- locally stored synchronization state, mapping caches, and ratings snapshots;
- application logs, workflow logs, and diagnostic information.

The application processes this information on the user's device or within a user-controlled execution environment, such as a private GitHub Actions Runner.

## Public IMDb ratings source

The recommended IMDb ratings source uses a public IMDb ratings-page URL or profile identifier.

This mode does not require the user's Amazon password, an AWS account, or an authenticated IMDb browser session. It reads only rating information that the IMDb account owner has made publicly accessible.

A public ratings profile may reveal the user's IMDb profile identifier, rated titles, personal rating values, and rating dates to anyone who can access that public IMDb page.

The public ratings source does not provide private IMDb Watchlist or custom-list information.

## Optional IMDb authentication

Users who enable authenticated IMDb features may supply IMDb or Amazon-backed session cookies, credential information, or a persisted browser cookie jar.

These values should be treated as sensitive credentials. They are used only to authenticate directly with IMDb or its associated authentication services.

Psycotix does not operate a central service that collects or stores users' Amazon passwords, IMDb cookies, browser sessions, or IMDb account data.

## Trakt authorization

When Trakt synchronization is enabled, the application uses Trakt OAuth authorization to access the user's Trakt account.

Depending on the configured options, the application may read or update:

- ratings;
- rating-derived watch history;
- Watchlist items;
- custom lists;
- related account identifiers.

Trakt OAuth tokens and application credentials are stored only in locations selected or controlled by the user, such as local files, environment variables, or private GitHub Actions secrets.

Users may revoke the application's access through their Trakt account settings.

## TMDb authorization

When TMDb synchronization is enabled, the application may use a TMDb API access token and user session ID to read or update supported TMDb account information.

This may include ratings, Watchlist items, and identifiers needed to map IMDb titles to TMDb titles.

TMDb credentials and session information remain in storage locations controlled by the user.

## Network requests and third-party services

IMDb-Trakt-TMDb-Sync communicates directly with third-party services required for enabled functionality.

These services may include:

- IMDb and associated Amazon authentication infrastructure;
- Trakt;
- TMDb;
- GitHub and GitHub Actions when the user chooses the private Runner deployment model.

Requests may include title identifiers, ratings, rating dates, list data, OAuth tokens, API credentials, and other information necessary to perform the requested synchronization.

Third-party services necessarily receive technical connection information such as the user's IP address, request headers, and request timing. Those services process information according to their own privacy policies and terms.

## Private GitHub Actions Runner

Users may operate the application through a separate private GitHub repository containing:

- configuration;
- encrypted Actions secrets;
- OAuth tokens;
- synchronization state;
- ratings snapshots;
- identifier-mapping caches;
- workflow logs.

GitHub stores and processes that information according to GitHub's own privacy policy and service terms.

Users should never place credentials, tokens, cookies, or private synchronization state in a public repository.

## Synchronization state

The application may retain state files such as:

- the last successfully processed IMDb ratings snapshot;
- synchronization timestamps and counts;
- IMDb-to-TMDb identifier mappings;
- pending or completed destination state.

This information is used to calculate changes between runs and reduce unnecessary third-party requests.

State remains in storage controlled by the user and is not automatically transmitted to Psycotix.

## Diagnostics and logs

The application and its Runner workflows may create logs containing:

- synchronization counts;
- IMDb, Trakt, or TMDb title identifiers;
- destination results;
- API response status information;
- retry and rate-limit information;
- errors and diagnostic details.

The application is designed not to intentionally print passwords, cookies, OAuth tokens, or API secrets. Users should nevertheless inspect logs before sharing them publicly.

Logs are not automatically transmitted to Psycotix. A user may choose to provide logs when requesting support.

## Telemetry, advertising, and sale of data

IMDb-Trakt-TMDb-Sync does not include Psycotix-operated advertising or developer-operated usage telemetry.

Psycotix does not sell or rent personal information processed by the application.

GitHub, IMDb, Amazon, Trakt, TMDb, internet service providers, and operating-system services may independently collect information under their own privacy policies.

## Data retention and user control

Application data remains in locations selected or controlled by the user.

Users may:

- remove the public IMDb ratings-page reference;
- make their IMDb ratings private;
- delete IMDb cookies or browser-session files;
- revoke Trakt OAuth access;
- remove TMDb authorization or session information;
- delete local or Runner configuration, logs, state, caches, and mapping files;
- delete associated GitHub Actions secrets;
- disable individual destinations or synchronization features.

Deleting the application does not necessarily remove files, private repositories, third-party account data, or storage locations created or configured separately by the user.

## Children

IMDb-Trakt-TMDb-Sync is a general-purpose media-account synchronization utility and is not directed specifically to children.

Psycotix does not knowingly use the application to collect personal information from children.

## Changes to this policy

This policy may be updated when the application's supported services, authentication methods, or data practices change.

The current version will be published in the IMDb-Trakt-TMDb-Sync repository with its effective date.

## Contact

Privacy or support questions may be submitted through the IMDb-Trakt-TMDb-Sync GitHub repository:

https://github.com/scottia/IMDb-Trakt-TMDb-Sync/issues
