package httpapi_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vexterAA/url-shortener/internal/core/entities"
	"github.com/vexterAA/url-shortener/internal/core/links"
	"github.com/vexterAA/url-shortener/internal/httpapi"
	"github.com/vexterAA/url-shortener/internal/storage/inmemory"
)

func TestShortenAndResolve(t *testing.T) {
	t.Parallel()

	mux := httpapi.NewMux(links.NewService(inmemory.New()), slog.New(slog.DiscardHandler))
	const originalURL = "https://example.org/go?lang=go#intro"

	shortened := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/links", strings.NewReader(`{"url":"`+originalURL+`"}`))
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	mux.ServeHTTP(shortened, request)
	if shortened.Code != http.StatusOK {
		t.Fatalf("POST /links status = %d, body = %s", shortened.Code, shortened.Body)
	}
	var shortenedBody struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(shortened.Body.Bytes(), &shortenedBody); err != nil {
		t.Fatalf("decode POST /links response: %v", err)
	}
	if !entities.IsCodeValid(shortenedBody.Code) {
		t.Fatalf("POST /links code = %q, want a valid code", shortenedBody.Code)
	}

	resolved := httptest.NewRecorder()
	mux.ServeHTTP(resolved, httptest.NewRequest(http.MethodGet, "/links/"+shortenedBody.Code, nil))
	if resolved.Code != http.StatusOK {
		t.Fatalf("GET /links/{code} status = %d, body = %s", resolved.Code, resolved.Body)
	}
	var resolvedBody struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(resolved.Body.Bytes(), &resolvedBody); err != nil {
		t.Fatalf("decode GET /links/{code} response: %v", err)
	}
	if resolvedBody.URL != originalURL {
		t.Errorf("GET /links/{code} URL = %q, want %q", resolvedBody.URL, originalURL)
	}
}

func TestRequestErrors(t *testing.T) {
	t.Parallel()

	mux := httpapi.NewMux(links.NewService(inmemory.New()), slog.New(slog.DiscardHandler))
	const validBody = `{"url":"https://example.org/go"}`
	tests := []struct {
		name        string
		method      string
		path        string
		contentType string
		body        string
		status      int
	}{
		{
			name: "invalid JSON", method: http.MethodPost, path: "/links",
			contentType: "application/json", body: `{"url":`, status: http.StatusBadRequest,
		},
		{
			name: "invalid URL", method: http.MethodPost, path: "/links",
			contentType: "application/json", body: `{"url":"/relative"}`, status: http.StatusBadRequest,
		},
		{
			name: "large body", method: http.MethodPost, path: "/links",
			contentType: "application/json", body: strings.Repeat(" ", 65<<10), status: http.StatusRequestEntityTooLarge,
		},
		{
			name: "missing content type", method: http.MethodPost, path: "/links",
			body: validBody, status: http.StatusUnsupportedMediaType,
		},
		{
			name: "unsupported content type", method: http.MethodPost, path: "/links",
			contentType: "text/plain", body: validBody, status: http.StatusUnsupportedMediaType,
		},
		{
			name: "malformed content type", method: http.MethodPost, path: "/links",
			contentType: "application/json; charset", body: validBody, status: http.StatusUnsupportedMediaType,
		},
		{name: "invalid code", method: http.MethodGet, path: "/links/aB3-45CxY9", status: 400},
		{name: "unknown code", method: http.MethodGet, path: "/links/aB3_45CxY9", status: 404},
		{name: "unsupported method", method: http.MethodPut, path: "/links/aB3_45CxY9", status: 405},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			if test.contentType != "" {
				request.Header.Set("Content-Type", test.contentType)
			}
			mux.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Errorf("%s %s status = %d, want %d", test.method, test.path, response.Code, test.status)
			}
		})
	}
}
