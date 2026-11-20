package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/vexterAA/url-shortener/internal/core/entities"
	"github.com/vexterAA/url-shortener/internal/core/links"
)

type Storage struct {
	db *sql.DB
}

func New(db *sql.DB) *Storage {
	return &Storage{db: db}
}

func (s *Storage) CreateLink(ctx context.Context, link entities.Link) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO links (code, url) VALUES ($1, $2)`, link.Code, link.URL)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return links.ErrConflict
	}
	return err
}

func (s *Storage) GetLinkByURL(ctx context.Context, url string) (entities.Link, error) {
	var link entities.Link
	row := s.db.QueryRowContext(ctx, `SELECT code, url FROM links WHERE url = $1`, url)
	switch err := row.Scan(&link.Code, &link.URL); {
	case errors.Is(err, sql.ErrNoRows):
		return entities.Link{}, links.ErrNotFound
	case err != nil:
		return entities.Link{}, err
	}
	return link, nil
}

func (s *Storage) GetLinkByCode(ctx context.Context, code string) (entities.Link, error) {
	var link entities.Link
	row := s.db.QueryRowContext(ctx, `SELECT code, url FROM links WHERE code = $1`, code)
	switch err := row.Scan(&link.Code, &link.URL); {
	case errors.Is(err, sql.ErrNoRows):
		return entities.Link{}, links.ErrNotFound
	case err != nil:
		return entities.Link{}, err
	}
	return link, nil
}
