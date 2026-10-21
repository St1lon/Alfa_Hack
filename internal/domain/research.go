package domain

import (
	"time"

	"github.com/google/uuid"
)

// ResearchDocument — аналитический материал ЦМИ Альфа-Банка (Приложение 2 ТЗ).
// Позиция банка в инсайте — это цитата из его чанка, а не текст модели.
type ResearchDocument struct {
	ID          int16
	Code        string
	Title       string
	FileName    string
	PublishedAt *time.Time
	Pages       int
}

// ResearchChunk — фрагмент материала с привязкой к странице.
// Page обязателен: на нём держится провенанс «документ, стр. N» в карточке.
type ResearchChunk struct {
	ID         uuid.UUID
	Page       int
	ChunkIndex int
	Text       string
	Tokens     int
	// OKVEDHints и Entities — классификация чанка тем же словарём, что и
	// новостей: агент сужает поиск позиции банка до чанков нужной темы.
	OKVEDHints []OKVEDHint
	Entities   []Entity
}
