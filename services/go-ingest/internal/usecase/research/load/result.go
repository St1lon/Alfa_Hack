package loadresearch

// DocumentOutcome — что произошло с одним документом.
type DocumentOutcome string

// Исходы загрузки документа.
const (
	OutcomeLoaded  DocumentOutcome = "loaded"
	OutcomeSkipped DocumentOutcome = "skipped"
	OutcomeFailed  DocumentOutcome = "failed"
)

// DocumentReport — отчёт по одному документу.
type DocumentReport struct {
	Code    string
	Outcome DocumentOutcome
	Pages   int
	Chunks  int
	Error   string
}

// Result — отчёт по всей загрузке.
type Result struct {
	Documents []DocumentReport
}

// Count — сколько документов закончилось с заданным исходом.
func (r *Result) Count(outcome DocumentOutcome) int {
	n := 0
	for _, doc := range r.Documents {
		if doc.Outcome == outcome {
			n++
		}
	}
	return n
}
