// Package domain содержит чистые сущности и sentinel-ошибки.
// Ноль импортов инфраструктуры, SQL и HTTP (CLAUDE.md §1).
package domain

// PipelineStatus — статус в машине состояний пайплайна.
// Значения совпадают с таблицей-справочником pipeline_statuses.
type PipelineStatus string

// Статусы пайплайна: RAW → DEDUPED → ENRICHED → SCORED → ANALYZED → PUBLISHED.
// FILTERED — штатный выход из воронки (шум, нерелевантный сюжет), FAILED — сбой.
const (
	StatusRaw       PipelineStatus = "RAW"
	StatusDeduped   PipelineStatus = "DEDUPED"
	StatusEnriched  PipelineStatus = "ENRICHED"
	StatusScored    PipelineStatus = "SCORED"
	StatusAnalyzed  PipelineStatus = "ANALYZED"
	StatusPublished PipelineStatus = "PUBLISHED"
	StatusFailed    PipelineStatus = "FAILED"
	StatusFiltered  PipelineStatus = "FILTERED"
)

// Идентификаторы статусов в справочнике pipeline_statuses (миграции 00001, 00003).
const (
	statusIDRaw int16 = iota + 1
	statusIDDeduped
	statusIDEnriched
	statusIDScored
	statusIDAnalyzed
	statusIDPublished
	statusIDFailed
	statusIDFiltered
)

// statusIDs — соответствие кода статуса его id в справочнике pipeline_statuses.
// Держим здесь, чтобы репозитории не хардкодили числа в SQL.
var statusIDs = map[PipelineStatus]int16{
	StatusRaw:       statusIDRaw,
	StatusDeduped:   statusIDDeduped,
	StatusEnriched:  statusIDEnriched,
	StatusScored:    statusIDScored,
	StatusAnalyzed:  statusIDAnalyzed,
	StatusPublished: statusIDPublished,
	StatusFailed:    statusIDFailed,
	StatusFiltered:  statusIDFiltered,
}

// allowedTransitions — разрешённые переходы статусной машины.
// Владелец перехода один, идемпотентность — на уровне БД (см. миграцию 00001).
var allowedTransitions = map[PipelineStatus][]PipelineStatus{
	StatusRaw:       {StatusDeduped, StatusFailed},
	StatusDeduped:   {StatusEnriched, StatusFiltered, StatusFailed},
	StatusEnriched:  {StatusScored, StatusFailed},
	StatusScored:    {StatusAnalyzed, StatusFiltered, StatusFailed},
	StatusAnalyzed:  {StatusPublished, StatusFailed},
	StatusPublished: {},
	StatusFailed:    {StatusRaw},
	StatusFiltered:  {},
}

// ParsePipelineStatus проверяет, что код статуса известен справочнику.
func ParsePipelineStatus(code string) (PipelineStatus, error) {
	status := PipelineStatus(code)
	if _, ok := statusIDs[status]; !ok {
		return "", ErrUnknownPipelineStatus
	}
	return status, nil
}

// ID возвращает числовой идентификатор статуса из справочника.
func (s PipelineStatus) ID() (int16, error) {
	id, ok := statusIDs[s]
	if !ok {
		return 0, ErrUnknownPipelineStatus
	}
	return id, nil
}

// String возвращает код статуса.
func (s PipelineStatus) String() string {
	return string(s)
}

// CanTransitionTo сообщает, разрешён ли переход в целевой статус.
func (s PipelineStatus) CanTransitionTo(target PipelineStatus) bool {
	for _, allowed := range allowedTransitions[s] {
		if allowed == target {
			return true
		}
	}
	return false
}
