package loadresearch

import (
	"fmt"
	"strings"
	"time"

	"github.com/st1lon/alfa-hack/internal/domain"
)

// DocumentSpec — строка манифеста configs/research.yaml.
type DocumentSpec struct {
	Code        string
	Title       string
	File        string
	PublishedAt *time.Time
}

// Request — что загрузить.
type Request struct {
	// Dir — каталог, где лежат PDF (data/research).
	Dir       string
	Documents []DocumentSpec
	// Force — перезалить документы, даже если чанки уже есть.
	Force bool
}

func (req *Request) validate() error {
	if strings.TrimSpace(req.Dir) == "" || len(req.Documents) == 0 {
		return domain.ErrInvalidResearchSpec
	}

	seen := make(map[string]struct{}, len(req.Documents))
	for i := range req.Documents {
		doc := &req.Documents[i]
		doc.Code = strings.TrimSpace(doc.Code)
		doc.Title = strings.TrimSpace(doc.Title)
		doc.File = strings.TrimSpace(doc.File)

		if doc.Code == "" || doc.Title == "" || doc.File == "" {
			return fmt.Errorf("%w: document #%d needs code, title and file", domain.ErrInvalidResearchSpec, i+1)
		}
		if _, dup := seen[doc.Code]; dup {
			return fmt.Errorf("%w: duplicate code %q", domain.ErrInvalidResearchSpec, doc.Code)
		}
		seen[doc.Code] = struct{}{}
	}
	return nil
}
