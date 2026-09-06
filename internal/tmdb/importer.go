package tmdb

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	appconfig "github.com/cecobask/imdb-trakt-sync/internal/config"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

const (
	baseURL           = "https://www.themoviedb.org"
	importURL         = baseURL + "/settings/import-list"
	cookieDomain      = ".themoviedb.org"
	selectorFileInput = "input[type='file']"
)

type BrowserOptions struct {
	BrowserPath string
	Headless    bool
	Trace       bool
}

// ImportRatings submits the original IMDb ratings CSV to TMDb's native
// Settings -> Import List workflow. It intentionally performs no TMDb-ID
// resolution and no OMDb/API fallback.
func ImportRatings(
	ctx context.Context,
	conf *appconfig.TMDb,
	browserOptions BrowserOptions,
	logger *slog.Logger,
	data []byte,
) error {
	if conf == nil || conf.Enabled == nil || !*conf.Enabled {
		return nil
	}
	if conf.Cookie == nil || strings.TrimSpace(*conf.Cookie) == "" {
		return fmt.Errorf("tmdb cookie must not be empty")
	}
	if len(data) == 0 {
		return fmt.Errorf("imdb ratings csv must not be empty")
	}

	cookies, err := parseCookieHeader(*conf.Cookie)
	if err != nil {
		return fmt.Errorf("failure parsing tmdb cookie header: %w", err)
	}

	browser, err := launchBrowser(ctx, browserOptions)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := browser.Close(); closeErr != nil {
			logger.Warn("failure closing tmdb browser", "error", closeErr)
		}
	}()

	if err := browser.SetCookies(cookies); err != nil {
		return fmt.Errorf("failure setting tmdb browser cookies: %w", err)
	}

	page, err := browser.Page(proto.TargetCreateTarget{URL: importURL})
	if err != nil {
		return fmt.Errorf("failure opening tmdb import page: %w", err)
	}
	if err := page.WaitLoad(); err != nil {
		return fmt.Errorf("failure waiting for tmdb import page: %w", err)
	}

	info, err := page.Info()
	if err != nil {
		return fmt.Errorf("failure reading tmdb import page info: %w", err)
	}
	if strings.Contains(strings.ToLower(info.URL), "/login") {
		return fmt.Errorf("tmdb authentication failed: import page redirected to login")
	}

	fileInput, err := page.Timeout(20 * time.Second).Element(selectorFileInput)
	if err != nil {
		return fmt.Errorf(
			"failure finding tmdb import file input; tmdb authentication may have expired or the import page changed: %w",
			err,
		)
	}

	tempDir, err := os.MkdirTemp("", "imdb-tmdb-import-*")
	if err != nil {
		return fmt.Errorf("failure creating temporary tmdb import directory: %w", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(tempDir); removeErr != nil {
			logger.Warn("failure removing temporary tmdb import directory", "error", removeErr)
		}
	}()

	// Keep the same basename used by IMDb's ratings export. The bytes are written
	// unchanged; TMDb receives the original IMDb CSV rather than a reconstructed file.
	csvPath := filepath.Join(tempDir, "ratings.csv")
	if err := os.WriteFile(csvPath, data, 0o600); err != nil {
		return fmt.Errorf("failure writing temporary imdb ratings csv: %w", err)
	}

	if err := fileInput.SetFiles([]string{csvPath}); err != nil {
		return fmt.Errorf("failure attaching imdb ratings csv to tmdb import form: %w", err)
	}

	submitButton, err := findImportButton(page)
	if err != nil {
		return err
	}

	waitIdle := page.Timeout(30*time.Second).WaitRequestIdle(
		750*time.Millisecond,
		nil,
		nil,
		nil,
	)
	if err := submitButton.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("failure submitting tmdb import form: %w", err)
	}
	waitIdle()

	// TMDb processes imports asynchronously. At this point we verify that the
	// submission was not immediately rejected; completion belongs to TMDb's
	// import-history queue rather than this GitHub Actions run.
	body, err := page.Element("body")
	if err != nil {
		return fmt.Errorf("failure reading tmdb response page: %w", err)
	}
	text, err := body.Text()
	if err != nil {
		return fmt.Errorf("failure reading tmdb response text: %w", err)
	}
	if reason := knownImportError(text); reason != "" {
		return fmt.Errorf("tmdb rejected imdb ratings import: %s", reason)
	}

	logger.Info("submitted imdb ratings csv to tmdb native importer", "bytes", len(data))
	return nil
}

func launchBrowser(ctx context.Context, opts BrowserOptions) (*rod.Browser, error) {
	l := launcher.New().
		Headless(opts.Headless).
		Set("disable-component-update").
		Set("disable-domain-reliability").
		Set("disable-print-preview").
		Set("disable-search-engine-choice-screen").
		Set("disable-setuid-sandbox").
		Set("hide-scrollbars").
		Set("mute-audio").
		Set("no-default-browser-check").
		Set("no-pings").
		Set("no-sandbox").
		Set("no-zygote")

	if strings.TrimSpace(opts.BrowserPath) != "" {
		l = l.Bin(opts.BrowserPath)
	}

	browserURL, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("failure launching tmdb browser: %w", err)
	}

	browser := rod.New().
		Context(ctx).
		ControlURL(browserURL).
		Trace(opts.Trace)

	if err := browser.Connect(); err != nil {
		return nil, fmt.Errorf("failure connecting to tmdb browser: %w", err)
	}
	return browser, nil
}

func parseCookieHeader(header string) ([]*proto.NetworkCookieParam, error) {
	header = strings.TrimSpace(header)
	if strings.HasPrefix(strings.ToLower(header), "cookie:") {
		header = strings.TrimSpace(header[len("cookie:"):])
	}

	var cookies []*proto.NetworkCookieParam
	for _, raw := range strings.Split(header, ";") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		parts := strings.SplitN(raw, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid cookie pair %q", raw)
		}
		name := strings.TrimSpace(parts[0])
		if name == "" {
			return nil, fmt.Errorf("cookie name must not be empty")
		}
		cookies = append(cookies, &proto.NetworkCookieParam{
			Name:   name,
			Value:  parts[1],
			Domain: cookieDomain,
			Path:   "/",
			Secure: true,
		})
	}
	if len(cookies) == 0 {
		return nil, fmt.Errorf("no cookies found")
	}
	return cookies, nil
}

func findImportButton(page *rod.Page) (*rod.Element, error) {
	// Prefer the visible button whose label is exactly "Import".
	if button, err := page.Timeout(10*time.Second).ElementR("button", `^\s*[Ii][Mm][Pp][Oo][Rr][Tt]\s*$`); err == nil {
		return button, nil
	}

	// Fallbacks keep this resilient to small TMDb markup changes.
	if button, err := page.Timeout(10 * time.Second).Element("button[type='submit']"); err == nil {
		return button, nil
	}
	if button, err := page.Timeout(10 * time.Second).Element("input[type='submit']"); err == nil {
		return button, nil
	}

	return nil, fmt.Errorf("failure finding tmdb import submit button; tmdb import page markup may have changed")
}

func knownImportError(pageText string) string {
	text := strings.ToLower(pageText)
	for _, message := range []string{
		"the file you submitted could not be mapped to one of our supported formats",
		"could not be mapped to one of our supported formats",
		"there was a problem importing",
		"unable to import",
		"invalid csv",
	} {
		if strings.Contains(text, message) {
			return message
		}
	}
	return ""
}
