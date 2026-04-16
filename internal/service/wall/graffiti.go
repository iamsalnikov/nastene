package wall

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"image/png"
	"io"
	"os"
	"path/filepath"

	"github.com/iamsalnikov/nastene/internal/domain"
)

const (
	maxGraffitiBytes = 1 << 20 // 1 MiB
)

type GraffitiStore interface {
	Save(ctx context.Context, relPath string, data io.Reader) error
}

// CreateGraffitiPost validates PNG, saves it to disk, and creates a post of kind=graffiti.
func (s *Service) CreateGraffitiPost(ctx context.Context, authorID, ownerID int64, pngData io.Reader, store GraffitiStore) (domain.WallPost, error) {
	ok, err := s.authorizer.CanPost(ctx, authorID, ownerID)
	if err != nil {
		return domain.WallPost{}, fmt.Errorf("create graffiti: %w", err)
	}
	if !ok {
		return domain.WallPost{}, fmt.Errorf("create graffiti: %w", domain.ErrForbidden)
	}

	limited := io.LimitReader(pngData, maxGraffitiBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return domain.WallPost{}, fmt.Errorf("create graffiti: read: %w", err)
	}
	if len(raw) > maxGraffitiBytes {
		return domain.WallPost{}, fmt.Errorf("create graffiti: too large: %w", domain.ErrInvalidInput)
	}

	// decode to verify valid PNG
	if _, err := png.Decode(bytesReaderFromSlice(raw)); err != nil {
		return domain.WallPost{}, fmt.Errorf("create graffiti: invalid png: %w", domain.ErrInvalidInput)
	}

	name, err := randomName()
	if err != nil {
		return domain.WallPost{}, fmt.Errorf("create graffiti: name: %w", err)
	}
	rel := filepath.Join("graffiti", name+".png")

	if err := store.Save(ctx, rel, bytesReaderFromSlice(raw)); err != nil {
		return domain.WallPost{}, fmt.Errorf("create graffiti: save: %w", err)
	}

	post, err := s.posts.Create(ctx, domain.WallPost{
		WallOwnerID:  ownerID,
		AuthorID:     authorID,
		Kind:         domain.PostGraffiti,
		GraffitiPath: rel,
	})
	if err != nil {
		return domain.WallPost{}, fmt.Errorf("create graffiti: insert: %w", err)
	}
	return post, nil
}

func bytesReaderFromSlice(b []byte) io.Reader {
	return &sliceReader{b: b}
}

type sliceReader struct {
	b   []byte
	pos int
}

func (s *sliceReader) Read(p []byte) (int, error) {
	if s.pos >= len(s.b) {
		return 0, io.EOF
	}
	n := copy(p, s.b[s.pos:])
	s.pos += n
	return n, nil
}

func randomName() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("random: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// DiskStore saves files under baseDir; relPath is joined with baseDir. Creates subdirs as needed.
type DiskStore struct {
	BaseDir string
}

func (d *DiskStore) Save(ctx context.Context, relPath string, data io.Reader) error {
	_ = ctx
	if d.BaseDir == "" {
		return errors.New("disk store: base dir empty")
	}
	full := filepath.Join(d.BaseDir, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	f, err := os.Create(full)
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, data); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}
