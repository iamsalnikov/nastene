package render

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
)

const csrfCookieName = "nastene_csrf"

func CSRFMiddleware(next http.Handler) http.Handler {
	return CSRFMiddlewareWithLogger(slog.Default())(next)
}

func CSRFMiddlewareWithLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, err := r.Cookie(csrfCookieName)
			if err != nil || token.Value == "" {
				issued, err := newCSRF()
				if err != nil {
					http.Error(w, "csrf issue", http.StatusInternalServerError)
					return
				}
				http.SetCookie(w, &http.Cookie{
					Name:     csrfCookieName,
					Value:    issued,
					Path:     "/",
					HttpOnly: false,
					SameSite: http.SameSiteLaxMode,
				})
				token = &http.Cookie{Value: issued}
			}

			if isMutating(r.Method) {
				form := r.PostFormValue("_csrf")
				if form == "" {
					form = r.Header.Get("X-CSRF-Token")
				}
				if form == "" || form != token.Value {
					log.Warn("csrf mismatch",
						"method", r.Method,
						"path", r.URL.Path,
						"has_form", form != "",
						"form_len", len(form),
						"cookie_len", len(token.Value),
						"match", form == token.Value,
					)
					http.Error(w, "csrf token invalid — вернитесь на /, перезайдите", http.StatusForbidden)
					return
				}
			}

			ctx := ContextWithCSRF(r.Context(), token.Value)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func isMutating(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}

func newCSRF() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate csrf: %w", err)
	}
	return hex.EncodeToString(b), nil
}
