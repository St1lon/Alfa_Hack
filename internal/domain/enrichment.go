package domain

import "github.com/google/uuid"

// EntityType — вид сущности, извлечённой из новости.
type EntityType string

// Виды сущностей. По ним Go #2 сопоставляет новость с признаками компании:
// страна — с FTS_COUNTRY_NAME, сырьё и ОКВЭД — с MAINOKVED_CODE, риск-теги —
// с продуктовыми метками (кредит → ставка, ВЭД → курс).
const (
	EntityCountry   EntityType = "COUNTRY"   // КИТАЙ, ТУРЦИЯ — как в данных ФТС
	EntityCommodity EntityType = "COMMODITY" // сталь, нефть, пшеница
	EntityRiskTag   EntityType = "RISK_TAG"  // rate_risk, fx_risk, trade_restriction...
)

// Entity — сущность, найденная в тексте новости.
type Entity struct {
	Type     EntityType
	Value    string
	Mentions int
}

// OKVEDHint — подсказка, к какой отрасли (группе ОКВЭД) относится новость.
type OKVEDHint struct {
	Code   string // группа ОКВЭД: «24» — металлургия, «49» — сухопутный транспорт
	Label  string
	Weight float64 // 0..1, доля от самой сильной отрасли в новости
}

// Enrichment — результат обогащения новости (таблица news_enrichment).
// Его выдаёт py-ml, а пока он не готов — лексический fallback внутри Go.
type Enrichment struct {
	NewsID     uuid.UUID
	Embedding  []float32 // nil, если эмбеддинга нет (fallback-режим)
	Entities   []Entity
	OKVEDHints []OKVEDHint
	// NoiseScore — 0..1, насколько новость похожа на шум для бизнеса
	// (спорт, светская хроника, происшествия). Выше порога — FILTERED.
	NoiseScore   float64
	ModelVersion string
}
