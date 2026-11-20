package links_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/vexterAA/url-shortener/internal/core/entities"
	"github.com/vexterAA/url-shortener/internal/core/links"
	"github.com/vexterAA/url-shortener/internal/storage/inmemory"
)

// testRepo создаем, чтобы подменить вставку, чтобы воспроизвести конфликт на заданной попытке
type testRepo struct {
	links.Repository
	create func(context.Context, entities.Link) error
}

func (r testRepo) CreateLink(ctx context.Context, link entities.Link) error {
	return r.create(ctx, link)
}

func TestShorten(t *testing.T) {
	t.Parallel()

	service := links.NewService(inmemory.New())
	const originalURL = "HTTPS://Example.org:443/articles/go?tag=short%20link#Intro"

	code, err := service.Shorten(t.Context(), originalURL)
	if err != nil {
		t.Fatalf("Shorten() error = %v", err)
	}
	if !entities.IsCodeValid(code) {
		t.Fatalf("Shorten() = %q, want 10 symbols from the link alphabet", code)
	}

	repeatedCode, err := service.Shorten(t.Context(), originalURL)
	if err != nil {
		t.Fatalf("Shorten() for existing URL error = %v", err)
	}
	if repeatedCode != code {
		t.Errorf("Shorten() for existing URL = %q, want %q", repeatedCode, code)
	}

	resolvedURL, err := service.Resolve(t.Context(), code)
	if err != nil {
		t.Fatalf("Resolve(%q) error = %v", code, err)
	}
	if resolvedURL != originalURL {
		t.Errorf("Resolve(%q) = %q, want %q", code, resolvedURL, originalURL)
	}

	t.Run("URL byte limit", func(t *testing.T) {
		const prefix = "https://example.org/"
		url := prefix + strings.Repeat("я", (2048-len(prefix))/len("я"))

		if _, err := service.Shorten(t.Context(), url); err != nil {
			t.Fatalf("Shorten() for URL of 2048 bytes error = %v", err)
		}
		if _, err := service.Shorten(t.Context(), url+"a"); !errors.Is(err, links.ErrInvalidURL) {
			t.Errorf("Shorten() for URL of 2049 bytes error = %v, want ErrInvalidURL", err)
		}
	})
}

func TestShortenInvalidURL(t *testing.T) {
	t.Parallel()

	service := links.NewService(inmemory.New())
	tests := []struct {
		name string
		url  string
	}{
		{name: "relative URL", url: "/articles/go"},
		{name: "unsupported scheme", url: "ftp://example.org/go"},
		{name: "missing host", url: "http://:80/go"},
		{name: "unescaped space", url: "https://example.org/go lang"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := service.Shorten(t.Context(), test.url); !errors.Is(err, links.ErrInvalidURL) {
				t.Errorf("Shorten(%q) error = %v, want ErrInvalidURL", test.url, err)
			}
		})
	}
}

func TestShortenConflicts(t *testing.T) {
	t.Parallel()

	const (
		originalURL       = "https://example.org/articles/go"
		maxCreateAttempts = 3
	)

	t.Run("same URL created on last attempt", func(t *testing.T) {
		storage := inmemory.New()
		winner := entities.Link{Code: "aB3_45CxY9", URL: originalURL}
		attempts := 0
		repo := testRepo{
			Repository: storage,
			create: func(ctx context.Context, link entities.Link) error {
				attempts++
				if attempts < maxCreateAttempts {
					return links.ErrConflict
				}
				if err := storage.CreateLink(ctx, winner); err != nil {
					return err
				}
				return storage.CreateLink(ctx, link)
			},
		}

		code, err := links.NewService(repo).Shorten(t.Context(), originalURL)
		if err != nil {
			t.Fatalf("Shorten() error = %v", err)
		}
		if code != winner.Code {
			t.Errorf("Shorten() = %q, want %q", code, winner.Code)
		}
		if attempts != maxCreateAttempts {
			t.Errorf("create attempts = %d, want %d", attempts, maxCreateAttempts)
		}
	})

	t.Run("attempts exhausted", func(t *testing.T) {
		attempts := 0
		repo := testRepo{
			Repository: inmemory.New(),
			create: func(context.Context, entities.Link) error {
				attempts++
				return links.ErrConflict
			},
		}

		if _, err := links.NewService(repo).Shorten(t.Context(), originalURL); !errors.Is(err, links.ErrCodeAssignmentFailed) {
			t.Errorf("Shorten() error = %v, want ErrCodeAssignmentFailed", err)
		}
		if attempts != maxCreateAttempts {
			t.Errorf("create attempts = %d, want %d", attempts, maxCreateAttempts)
		}
	})
}

func TestResolveErrors(t *testing.T) {
	t.Parallel()

	service := links.NewService(inmemory.New())
	tests := []struct {
		name string
		code string
		want error
	}{
		{name: "invalid code", code: "aB3-45CxY9", want: links.ErrInvalidCode},
		{name: "unknown code", code: "aB3_45CxY9", want: links.ErrNotFound},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := service.Resolve(t.Context(), test.code); !errors.Is(err, test.want) {
				t.Errorf("Resolve(%q) error = %v, want %v", test.code, err, test.want)
			}
		})
	}
}
