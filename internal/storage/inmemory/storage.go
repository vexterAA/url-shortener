package inmemory

import (
	"context"
	"sync"

	"github.com/vexterAA/url-shortener/internal/core/entities"
	"github.com/vexterAA/url-shortener/internal/core/links"
)

type Storage struct {
	mu         sync.Mutex
	codeByURL  map[string]string
	linkByCode map[string]entities.Link
}

func New() *Storage {
	return &Storage{
		codeByURL:  make(map[string]string),
		linkByCode: make(map[string]entities.Link),
	}
}

func (r *Storage) CreateLink(_ context.Context, link entities.Link) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.codeByURL[link.URL]; exists {
		return links.ErrConflict
	}
	if _, exists := r.linkByCode[link.Code]; exists {
		return links.ErrConflict
	}

	r.codeByURL[link.URL] = link.Code
	r.linkByCode[link.Code] = link
	return nil
}

func (r *Storage) GetLinkByURL(_ context.Context, url string) (entities.Link, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	code, exists := r.codeByURL[url]
	if !exists {
		return entities.Link{}, links.ErrNotFound
	}

	return r.linkByCode[code], nil
}

func (r *Storage) GetLinkByCode(_ context.Context, code string) (entities.Link, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	link, exists := r.linkByCode[code]
	if !exists {
		return entities.Link{}, links.ErrNotFound
	}

	return link, nil
}
