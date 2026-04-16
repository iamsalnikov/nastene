package render

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/iamsalnikov/nastene/internal/session"
)

type IncomingCounter interface {
	CountIncomingRequests(ctx context.Context, toID int64) (int, error)
}

type headerStatsKey struct{}

// HeaderStatsMiddleware injects the logged-in user's incoming friend request count into ctx.
// Templates read it via CSRFFromContext-style helper and show it in the header.
func HeaderStatsMiddleware(counter IncomingCounter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := session.FromContext(r.Context())
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			n, err := counter.CountIncomingRequests(r.Context(), user.ID)
			if err != nil {
				slog.Default().Warn("header stats: count failed", "err", err)
				next.ServeHTTP(w, r)
				return
			}
			ctx := context.WithValue(r.Context(), headerStatsKey{}, n)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func IncomingFromContext(ctx context.Context) int {
	n, _ := ctx.Value(headerStatsKey{}).(int)
	return n
}

// AbbrevCount → "N" / "1.2K" / "3M" / "1.5B" — без плюса, его добавляет шаблон.
func AbbrevCount(n int) string {
	if n < 0 {
		return "0"
	}
	switch {
	case n < 1000:
		return fmt.Sprintf("%d", n)
	case n < 1_000_000:
		return abbrev(n, 1000, "K")
	case n < 1_000_000_000:
		return abbrev(n, 1_000_000, "M")
	default:
		return abbrev(n, 1_000_000_000, "B")
	}
}

func abbrev(n, unit int, suffix string) string {
	q := n / unit
	r := (n % unit) / (unit / 10) // first decimal digit
	if r == 0 {
		return fmt.Sprintf("%d%s", q, suffix)
	}
	return fmt.Sprintf("%d.%d%s", q, r, suffix)
}
