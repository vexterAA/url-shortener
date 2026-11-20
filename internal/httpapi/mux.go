package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/vexterAA/url-shortener/internal/core/links"
)

func NewMux(service *links.Service, logger *slog.Logger) http.Handler {
	h := handler{service: service, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /links", h.shorten)
	mux.HandleFunc("GET /links/{code}", h.resolve)
	return mux
}
