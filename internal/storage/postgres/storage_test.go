//go:build integration

package postgres_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/vexterAA/url-shortener/internal/core/entities"
	"github.com/vexterAA/url-shortener/internal/core/links"
	"github.com/vexterAA/url-shortener/internal/storage/postgres"
)

func TestCreateAndGetLink(t *testing.T) {
	t.Parallel()

	repo := newTestStorage(t)
	ctx := t.Context()
	const prefix = "https://Example.org/"
	path := make([]byte, (2048-len(prefix))/2)
	rand.Read(path)
	link := entities.Link{Code: "aB3_45CxY9", URL: prefix + hex.EncodeToString(path)}
	if err := repo.CreateLink(ctx, link); err != nil {
		t.Fatalf("CreateLink() error = %v", err)
	}

	if got, err := repo.GetLinkByURL(ctx, link.URL); err != nil || got != link {
		t.Errorf("GetLinkByURL() = %+v, %v, want %+v, nil", got, err, link)
	}
	if got, err := repo.GetLinkByCode(ctx, link.Code); err != nil || got != link {
		t.Errorf("GetLinkByCode() = %+v, %v, want %+v, nil", got, err, link)
	}
	if _, err := repo.GetLinkByURL(ctx, strings.ToLower(link.URL)); !errors.Is(err, links.ErrNotFound) {
		t.Errorf("GetLinkByURL() with different case error = %v, want ErrNotFound", err)
	}
	if _, err := repo.GetLinkByCode(ctx, strings.ToLower(link.Code)); !errors.Is(err, links.ErrNotFound) {
		t.Errorf("GetLinkByCode() with different case error = %v, want ErrNotFound", err)
	}
}

func TestCreateLinkConflicts(t *testing.T) {
	t.Parallel()

	repo := newTestStorage(t)
	link := entities.Link{Code: "aB3_45CxY9", URL: "https://example.org/a"}
	if err := repo.CreateLink(t.Context(), link); err != nil {
		t.Fatalf("CreateLink() error = %v", err)
	}

	tests := []struct {
		name string
		link entities.Link
	}{
		{name: "duplicate URL", link: entities.Link{Code: "abcdefghij", URL: link.URL}},
		{name: "duplicate code", link: entities.Link{Code: link.Code, URL: "https://example.org/b"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := repo.CreateLink(t.Context(), test.link); !errors.Is(err, links.ErrConflict) {
				t.Errorf("CreateLink() error = %v, want ErrConflict", err)
			}
		})
	}
}

func newTestStorage(t *testing.T) links.Repository {
	t.Helper()

	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Fatal("TEST_POSTGRES_DSN is required for integration tests")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse test database config: %v", err)
	}
	admin := stdlib.OpenDB(*config)

	schema := "shortener_test_" + strings.ToLower(rand.Text())
	schemaIdentifier := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+schemaIdentifier); err != nil {
		_ = admin.Close()
		t.Fatalf("create test schema: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		defer admin.Close()
		if _, err := admin.ExecContext(cleanupCtx, "DROP SCHEMA "+schemaIdentifier+" CASCADE"); err != nil {
			t.Errorf("drop test schema: %v", err)
		}
	})

	schemaConfig := config.Copy()
	schemaConfig.RuntimeParams["search_path"] = schema
	schemaConfig.RuntimeParams["statement_timeout"] = "5000"
	db := stdlib.OpenDB(*schemaConfig)
	t.Cleanup(func() { _ = db.Close() })

	migration, err := os.ReadFile("migrations/001_create_links.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if _, err := db.ExecContext(ctx, string(migration)); err != nil {
		t.Fatalf("apply migration: %v", err)
	}
	return postgres.New(db)
}
