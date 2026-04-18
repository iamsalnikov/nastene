package render_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iamsalnikov/nastene/internal/render"
)

func TestTimezoneMiddleware(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		cookie string
		want   string
	}{
		"no cookie falls back to Moscow": {cookie: "", want: "Europe/Moscow"},
		"valid IANA name":                {cookie: "America/New_York", want: "America/New_York"},
		"invalid name falls back":        {cookie: "Not/AZone", want: "Europe/Moscow"},
		"overlong value falls back":      {cookie: string(make([]byte, 128)), want: "Europe/Moscow"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var got *time.Location
			handler := render.TimezoneMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = render.LocationFromContext(r.Context())
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.cookie != "" {
				req.AddCookie(&http.Cookie{Name: "nastene_tz", Value: tc.cookie})
			}
			handler.ServeHTTP(httptest.NewRecorder(), req)

			require.NotNil(t, got)
			require.Equal(t, tc.want, got.String())
		})
	}
}

func TestLocationFromContextDefaults(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	loc := render.LocationFromContext(req.Context())
	require.NotNil(t, loc)
	require.Equal(t, "Europe/Moscow", loc.String())
}

func TestPresenceRU(t *testing.T) {
	t.Parallel()

	require.Equal(t, "давно не заходил", render.Presence(nil))

	// Год назад → «был 2 янв 2025» (по Europe/Moscow — фолбэку).
	yearAgo := time.Now().Add(-365 * 24 * time.Hour)
	got := render.Presence(&yearAgo)
	require.True(t, strings.HasPrefix(got, "был "), "got %q", got)
}

func TestFormatBirthDateRU(t *testing.T) {
	t.Parallel()

	d := time.Date(2006, time.April, 18, 0, 0, 0, 0, time.UTC)
	require.Equal(t, "18 апреля 2006", render.FormatBirthDate(&d))
	require.Equal(t, "", render.FormatBirthDate(nil))
}
