package wall_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/iamsalnikov/nastene/internal/domain"
)

func TestService_CreateTextPost_RateLimit(t *testing.T) {
	t.Parallel()

	const author int64 = 7

	tests := map[string]struct {
		postsPerHour int
		count        int
		countErr     error
		expectCount  bool
		expectCreate bool
		wantErr      error
	}{
		"limit disabled (0) skips counter": {
			postsPerHour: 0,
			expectCount:  false,
			expectCreate: true,
		},
		"under limit proceeds": {
			postsPerHour: 60,
			count:        59,
			expectCount:  true,
			expectCreate: true,
		},
		"at limit returns ErrRateLimited": {
			postsPerHour: 60,
			count:        60,
			expectCount:  true,
			expectCreate: false,
			wantErr:      domain.ErrRateLimited,
		},
		"counter error bubbles up": {
			postsPerHour: 60,
			countErr:     errors.New("db boom"),
			expectCount:  true,
			expectCreate: false,
			wantErr:      errors.New("db boom"),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			svc, f := newLoadPostService(t)
			svc.SetRateLimits(tc.postsPerHour, 0)

			if tc.expectCount {
				f.posts.EXPECT().
					CountByAuthorSince(mock.Anything, author, mock.Anything).
					Return(tc.count, tc.countErr).Once()
			}
			if tc.expectCreate {
				f.posts.EXPECT().Create(mock.Anything, mock.MatchedBy(func(p domain.WallPost) bool {
					return p.AuthorID == author && p.WallOwnerID == author && p.BodyText == "hi"
				})).Return(domain.WallPost{ID: 1, AuthorID: author, WallOwnerID: author}, nil).Once()
			}

			_, err := svc.CreateTextPost(context.Background(), author, author, "hi")
			if tc.wantErr != nil {
				require.Error(t, err)
				if errors.Is(tc.wantErr, domain.ErrRateLimited) {
					require.True(t, errors.Is(err, domain.ErrRateLimited), "want ErrRateLimited, got %v", err)
				}
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestService_CreateComment_RateLimit(t *testing.T) {
	t.Parallel()

	const author int64 = 7
	const postID int64 = 42

	samplePost := domain.WallPost{
		ID: postID, WallOwnerID: author, AuthorID: author,
		Kind: domain.PostText, BodyText: "seed", CreatedAt: time.Now(),
	}

	tests := map[string]struct {
		commentsPerHour int
		count           int
		countErr        error
		expectCount     bool
		expectCreate    bool
		wantErr         error
	}{
		"limit disabled (0) skips counter": {
			commentsPerHour: 0,
			expectCount:     false,
			expectCreate:    true,
		},
		"under limit proceeds": {
			commentsPerHour: 120,
			count:           119,
			expectCount:     true,
			expectCreate:    true,
		},
		"at limit returns ErrRateLimited": {
			commentsPerHour: 120,
			count:           120,
			expectCount:     true,
			expectCreate:    false,
			wantErr:         domain.ErrRateLimited,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			svc, f := newLoadPostService(t)
			svc.SetRateLimits(0, tc.commentsPerHour)

			f.posts.EXPECT().ByID(mock.Anything, postID).Return(samplePost, nil).Once()

			if tc.expectCount {
				f.comments.EXPECT().
					CountByAuthorSince(mock.Anything, author, mock.Anything).
					Return(tc.count, tc.countErr).Once()
			}
			if tc.expectCreate {
				f.comments.EXPECT().Create(mock.Anything, postID, author, "hi").
					Return(domain.Comment{ID: 1, PostID: postID, AuthorID: author, Body: "hi"}, nil).Once()
			}

			_, err := svc.CreateComment(context.Background(), author, postID, "hi")
			if tc.wantErr != nil {
				require.Error(t, err)
				require.True(t, errors.Is(err, domain.ErrRateLimited), "want ErrRateLimited, got %v", err)
				return
			}
			require.NoError(t, err)
		})
	}
}

