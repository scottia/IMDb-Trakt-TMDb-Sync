package imdb

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-rod/rod/lib/proto"
	"github.com/scottia/IMDb-Trakt-TMDb-Sync/internal/config"
)

const (
	imdbCookieJarEnv     = "ITS_IMDB_COOKIEJARFILE"
	imdbCookieJarVersion = 1
)

var imdbAuthCookieNames = map[string]struct{}{
	"at-main":         {},
	"sess-at-main":    {},
	"session-id":      {},
	"session-id-time": {},
	"session-token":   {},
	"ubid-main":       {},
	"x-main":          {},
}

type imdbCookieJar struct {
	Version int                    `json:"version"`
	Cookies []*proto.NetworkCookie `json:"cookies"`
}

func (c *client) seedBrowserAuthCookies() error {
	path := strings.TrimSpace(os.Getenv(imdbCookieJarEnv))
	if path != "" {
		cookies, err := loadIMDBCookieJar(path)
		if err == nil && len(cookies) > 0 {
			if err = c.browser.SetCookies(proto.CookiesToParams(cookies)); err == nil {
				c.logger.Info("seeded imdb browser authentication from persisted cookie jar", "cookies", len(cookies), "names", imdbCookieNames(cookies))
				return nil
			}
			c.logger.Warn("failed setting persisted imdb cookie jar; falling back to legacy at-main bootstrap", "error", err)
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			c.logger.Warn("failed loading persisted imdb cookie jar; falling back to legacy at-main bootstrap", "error", err)
		}
	}

	if err := setBrowserCookies(c.browser, *c.CookieAtMain); err != nil {
		return err
	}
	c.logger.Info("seeded imdb browser authentication from legacy at-main cookie")
	return nil
}

func (c *client) persistBrowserAuthCookies() error {
	if *c.Auth != config.IMDbAuthMethodCookies {
		return nil
	}
	path := strings.TrimSpace(os.Getenv(imdbCookieJarEnv))
	if path == "" {
		return nil
	}

	cookies, err := c.browser.GetCookies()
	if err != nil {
		return fmt.Errorf("failure retrieving browser cookies: %w", err)
	}
	cookies = filterIMDBAuthCookies(cookies)
	if len(cookies) == 0 {
		c.logger.Warn("no allowlisted imdb authentication cookies were available to persist")
		return nil
	}
	if err = writeIMDBCookieJar(path, cookies); err != nil {
		return fmt.Errorf("failure writing imdb cookie jar: %w", err)
	}
	c.logger.Info("persisted imdb authentication cookie jar", "cookies", len(cookies), "names", imdbCookieNames(cookies))
	return nil
}

func loadIMDBCookieJar(path string) ([]*proto.NetworkCookie, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var jar imdbCookieJar
	if err = json.Unmarshal(data, &jar); err != nil {
		return nil, fmt.Errorf("failure decoding imdb cookie jar: %w", err)
	}
	if jar.Version != imdbCookieJarVersion {
		return nil, fmt.Errorf("unsupported imdb cookie jar version %d", jar.Version)
	}
	cookies := filterIMDBAuthCookies(jar.Cookies)
	if len(cookies) == 0 {
		return nil, fmt.Errorf("imdb cookie jar contains no allowlisted authentication cookies")
	}
	return cookies, nil
}

func writeIMDBCookieJar(path string, cookies []*proto.NetworkCookie) error {
	jar := imdbCookieJar{
		Version: imdbCookieJarVersion,
		Cookies: filterIMDBAuthCookies(cookies),
	}
	data, err := json.MarshalIndent(jar, "", "  ")
	if err != nil {
		return fmt.Errorf("failure encoding imdb cookie jar: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(path)
	if err = os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failure creating imdb cookie jar directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".imdb-cookie-jar-*")
	if err != nil {
		return fmt.Errorf("failure creating temporary imdb cookie jar: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err = tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("failure protecting temporary imdb cookie jar: %w", err)
	}
	if _, err = tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("failure writing temporary imdb cookie jar: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("failure closing temporary imdb cookie jar: %w", err)
	}
	if err = os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failure replacing imdb cookie jar: %w", err)
	}
	return os.Chmod(path, 0o600)
}

func filterIMDBAuthCookies(cookies []*proto.NetworkCookie) []*proto.NetworkCookie {
	filtered := make([]*proto.NetworkCookie, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie == nil || !isIMDBDomain(cookie.Domain) {
			continue
		}
		if _, ok := imdbAuthCookieNames[cookie.Name]; !ok {
			continue
		}
		filtered = append(filtered, cookie)
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Name != filtered[j].Name {
			return filtered[i].Name < filtered[j].Name
		}
		if filtered[i].Domain != filtered[j].Domain {
			return filtered[i].Domain < filtered[j].Domain
		}
		return filtered[i].Path < filtered[j].Path
	})
	return filtered
}

func isIMDBDomain(domain string) bool {
	domain = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(domain)), ".")
	return domain == "imdb.com" || strings.HasSuffix(domain, ".imdb.com")
}

func imdbCookieNames(cookies []*proto.NetworkCookie) []string {
	names := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie != nil {
			names = append(names, cookie.Name)
		}
	}
	return names
}
