package links

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/vexterAA/url-shortener/internal/core/entities"
)

var (
	ErrInvalidURL           = errors.New("invalid URL")
	ErrInvalidCode          = errors.New("invalid link code")
	ErrCodeAssignmentFailed = errors.New("could not assign a link code")
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Shorten(ctx context.Context, rawURL string) (string, error) {
	// разбираем URL для проверки, в хранилище передаём исходную строку
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" || strings.ContainsRune(rawURL, ' ') {
		return "", ErrInvalidURL
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", ErrInvalidURL
	}

	maxCreateAttempts := 3
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		// после конфликта проверяем, успел ли другой запрос сохранить URL
		// проверка нужна и после последней попытки вставки
		saved, err := s.repo.GetLinkByURL(ctx, rawURL)
		switch {
		case err == nil:
			return saved.Code, nil
		case !errors.Is(err, ErrNotFound):
			return "", fmt.Errorf("find link by URL: %w", err)
		case attempt == maxCreateAttempts:
			return "", fmt.Errorf("%w after %d attempts", ErrCodeAssignmentFailed, maxCreateAttempts)
		}

		code, err := generateCode()
		if err != nil {
			return "", err
		}

		err = s.repo.CreateLink(ctx, entities.Link{Code: code, URL: rawURL})
		switch {
		case err == nil:
			return code, nil
		case !errors.Is(err, ErrConflict):
			return "", fmt.Errorf("create link: %w", err)
		}
	}
}

func (s *Service) Resolve(ctx context.Context, code string) (string, error) {
	if !entities.IsCodeValid(code) {
		return "", ErrInvalidCode
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	link, err := s.repo.GetLinkByCode(ctx, code)
	if err != nil {
		return "", fmt.Errorf("find link by code: %w", err)
	}
	return link.URL, nil
}
