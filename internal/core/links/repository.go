package links

import (
	"context"
	"errors"

	"github.com/vexterAA/url-shortener/internal/core/entities"
)

var (
	ErrConflict = errors.New("link code or URL is already in use")
	ErrNotFound = errors.New("link not found")
)

type Repository interface {
	CreateLink(ctx context.Context, link entities.Link) error
	GetLinkByURL(ctx context.Context, url string) (entities.Link, error)
	GetLinkByCode(ctx context.Context, code string) (entities.Link, error)
}
