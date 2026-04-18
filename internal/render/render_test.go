package render_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iamsalnikov/nastene/internal/render"
	"github.com/iamsalnikov/nastene/web"
)

type nopPresigner struct{}

func (nopPresigner) PresignURL(_ context.Context, _ string) (string, error) {
	return "", nil
}

func TestTemplatesParse(t *testing.T) {
	t.Parallel()
	_, err := render.New(web.FS, nopPresigner{})
	require.NoError(t, err)
}
