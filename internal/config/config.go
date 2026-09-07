package config

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

type IMDb struct {
	Auth         *IMDbAuthMethod `koanf:"AUTH"`
	Email        *string         `koanf:"EMAIL"`
	Password     *string         `koanf:"PASSWORD"`
	CookieAtMain *string         `koanf:"COOKIEATMAIN"`
	Lists        *[]string       `koanf:"LISTS"`
	IgnoredLists *[]string       `koanf:"IGNOREDLISTS"`
	Trace        *bool           `koanf:"TRACE"`
	Headless     *bool           `koanf:"HEADLESS"`
	BrowserPath  *string         `koanf:"BROWSERPATH"`
}

type Sync struct {
	Mode      *SyncMode      `koanf:"MODE"`
	History   *bool          `koanf:"HISTORY"`
	Ratings   *bool          `koanf:"RATINGS"`
	Watchlist *bool          `koanf:"WATCHLIST"`
	Lists     *bool          `koanf:"LISTS"`
	Timeout   *time.Duration `koanf:"TIMEOUT"`
}

type Trakt struct {
	Enabled      *bool   `koanf:"ENABLED"`
	ClientID     *string `koanf:"CLIENTID"`
	ClientSecret *string `koanf:"CLIENTSECRET"`
	TokenFile    *string `koanf:"TOKENFILE"`
	Sync         Sync    `koanf:"SYNC"`

	SyncMode      *SyncMode      `koanf:"-"`
	SyncHistory   *bool          `koanf:"-"`
	SyncRatings   *bool          `koanf:"-"`
	SyncWatchlist *bool          `koanf:"-"`
	SyncLists     *bool          `koanf:"-"`
	SyncTimeout   *time.Duration `koanf:"-"`
}

type TMDb struct {
	Enabled         *bool   `koanf:"ENABLED"`
	ReadAccessToken *string `koanf:"READACCESSTOKEN"`
	SessionID       *string `koanf:"SESSIONID"`
	Sync            Sync    `koanf:"SYNC"`

	SyncMode      *SyncMode      `koanf:"-"`
	SyncHistory   *bool          `koanf:"-"`
	SyncRatings   *bool          `koanf:"-"`
	SyncWatchlist *bool          `koanf:"-"`
	SyncLists     *bool          `koanf:"-"`
	SyncTimeout   *time.Duration `koanf:"-"`
}

type Config struct {
	koanf *koanf.Koanf
	IMDb  IMDb  `koanf:"IMDB"`
	Trakt Trakt `koanf:"TRAKT"`
	TMDb  TMDb  `koanf:"TMDB"`
	Sync  Sync  `koanf:"SYNC"`
}

const (
	delimiter = "_"
	prefix    = "ITS" + delimiter

	IMDbAuthMethodCredentials IMDbAuthMethod = "credentials"
	IMDbAuthMethodCookies     IMDbAuthMethod = "cookies"
	IMDbAuthMethodNone        IMDbAuthMethod = "none"
	SyncModeAddOnly           SyncMode       = "add-only"
	SyncModeDryRun            SyncMode       = "dry-run"
	SyncModeFull              SyncMode       = "full"
	SyncTimeoutDefault                       = time.Minute * 30
)

type IMDbAuthMethod string

type SyncMode string

func New(path string, includeEnv bool) (*Config, error) {
	k := koanf.New(delimiter)
	fileProvider := file.Provider(path)
	if err := k.Load(fileProvider, yaml.Parser()); err != nil {
		return nil, fmt.Errorf("error loading config from yaml file: %w", err)
	}
	if includeEnv {
		envProvider := env.ProviderWithValue(prefix, delimiter, environmentVariableModifier)
		if err := k.Load(envProvider, nil); err != nil {
			return nil, fmt.Errorf("error loading config from environment variables: %w", err)
		}
	}
	conf := Config{koanf: k}
	if err := k.Unmarshal("", &conf); err != nil {
		return nil, fmt.Errorf("error unmarshalling config: %w", err)
	}
	conf.applyDefaults()
	return &conf, nil
}

