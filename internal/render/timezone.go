package render

import (
	"context"
	"net/http"
	"sync"
	"time"

	_ "time/tzdata"
)

const tzCookieName = "nastene_tz"

var defaultLocation = mustLoadLocation("Europe/Moscow")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return loc
}

var locCache sync.Map // string -> *time.Location

func loadLocation(name string) (*time.Location, bool) {
	if name == "" || len(name) > 64 {
		return nil, false
	}
	if v, ok := locCache.Load(name); ok {
		return v.(*time.Location), true
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, false
	}
	locCache.Store(name, loc)
	return loc, true
}

type locationKey struct{}

// TimezoneMiddleware reads the `nastene_tz` cookie (IANA name set by browser JS)
// and puts the resolved *time.Location into the request context.
// Missing/invalid cookie falls back to Europe/Moscow.
func TimezoneMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		loc := defaultLocation
		if c, err := r.Cookie(tzCookieName); err == nil {
			if l, ok := loadLocation(c.Value); ok {
				loc = l
			}
		}
		ctx := context.WithValue(r.Context(), locationKey{}, loc)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func LocationFromContext(ctx context.Context) *time.Location {
	if loc, ok := ctx.Value(locationKey{}).(*time.Location); ok && loc != nil {
		return loc
	}
	return defaultLocation
}
