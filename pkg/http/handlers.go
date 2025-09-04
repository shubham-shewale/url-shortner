package http

import (
	"encoding/json"
	"net/http"

	"url-shortener/pkg/middleware"
	"url-shortener/pkg/service"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	linkService *service.LinkService
}

func NewHandler(linkService *service.LinkService) *Handler {
	return &Handler{linkService: linkService}
}

func (h *Handler) CreateLink(w http.ResponseWriter, r *http.Request) {
	var req service.CreateLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	resp, err := h.linkService.CreateLink(r.Context(), &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) Redirect(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	link, err := h.linkService.GetLink(r.Context(), code)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if link == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// Check expiry
	if h.linkService.IsExpired(link) {
		http.Error(w, "gone", http.StatusGone)
		return
	}

	// Check password
	if link.PasswordHash != nil {
		cookie, err := r.Cookie("verified_" + code)
		if err != nil || cookie.Value != "true" {
			// Generate simple CSRF token
			csrfToken := "csrf_" + code // In production, use proper token generation
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			html := `<html>
<head><title>Password Required</title></head>
<body>
<h2>Enter Password to Access Link</h2>
<form method="post" action="/v1/links/` + code + `/verify">
<input type="hidden" name="csrf_token" value="` + csrfToken + `">
<label>Password: <input type="password" name="password" required></label>
<input type="submit" value="Submit">
</form>
</body>
</html>`
			w.Write([]byte(html))
			return
		}
	}

	// Increment click count
	h.linkService.IncrementClickCount(r.Context(), code)

	// Redirect
	http.Redirect(w, r, link.LongURL, http.StatusFound)
}

func (h *Handler) GetLink(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	link, err := h.linkService.GetLink(r.Context(), code)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if link == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(link)
}

func (h *Handler) DeleteLink(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	err := h.linkService.DeleteLink(r.Context(), code)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) UpdateLink(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	var req service.UpdateLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	err := h.linkService.UpdateLink(r.Context(), code, &req)
	if err != nil {
		if err.Error() == "link not found" {
			http.Error(w, "not found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) VerifyPassword(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	password := r.FormValue("password")
	csrfToken := r.FormValue("csrf_token")

	// Simple CSRF check
	expectedToken := "csrf_" + code
	if csrfToken != expectedToken {
		http.Error(w, "invalid csrf token", http.StatusForbidden)
		return
	}

	err := h.linkService.VerifyPassword(r.Context(), code, password)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "verified_" + code,
		Value:    "true",
		Path:     "/r/" + code,
		HttpOnly: true,
		MaxAge:   300,
	})
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func SetupRoutes(r *chi.Mux, handler *Handler, oauthMiddleware *middleware.OAuthMiddleware) {
	r.Get("/health", handler.HealthCheck)
	r.Route("/v1", func(r chi.Router) {
		if oauthMiddleware != nil {
			r.With(oauthMiddleware.Authenticate("links:write")).Post("/links", handler.CreateLink)
			r.With(oauthMiddleware.Authenticate("links:read")).Get("/links/{code}", handler.GetLink)
			r.With(oauthMiddleware.Authenticate("links:write")).Patch("/links/{code}", handler.UpdateLink)
			r.With(oauthMiddleware.Authenticate("links:write")).Delete("/links/{code}", handler.DeleteLink)
		} else {
			r.Post("/links", handler.CreateLink)
			r.Get("/links/{code}", handler.GetLink)
			r.Patch("/links/{code}", handler.UpdateLink)
			r.Delete("/links/{code}", handler.DeleteLink)
		}
		r.Post("/links/{code}/verify", handler.VerifyPassword)
	})
	r.Get("/r/{code}", handler.Redirect)
}