func NewFromMap(data map[string]interface{}) (*Config, error) {
	k := koanf.New(delimiter)
	cmProvider := confmap.Provider(data, delimiter)
	if err := k.Load(cmProvider, nil); err != nil {
		return nil, err
	}
	conf := Config{koanf: k}
	if err := k.Unmarshal("", &conf); err != nil {
		return nil, fmt.Errorf("error unmarshalling config: %w", err)
	}
	conf.applyDefaults()
	return &conf, nil
}

func (c *Config) Validate() error {
	if c.IMDb.Auth == nil || *c.IMDb.Auth == "" {
		return fmt.Errorf("field 'IMDB_AUTH' is required")
	}
	switch *c.IMDb.Auth {
	case IMDbAuthMethodCredentials:
		if isNilOrEmpty(c.IMDb.Email) {
			return fmt.Errorf("field 'IMDB_EMAIL' is required")
		}
		if isNilOrEmpty(c.IMDb.Password) {
			return fmt.Errorf("field 'IMDB_PASSWORD' is required")
		}
	case IMDbAuthMethodCookies:
		if isNilOrEmpty(c.IMDb.CookieAtMain) {
			return fmt.Errorf("field 'IMDB_COOKIEATMAIN' is required")
		}
	case IMDbAuthMethodNone:
	default:
		return fmt.Errorf("field 'IMDB_AUTH' must be one of: %s", strings.Join(validIMDbAuthMethods(), ", "))
	}
	if err := c.validateListIdentifiers(*c.IMDb.Lists); err != nil {
		return fmt.Errorf("field 'IMDB_LISTS' is invalid: %w", err)
	}
	if err := c.validateListIdentifiers(*c.IMDb.IgnoredLists); err != nil {
		return fmt.Errorf("field 'IMDB_IGNOREDLISTS' is invalid: %w", err)
	}
	if !*c.Trakt.Enabled && !*c.TMDb.Enabled {
		return fmt.Errorf("at least one destination must be enabled: 'TRAKT_ENABLED' or 'TMDB_ENABLED'")
	}
	if err := validateSyncMode("TRAKT_SYNC_MODE", c.Trakt.Sync.Mode); err != nil {
		return err
	}
	if err := validateSyncTimeout("TRAKT_SYNC_TIMEOUT", c.Trakt.Sync.Timeout); err != nil {
		return err
	}
	if *c.Trakt.Enabled {
		if isNilOrEmpty(c.Trakt.ClientID) {
			return fmt.Errorf("field 'TRAKT_CLIENTID' is required when 'TRAKT_ENABLED' is true")
		}
		if isNilOrEmpty(c.Trakt.ClientSecret) {
			return fmt.Errorf("field 'TRAKT_CLIENTSECRET' is required when 'TRAKT_ENABLED' is true")
		}
	}
	if err := validateSyncMode("TMDB_SYNC_MODE", c.TMDb.Sync.Mode); err != nil {
		return err
	}
	if err := validateSyncTimeout("TMDB_SYNC_TIMEOUT", c.TMDb.Sync.Timeout); err != nil {
		return err
	}
	if *c.TMDb.Enabled {
		if *c.TMDb.Sync.History {
			return fmt.Errorf("field 'TMDB_SYNC_HISTORY' cannot be enabled: TMDb does not expose an account watch-history API")
		}
		if *c.TMDb.Sync.Lists {
			return fmt.Errorf("field 'TMDB_SYNC_LISTS' cannot be enabled with the current TMDb authentication model: mixed custom-list writes require TMDb v4 user-access-token support")
		}
		if *c.TMDb.Sync.Ratings || *c.TMDb.Sync.Watchlist {
			if isNilOrEmpty(c.TMDb.ReadAccessToken) {
				return fmt.Errorf("field 'TMDB_READACCESSTOKEN' is required when a TMDb sync feature is enabled")
			}
			if isNilOrEmpty(c.TMDb.SessionID) {
				return fmt.Errorf("field 'TMDB_SESSIONID' is required when a TMDb sync feature is enabled")
			}
		}
	}
	return c.checkDummies()
}

