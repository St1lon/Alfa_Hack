package ingestnews_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/st1lon/alfa-hack/internal/domain"
	ingestnews "github.com/st1lon/alfa-hack/services/go-ingest/internal/usecase/news/ingest"
)

// stubRepo — рукописный стаб порта (CLAUDE.md §10: для простых случаев допустим).
type stubRepo struct {
	exists    bool
	existsErr error
	createErr error
	created   *domain.News
}

func (s *stubRepo) ExistsByTextHash(_ context.Context, _ string) (bool, error) {
	return s.exists, s.existsErr
}

func (s *stubRepo) Create(_ context.Context, news *domain.News) error {
	if s.createErr != nil {
		return s.createErr
	}
	s.created = news
	return nil
}

type stubEvents struct {
	appended int
}

func (s *stubEvents) Append(
	_ context.Context, _, _ string, _ domain.PipelineStatus, _, _ string, _ any,
) error {
	s.appended++
	return nil
}

const sampleBody = "Совет директоров Банка России сохранил ключевую ставку на уровне 14 процентов."

func TestIngestNewsStoresFreshNews(t *testing.T) {
	t.Parallel()

	repo := &stubRepo{}
	events := &stubEvents{}
	uc := ingestnews.NewUsecase(repo, events)

	result, err := uc.Execute(context.Background(), &ingestnews.Request{
		URL:   "https://WWW.rbc.ru/news/1?utm_source=tg",
		Title: "ЦБ сохранил ставку",
		Body:  sampleBody,
	})

	require.NoError(t, err)
	require.False(t, result.Duplicate)
	require.NotEmpty(t, result.NewsID.String())

	require.NotNil(t, repo.created)
	require.Equal(t, domain.StatusRaw, repo.created.Status)
	require.Equal(t, "https://rbc.ru/news/1", repo.created.CanonicalURL)
	require.Len(t, repo.created.TextHash, 64)
	require.Equal(t, "ru", repo.created.Lang)
	require.Equal(t, 1, events.appended)
}

func TestIngestNewsSkipsKnownTextHash(t *testing.T) {
	t.Parallel()

	repo := &stubRepo{exists: true}
	uc := ingestnews.NewUsecase(repo, &stubEvents{})

	result, err := uc.Execute(context.Background(), &ingestnews.Request{
		Title: "ЦБ сохранил ставку",
		Body:  sampleBody,
	})

	require.NoError(t, err)
	require.True(t, result.Duplicate)
	require.Nil(t, repo.created)
}

func TestIngestNewsTreatsUniqueViolationAsDuplicate(t *testing.T) {
	t.Parallel()

	// Гонка двух воркеров: проверка не увидела дубликат, а индекс — увидел.
	repo := &stubRepo{createErr: domain.ErrNewsAlreadyExists}
	uc := ingestnews.NewUsecase(repo, &stubEvents{})

	result, err := uc.Execute(context.Background(), &ingestnews.Request{
		Title: "ЦБ сохранил ставку",
		Body:  sampleBody,
	})

	require.NoError(t, err)
	require.True(t, result.Duplicate)
}

func TestIngestNewsValidation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		req     ingestnews.Request
		wantErr error
	}{
		{
			name:    "без заголовка",
			req:     ingestnews.Request{Body: sampleBody},
			wantErr: domain.ErrInvalidNewsInput,
		},
		{
			name:    "слишком короткий текст — анонс или заглушка пейволла",
			req:     ingestnews.Request{Title: "ЦБ", Body: "ставка"},
			wantErr: domain.ErrEmptyNewsBody,
		},
		{
			name:    "битая ссылка",
			req:     ingestnews.Request{Title: "ЦБ", Body: sampleBody, URL: "ftp://rbc.ru/news"},
			wantErr: domain.ErrInvalidNewsURL,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			uc := ingestnews.NewUsecase(&stubRepo{}, &stubEvents{})
			req := tc.req

			_, err := uc.Execute(context.Background(), &req)
			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestIngestNewsPropagatesRepoFailure(t *testing.T) {
	t.Parallel()

	repoErr := errors.New("connection refused")
	uc := ingestnews.NewUsecase(&stubRepo{existsErr: repoErr}, &stubEvents{})

	_, err := uc.Execute(context.Background(), &ingestnews.Request{
		Title: "ЦБ сохранил ставку",
		Body:  sampleBody,
	})

	require.ErrorIs(t, err, repoErr)
}
