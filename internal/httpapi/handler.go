package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"

	"github.com/vexterAA/url-shortener/internal/core/links"
)

type handler struct {
	service *links.Service
	logger  *slog.Logger
}

func (h *handler) shorten(w http.ResponseWriter, r *http.Request) {
	const maxBodyBytes = 64 << 10

	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		h.writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "content type must be application/json"})
		return
	}

	// лимит срабатывает при чтении тела, даже если Content-Length отсутствует
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		var sizeError *http.MaxBytesError
		if errors.As(err, &sizeError) {
			h.writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "request body too large"})
			return
		}
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "could not read request body"})
		return
	}

	var request struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	code, err := h.service.Shorten(r.Context(), request.URL)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]string{"code": code})
}

func (h *handler) resolve(w http.ResponseWriter, r *http.Request) {
	originalURL, err := h.service.Resolve(r.Context(), r.PathValue("code"))
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]string{"url": originalURL})
}

func (h *handler) writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	message := "internal server error"

	switch {
	case errors.Is(err, links.ErrInvalidURL):
		status, message = http.StatusBadRequest, "invalid URL"
	case errors.Is(err, links.ErrInvalidCode):
		status, message = http.StatusBadRequest, "invalid link code"
	case errors.Is(err, links.ErrNotFound):
		status, message = http.StatusNotFound, "link not found"
	case errors.Is(err, links.ErrCodeAssignmentFailed):
		message = "could not shorten the URL, try again later"
	}

	if status == http.StatusInternalServerError {
		h.logger.ErrorContext(r.Context(), "handle link request", "method", r.Method, "error", err)
	}
	h.writeJSON(w, status, map[string]string{"error": message})
}

func (h *handler) writeJSON(w http.ResponseWriter, status int, body map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		h.logger.Error("write HTTP response", "error", err)
	}
}
