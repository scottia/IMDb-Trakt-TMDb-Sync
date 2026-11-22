package sync

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const cachedRatingsFixture = `Const,Your Rating,Date Rated,Title,Title Type
tt0000001,8,2026-11-01,First,movie
tt0000002,7,2026-11-02,Second,tvSeries
`

func TestIsIMDbWAFBlocked(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "solver remains blocked",
			err: errors.New("failure initialising imdb client: failure handling waf challenge: aws waf challenge remained after solver token and reload"),
			want: true,
		},
		{
			name: "browser native remains blocked",
			err:  errors.New("browser-native aws waf challenge did not clear within 20s"),
			want: true,
		},
		{
			name: "unrelated authentication failure",
			err:  errors.New("failure authenticating with the provided cookies"),
			want: false,
		},
		{
			name: "nil",
			err:  nil,
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isIMDbWAFBlocked(test.err); got != test.want {
				t.Fatalf("isIMDbWAFBlocked() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestCachedIMDbRatingsCount(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, cachedIMDbRatingsFile)
	if err := os.WriteFile(path, []byte(cachedRatingsFixture), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	count, err := cachedIMDbRatingsCount(dir)
	if err != nil {
		t.Fatalf("cachedIMDbRatingsCount() error: %v", err)
	}
	if count != 2 {
		t.Fatalf("cachedIMDbRatingsCount() = %d, want 2", count)
	}
}

func TestHandleCachedWAFSourceUnavailablePreservesBaselineAndFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, cachedIMDbRatingsFile)
	before := []byte(cachedRatingsFixture)
	if err := os.WriteFile(path, before, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	t.Setenv("ITS_STATE_DIR", dir)

	handled, err := handleCachedWAFSourceUnavailable(errors.New("aws waf challenge remained after solver token and reload"))
	if !handled {
		t.Fatal("handleCachedWAFSourceUnavailable() did not recognize WAF error")
	}
	if err == nil {
		t.Fatal("WAF source outage must fail the sync command")
	}
	if !errors.Is(err, errIMDbSourceUnavailable) {
		t.Fatalf("error = %v, want errIMDbSourceUnavailable", err)
	}
	if !strings.Contains(err.Error(), "no sync performed") {
		t.Fatalf("error = %q, want explicit no-sync result", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture after source outage: %v", err)
	}
	if string(after) != string(before) {
		t.Fatal("cached IMDb ratings baseline changed during source outage")
	}
}

func TestHandleCachedWAFSourceUnavailableRequiresValidCache(t *testing.T) {
	t.Setenv("ITS_STATE_DIR", t.TempDir())

	handled, err := handleCachedWAFSourceUnavailable(errors.New("aws waf challenge remained after solver token and reload"))
	if !handled {
		t.Fatal("WAF error should be recognized even when cache is unavailable")
	}
	if err == nil {
		t.Fatal("expected missing cached baseline error")
	}
	if !errors.Is(err, errIMDbSourceUnavailable) {
		t.Fatalf("error = %v, want errIMDbSourceUnavailable", err)
	}
	if !strings.Contains(err.Error(), "cached baseline validation failed") {
		t.Fatalf("error = %q, want cache validation context", err)
	}
}

func TestHandleCachedWAFSourceUnavailableIgnoresOtherErrors(t *testing.T) {
	handled, err := handleCachedWAFSourceUnavailable(errors.New("failure authenticating with the provided cookies"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handled {
		t.Fatal("unrelated IMDb authentication error must not enter degraded source-unavailable handling")
	}
}