func validateSyncMode(fieldName string, mode *SyncMode) error {
	if mode == nil || *mode == "" {
		return fmt.Errorf("field '%s' is required", fieldName)
	}
	if !slices.Contains(validSyncModes(), string(*mode)) {
		return fmt.Errorf("field '%s' must be one of: %s", fieldName, strings.Join(validSyncModes(), ", "))
	}
	return nil
}

func validateSyncTimeout(fieldName string, timeout *time.Duration) error {
	if timeout == nil || *timeout <= 0 {
		return fmt.Errorf("field '%s' must be greater than zero", fieldName)
	}
	return nil
}

func (c *Config) validateListIdentifiers(lids []string) error {
	re := regexp.MustCompile(`^ls[0-9]{9,10}$`)
	for _, id := range lids {
		if ok := re.MatchString(id); !ok {
			return fmt.Errorf("valid list id starts with ls and is followed by 9 or 10 digits, but got %s", id)
		}
	}
	return nil
}

func (c *Config) WriteFile(path string) error {
	data, err := c.koanf.Marshal(yaml.Parser())
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

func (c *Config) Flatten() map[string]interface{} {
	return c.koanf.All()
}

func (c *Config) checkDummies() error {
	for k, v := range c.koanf.All() {
		if value, ok := v.(string); ok {
			if match := slices.Contains(dummyValues(), value); match {
				return fmt.Errorf("field '%s' contains dummy value '%s'", k, value)
			}
			continue
		}
		if value, ok := v.([]interface{}); ok {
			for _, sliceElement := range value {
				if str, isStr := sliceElement.(string); isStr {
					if match := slices.Contains(dummyValues(), str); match {
						return fmt.Errorf("field '%s' contains dummy value '%s'", k, str)
					}
				}
			}
		}
	}
	return nil
}

func (c *Config) applyDefaults() {
	if c.IMDb.Auth == nil {
		c.IMDb.Auth = pointer(IMDbAuthMethodCookies)
	}
	if c.IMDb.Lists == nil {
		c.IMDb.Lists = pointer(make([]string, 0))
	}
	if c.IMDb.IgnoredLists == nil {
		c.IMDb.IgnoredLists = pointer(make([]string, 0))
	}
	if c.IMDb.Trace == nil {
		c.IMDb.Trace = pointer(false)
	}
	if c.IMDb.Headless == nil {
		c.IMDb.Headless = pointer(true)
	}
	if c.IMDb.BrowserPath == nil {
		c.IMDb.BrowserPath = pointer("")
	}

	if c.Trakt.Enabled == nil {
		c.Trakt.Enabled = pointer(true)
	}
	if c.Trakt.TokenFile == nil || *c.Trakt.TokenFile == "" {
		c.Trakt.TokenFile = pointer("trakt-token.json")
	}
	if c.Trakt.Sync.Mode == nil {
		c.Trakt.Sync.Mode = legacyOrDefault(c.Sync.Mode, SyncModeAddOnly)
	}
	if c.Trakt.Sync.History == nil {
		c.Trakt.Sync.History = legacyOrDefault(c.Sync.History, true)
	}
	if c.Trakt.Sync.Ratings == nil {
		c.Trakt.Sync.Ratings = legacyOrDefault(c.Sync.Ratings, true)
	}
	if c.Trakt.Sync.Watchlist == nil {
		c.Trakt.Sync.Watchlist = legacyOrDefault(c.Sync.Watchlist, true)
	}
	if c.Trakt.Sync.Lists == nil {
		c.Trakt.Sync.Lists = legacyOrDefault(c.Sync.Lists, true)
	}
	if c.Trakt.Sync.Timeout == nil {
		c.Trakt.Sync.Timeout = legacyOrDefault(c.Sync.Timeout, SyncTimeoutDefault)
	}

	if c.TMDb.Enabled == nil {
		c.TMDb.Enabled = pointer(true)
	}
	if c.TMDb.ReadAccessToken == nil {
		c.TMDb.ReadAccessToken = pointer("")
	}
	if c.TMDb.SessionID == nil {
		c.TMDb.SessionID = pointer("")
	}
	if c.TMDb.Sync.Mode == nil {
		c.TMDb.Sync.Mode = legacyOrDefault(c.Sync.Mode, SyncModeAddOnly)
	}
	if c.TMDb.Sync.History == nil {
		c.TMDb.Sync.History = pointer(false)
	}
	if c.TMDb.Sync.Ratings == nil {
		c.TMDb.Sync.Ratings = legacyOrDefault(c.Sync.Ratings, true)
	}
	if c.TMDb.Sync.Watchlist == nil {
		c.TMDb.Sync.Watchlist = pointer(true)
	}
	if c.TMDb.Sync.Lists == nil {
		c.TMDb.Sync.Lists = pointer(false)
	}
	if c.TMDb.Sync.Timeout == nil {
		c.TMDb.Sync.Timeout = legacyOrDefault(c.Sync.Timeout, SyncTimeoutDefault)
	}

	c.Trakt.SyncMode = c.Trakt.Sync.Mode
	c.Trakt.SyncHistory = c.Trakt.Sync.History
	c.Trakt.SyncRatings = c.Trakt.Sync.Ratings
	c.Trakt.SyncWatchlist = c.Trakt.Sync.Watchlist
	c.Trakt.SyncLists = c.Trakt.Sync.Lists
	c.Trakt.SyncTimeout = c.Trakt.Sync.Timeout
	c.TMDb.SyncMode = c.TMDb.Sync.Mode
	c.TMDb.SyncHistory = c.TMDb.Sync.History
	c.TMDb.SyncRatings = c.TMDb.Sync.Ratings
	c.TMDb.SyncWatchlist = c.TMDb.Sync.Watchlist
	c.TMDb.SyncLists = c.TMDb.Sync.Lists
	c.TMDb.SyncTimeout = c.TMDb.Sync.Timeout
}

func legacyOrDefault[T any](legacy *T, fallback T) *T {
	if legacy != nil {
		return pointer(*legacy)
	}
	return pointer(fallback)
}

func pointer[T any](v T) *T {
	return &v
}

func validSyncModes() []string {
	return []string{
		string(SyncModeFull),
		string(SyncModeAddOnly),
		string(SyncModeDryRun),
	}
}

func validIMDbAuthMethods() []string {
	return []string{
		string(IMDbAuthMethodCredentials),
		string(IMDbAuthMethodCookies),
		string(IMDbAuthMethodNone),
	}
}

func dummyValues() []string {
	return []string{
		"user@domain.com",
		"password123",
		"zAta|RHiA67JIrBDPaswIym3GyrTlEuQH-u9yrKP3BUNCHgVyE4oNtUzBYVKlhjjzBiM_Z-GSVnH9rKW3Hf7LdbejovoF6SI4ZmgJcTIUXoA4NVcH1Qahwm0KYCyz95o1gsgby-uQwdU6CoS6MFTnjMkLe1puNiv4uFkvo8mOQulJJeutzYedxiUd0ns9w1X_WeVXPTZWjwisPZMw3EOR6-q9xR4kCEWRW7CmWxU1AEDQbT8ns_AJJD34w1nIQUkuLgBQrvJI_pY",
		"301-0710501-5367639",
		"ls000000000",
		"ls111111111",
		"ls222222222",
		"ls333333333",
		"828832482dea6fffa4453f849fe873de8be54791b9acc01f6923098d0a62972d",
		"bdf9bab88c17f3710a6394607e96cd3a21dee6e5ea0e0236e9ed06e425ed8b6f",
	}
}

func environmentVariableModifier(key string, value string) (string, any) {
	key = strings.TrimPrefix(key, prefix)
	if value == "" {
		return key, nil
	}
	if slices.Contains(sliceFields(), key) && strings.Contains(value, ",") {
		return key, strings.Split(value, ",")
	}
	return key, value
}

func sliceFields() []string {
	return []string{
		"IMDB_LISTS",
		"IMDB_IGNOREDLISTS",
	}
}

func isNilOrEmpty(value *string) bool {
	return value == nil || *value == ""
}
