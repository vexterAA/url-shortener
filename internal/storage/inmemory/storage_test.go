package inmemory_test

import (
	"errors"
	"testing"

	"github.com/vexterAA/url-shortener/internal/core/entities"
	"github.com/vexterAA/url-shortener/internal/core/links"
	"github.com/vexterAA/url-shortener/internal/storage/inmemory"
)

func TestCreateLink(t *testing.T) {
	t.Parallel()

	link := entities.Link{Code: "0123456789", URL: "https://example.org/a"}
	t.Run("new link", func(t *testing.T) {
		var repo links.Repository = inmemory.New()
		if err := repo.CreateLink(t.Context(), link); err != nil {
			t.Fatalf("CreateLink() error = %v", err)
		}

		saved, err := repo.GetLinkByCode(t.Context(), link.Code)
		if err != nil {
			t.Fatalf("GetLinkByCode() after create error = %v", err)
		}
		if saved != link {
			t.Errorf("saved link = %+v, want %+v", saved, link)
		}
	})

	t.Run("duplicate URL", func(t *testing.T) {
		repo := repositoryWithLink(t, link)
		duplicate := entities.Link{Code: "abcdefghij", URL: link.URL}
		if err := repo.CreateLink(t.Context(), duplicate); !errors.Is(err, links.ErrConflict) {
			t.Fatalf("CreateLink() with duplicate URL error = %v, want ErrConflict", err)
		}
		if _, err := repo.GetLinkByCode(t.Context(), duplicate.Code); !errors.Is(err, links.ErrNotFound) {
			t.Errorf("GetLinkByCode() for rejected code error = %v, want ErrNotFound", err)
		}

		saved, err := repo.GetLinkByURL(t.Context(), link.URL)
		if err != nil {
			t.Fatalf("GetLinkByURL() for saved link error = %v", err)
		}
		if saved != link {
			t.Errorf("saved link = %+v, want %+v", saved, link)
		}
	})

	t.Run("duplicate code", func(t *testing.T) {
		repo := repositoryWithLink(t, link)
		duplicate := entities.Link{Code: link.Code, URL: "https://example.org/b"}
		if err := repo.CreateLink(t.Context(), duplicate); !errors.Is(err, links.ErrConflict) {
			t.Fatalf("CreateLink() with duplicate code error = %v, want ErrConflict", err)
		}
		if _, err := repo.GetLinkByURL(t.Context(), duplicate.URL); !errors.Is(err, links.ErrNotFound) {
			t.Errorf("GetLinkByURL() for rejected URL error = %v, want ErrNotFound", err)
		}

		saved, err := repo.GetLinkByCode(t.Context(), link.Code)
		if err != nil {
			t.Fatalf("GetLinkByCode() for saved link error = %v", err)
		}
		if saved != link {
			t.Errorf("saved link = %+v, want %+v", saved, link)
		}
	})
}

func TestGetLinkByURL(t *testing.T) {
	t.Parallel()

	link := entities.Link{Code: "0123456789", URL: "https://example.org/a"}

	t.Run("existing URL", func(t *testing.T) {
		repo := repositoryWithLink(t, link)
		got, err := repo.GetLinkByURL(t.Context(), link.URL)
		if err != nil {
			t.Fatalf("GetLinkByURL() error = %v", err)
		}
		if got != link {
			t.Errorf("GetLinkByURL() = %+v, want %+v", got, link)
		}
	})

	t.Run("unknown URL", func(t *testing.T) {
		repo := repositoryWithLink(t, link)
		if _, err := repo.GetLinkByURL(t.Context(), "https://example.org/b"); !errors.Is(err, links.ErrNotFound) {
			t.Errorf("GetLinkByURL() error = %v, want ErrNotFound", err)
		}
	})
}

func TestGetLinkByCode(t *testing.T) {
	t.Parallel()

	link := entities.Link{Code: "0123456789", URL: "https://example.org/a"}

	t.Run("existing code", func(t *testing.T) {
		repo := repositoryWithLink(t, link)
		got, err := repo.GetLinkByCode(t.Context(), link.Code)
		if err != nil {
			t.Fatalf("GetLinkByCode() error = %v", err)
		}
		if got != link {
			t.Errorf("GetLinkByCode() = %+v, want %+v", got, link)
		}
	})

	t.Run("unknown code", func(t *testing.T) {
		repo := repositoryWithLink(t, link)
		if _, err := repo.GetLinkByCode(t.Context(), "abcdefghij"); !errors.Is(err, links.ErrNotFound) {
			t.Errorf("GetLinkByCode() error = %v, want ErrNotFound", err)
		}
	})
}

func TestCreateLinkConcurrent(t *testing.T) {
	t.Parallel()

	repo := inmemory.New()
	url := "https://example.org/a"
	candidates := []entities.Link{
		{Code: "0123456789", URL: url},
		{Code: "abcdefghij", URL: url},
	}

	ctx := t.Context()
	start := make(chan struct{})
	results := make(chan error, len(candidates))
	for _, candidate := range candidates {
		go func() {
			<-start
			results <- repo.CreateLink(ctx, candidate)
		}()
	}
	close(start)

	created := 0
	for range candidates {
		err := <-results
		if err == nil {
			created++
		} else if !errors.Is(err, links.ErrConflict) {
			t.Errorf("CreateLink() error = %v, want nil or ErrConflict", err)
		}
	}
	if created != 1 {
		t.Fatalf("successful creates = %d, want 1", created)
	}

	saved, err := repo.GetLinkByURL(ctx, url)
	if err != nil {
		t.Fatalf("GetLinkByURL() error = %v", err)
	}
	byCode, err := repo.GetLinkByCode(ctx, saved.Code)
	if err != nil {
		t.Fatalf("GetLinkByCode() for saved link error = %v", err)
	}
	if byCode != saved {
		t.Errorf("GetLinkByCode() = %+v, want %+v", byCode, saved)
	}
	for _, candidate := range candidates {
		if candidate.Code == saved.Code {
			continue
		}
		if _, err := repo.GetLinkByCode(ctx, candidate.Code); !errors.Is(err, links.ErrNotFound) {
			t.Errorf("GetLinkByCode() for losing candidate error = %v, want ErrNotFound", err)
		}
	}
}

func repositoryWithLink(t *testing.T, link entities.Link) *inmemory.Storage {
	t.Helper()

	repo := inmemory.New()
	if err := repo.CreateLink(t.Context(), link); err != nil {
		t.Fatalf("prepare repository: CreateLink() error = %v", err)
	}
	return repo
}
