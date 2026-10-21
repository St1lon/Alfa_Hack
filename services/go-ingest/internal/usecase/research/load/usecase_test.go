package loadresearch_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/st1lon/alfa-hack/internal/domain"
	loadresearch "github.com/st1lon/alfa-hack/services/go-ingest/internal/usecase/research/load"
)

type stubRepo struct {
	existing map[string]int
	saved    map[string][]domain.ResearchChunk
	docs     map[string]*domain.ResearchDocument
	saveErr  error
}

func newStubRepo() *stubRepo {
	return &stubRepo{
		existing: map[string]int{},
		saved:    map[string][]domain.ResearchChunk{},
		docs:     map[string]*domain.ResearchDocument{},
	}
}

func (s *stubRepo) CountChunks(_ context.Context, code string) (int, error) {
	return s.existing[code], nil
}

func (s *stubRepo) ReplaceDocument(_ context.Context, doc *domain.ResearchDocument, chunks []domain.ResearchChunk) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.docs[doc.Code] = doc
	s.saved[doc.Code] = chunks
	return nil
}

type stubExtractor struct {
	pages map[string][]string
	calls int
}

func (s *stubExtractor) ExtractPages(_ context.Context, path string) ([]string, error) {
	s.calls++
	pages, ok := s.pages[filepath.Base(path)]
	if !ok {
		return nil, errors.New("ocr failed")
	}
	return pages, nil
}

type stubTx struct{ calls int }

func (s *stubTx) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	s.calls++
	return fn(ctx)
}

var chunking = loadresearch.ChunkConfig{MinRunes: 120, TargetRunes: 200, MaxRunes: 300, MinLetters: 40}

const pageText = "Банк России сохранил ключевую ставку на уровне 14,0% годовых. " +
	"Регулятор отметил ускорение роста цен в устойчивых компонентах инфляции. " +
	"Совет директоров не рассматривал вариант снижения ставки на этом заседании. " +
	"Риторика пресс-релиза стала заметно жестче, чем на прошлом заседании."

// touch создаёт пустые файлы: usecase проверяет, что PDF существует.
func touch(t *testing.T, dir string, names ...string) {
	t.Helper()
	for _, name := range names {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), nil, 0o600))
	}
}

func TestLoadResearchStoresChunksWithPages(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	touch(t, dir, "rate.pdf")

	repo := newStubRepo()
	tx := &stubTx{}
	extractor := &stubExtractor{pages: map[string][]string{
		"rate.pdf": {pageText, "Обложка без текста", pageText},
	}}
	uc := loadresearch.NewUsecase(repo, extractor, tx, nil, chunking)

	result, err := uc.Execute(context.Background(), &loadresearch.Request{
		Dir:       dir,
		Documents: []loadresearch.DocumentSpec{{Code: "policy_rate", Title: "Ключевая ставка", File: "rate.pdf"}},
	})

	require.NoError(t, err)
	require.Equal(t, 1, result.Count(loadresearch.OutcomeLoaded))
	require.Equal(t, 1, tx.calls, "документ сохраняется одной транзакцией")

	doc := repo.docs["policy_rate"]
	require.Equal(t, 3, doc.Pages)
	require.Equal(t, "Ключевая ставка", doc.Title)

	chunks := repo.saved["policy_rate"]
	require.NotEmpty(t, chunks)
	pages := map[int]bool{}
	for i, chunk := range chunks {
		require.Equal(t, i, chunk.ChunkIndex, "нумерация сквозная")
		require.Positive(t, chunk.Tokens)
		pages[chunk.Page] = true
	}
	require.True(t, pages[1])
	require.True(t, pages[3])
	require.False(t, pages[2], "страница без текста не даёт чанков")
}

func TestLoadResearchSkipsAlreadyLoaded(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	touch(t, dir, "rate.pdf")

	repo := newStubRepo()
	repo.existing["policy_rate"] = 5
	extractor := &stubExtractor{pages: map[string][]string{"rate.pdf": {pageText}}}
	uc := loadresearch.NewUsecase(repo, extractor, &stubTx{}, nil, chunking)

	req := &loadresearch.Request{
		Dir:       dir,
		Documents: []loadresearch.DocumentSpec{{Code: "policy_rate", Title: "Ставка", File: "rate.pdf"}},
	}

	result, err := uc.Execute(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, 1, result.Count(loadresearch.OutcomeSkipped))
	require.Equal(t, 0, extractor.calls, "OCR не гоняем повторно — это минуты на документ")

	req.Force = true
	result, err = uc.Execute(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, 1, result.Count(loadresearch.OutcomeLoaded), "force перезаливает")
}

func TestLoadResearchFailureDoesNotStopOthers(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	touch(t, dir, "good.pdf", "broken.pdf", "blank.pdf")

	repo := newStubRepo()
	extractor := &stubExtractor{pages: map[string][]string{
		"good.pdf":  {pageText},
		"blank.pdf": {"*", ""},
	}}
	uc := loadresearch.NewUsecase(repo, extractor, &stubTx{}, nil, chunking)

	result, err := uc.Execute(context.Background(), &loadresearch.Request{
		Dir: dir,
		Documents: []loadresearch.DocumentSpec{
			{Code: "missing", Title: "Нет файла", File: "missing.pdf"},
			{Code: "broken", Title: "OCR упал", File: "broken.pdf"},
			{Code: "blank", Title: "Один мусор", File: "blank.pdf"},
			{Code: "good", Title: "Нормальный", File: "good.pdf"},
		},
	})

	require.NoError(t, err)
	require.Equal(t, 3, result.Count(loadresearch.OutcomeFailed))
	require.Equal(t, 1, result.Count(loadresearch.OutcomeLoaded))
	require.Contains(t, result.Documents[0].Error, domain.ErrResearchFileMissing.Error())
	require.Contains(t, result.Documents[2].Error, domain.ErrResearchNoText.Error())
	require.NotEmpty(t, repo.saved["good"])
}

func TestLoadResearchValidation(t *testing.T) {
	t.Parallel()

	uc := loadresearch.NewUsecase(newStubRepo(), &stubExtractor{}, &stubTx{}, nil, chunking)

	cases := map[string]*loadresearch.Request{
		"пустой манифест": {Dir: "data"},
		"без каталога":    {Documents: []loadresearch.DocumentSpec{{Code: "a", Title: "A", File: "a.pdf"}}},
		"без названия":    {Dir: "data", Documents: []loadresearch.DocumentSpec{{Code: "a", File: "a.pdf"}}},
		"дубль кода": {Dir: "data", Documents: []loadresearch.DocumentSpec{
			{Code: "a", Title: "A", File: "a.pdf"},
			{Code: "a", Title: "B", File: "b.pdf"},
		}},
	}

	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := uc.Execute(context.Background(), req)
			require.ErrorIs(t, err, domain.ErrInvalidResearchSpec)
		})
	}
}

func TestLoadResearchReportsSaveError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	touch(t, dir, "rate.pdf")

	repo := newStubRepo()
	repo.saveErr = errors.New("connection reset")
	extractor := &stubExtractor{pages: map[string][]string{"rate.pdf": {pageText}}}
	uc := loadresearch.NewUsecase(repo, extractor, &stubTx{}, nil, chunking)

	result, err := uc.Execute(context.Background(), &loadresearch.Request{
		Dir:       dir,
		Documents: []loadresearch.DocumentSpec{{Code: "rate", Title: "Ставка", File: "rate.pdf"}},
	})

	require.NoError(t, err)
	require.Equal(t, loadresearch.OutcomeFailed, result.Documents[0].Outcome)
	require.Contains(t, result.Documents[0].Error, "connection reset")
}
