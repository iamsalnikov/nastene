package wall

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"image/png"
	"io"
	"path"

	"github.com/iamsalnikov/nastene/internal/domain"
)

const (
	maxGraffitiBytes = 1 << 20 // 1 MiB
	graffitiMime     = "image/png"
)

type GraffitiStore interface {
	Save(ctx context.Context, key string, data io.Reader, size int64, contentType string) error
}

// CreateGraffitiPost validates PNG, saves it to the object store, and creates a post of kind=graffiti.
func (s *Service) CreateGraffitiPost(ctx context.Context, authorID, ownerID int64, pngData io.Reader, store GraffitiStore) (domain.WallPost, error) {
	ok, err := s.authorizer.CanPost(ctx, authorID, ownerID)
	if err != nil {
		return domain.WallPost{}, fmt.Errorf("create graffiti: %w", err)
	}
	if !ok {
		return domain.WallPost{}, fmt.Errorf("create graffiti: %w", domain.ErrForbidden)
	}

	if err := s.checkPostLimit(ctx, authorID); err != nil {
		return domain.WallPost{}, fmt.Errorf("create graffiti: %w", err)
	}

	limited := io.LimitReader(pngData, maxGraffitiBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return domain.WallPost{}, fmt.Errorf("create graffiti: read: %w", err)
	}
	if len(raw) > maxGraffitiBytes {
		return domain.WallPost{}, fmt.Errorf("create graffiti: too large: %w", domain.ErrInvalidInput)
	}

	if _, err := png.Decode(bytes.NewReader(raw)); err != nil {
		return domain.WallPost{}, fmt.Errorf("create graffiti: invalid png: %w", domain.ErrInvalidInput)
	}

	name, err := randomName()
	if err != nil {
		return domain.WallPost{}, fmt.Errorf("create graffiti: name: %w", err)
	}
	key := path.Join("graffiti", name+".png")

	if err := store.Save(ctx, key, bytes.NewReader(raw), int64(len(raw)), graffitiMime); err != nil {
		return domain.WallPost{}, fmt.Errorf("create graffiti: save: %w", err)
	}

	post, err := s.posts.Create(ctx, domain.WallPost{
		WallOwnerID:  ownerID,
		AuthorID:     authorID,
		Kind:         domain.PostGraffiti,
		GraffitiPath: key,
	})
	if err != nil {
		return domain.WallPost{}, fmt.Errorf("create graffiti: insert: %w", err)
	}

	s.publishWallPost(post.ID)
	return post, nil
}

func randomName() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("random: %w", err)
	}
	return hex.EncodeToString(b), nil
}
