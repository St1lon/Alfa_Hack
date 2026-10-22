package enricher_test

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/st1lon/alfa-hack/internal/domain"
	"github.com/st1lon/alfa-hack/services/go-ingest/internal/infra/enricher"
)

// noiseThreshold — тот же порог, что у оркестратора по умолчанию.
const noiseThreshold = 0.8

// loadRealDictionary берёт словарь из configs/: тест сторожит именно тот
// файл, который правит команда, а не тестовую копию.
func loadRealDictionary(t *testing.T) *enricher.Lexical {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	path := filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "configs", "enrichment.yaml")

	lex, err := enricher.LoadDictionary(path)
	require.NoError(t, err)
	return lex
}

func enrich(t *testing.T, lex *enricher.Lexical, title, body string) *domain.Enrichment {
	t.Helper()
	result, err := lex.Enrich(context.Background(), &domain.News{ID: uuid.New(), Title: title, Body: body})
	require.NoError(t, err)
	return result
}

func entityValues(e *domain.Enrichment, kind domain.EntityType) []string {
	var values []string
	for _, entity := range e.Entities {
		if entity.Type == kind {
			values = append(values, entity.Value)
		}
	}
	return values
}

func hintCodes(e *domain.Enrichment) []string {
	codes := make([]string, 0, len(e.OKVEDHints))
	for _, hint := range e.OKVEDHints {
		codes = append(codes, hint.Code)
	}
	return codes
}

// Заголовки — реальные, из потока, который собрал парсер в Д1–Д2.
func TestLexicalFiltersNoise(t *testing.T) {
	t.Parallel()

	lex := loadRealDictionary(t)
	for _, title := range []string{
		"Сборная России по футболу сыграет с Венесуэлой",
		"Таблица медалей чемпионата мира по спортивной гимнастике - 2026",
		"Губернатор сообщил о трех сбитых беспилотниках над Ленобластью",
		"Замглавы Минздрава Рязанской области погиб в ДТП",
		"Трамп попросил Путина освободить осужденного в РФ американца Хаббарда",
	} {
		t.Run(title, func(t *testing.T) {
			t.Parallel()
			result := enrich(t, lex, title, "")
			require.GreaterOrEqual(t, result.NoiseScore, noiseThreshold, "это не про бизнес клиента")
		})
	}
}

// Ложные срабатывания, найденные на живом прогоне оркестратора в Д2.
func TestLexicalLiveFalsePositives(t *testing.T) {
	t.Parallel()

	lex := loadRealDictionary(t)
	cases := []struct {
		name, title, body string
	}{
		{
			name:  "оптоволокно — не оптовая торговля",
			title: "Связисты наладили связь под Запорожьем, проложив 15 км оптоволокна под огнем ВСУ",
		},
		{
			name:  "сектор Газа — не газ",
			title: "В ЦАХАЛ началось голосование на выборах в парламент",
			body:  "Голосование проходит и среди военнослужащих в секторе Газа.",
		},
		{
			name:  "спорт в заголовке перевешивает случайное слово в теле",
			title: "СКА проиграл первый матч после увольнения Ларионова и прихода Разина",
			body:  "Команда добиралась на матч на автомобиле клуба.",
		},
		{
			name:  "название клуба совпало с отраслью",
			title: "СКА уступил «Автомобилисту» в первом матче под руководством Разина",
		},
		{
			name:  "«Золотой мяч» в теле — не золото",
			title: "Роналду поддержал лидера ПСЖ Кварацхелию после серьезной травмы",
			body:  "Обладатель «Золотого мяча» напомнил, что золотой сезон игрока прервала травма. Золотые годы впереди.",
		},
		{
			name:  "политика с одним случайным словом в теле",
			title: "Стубб назвал «самый сложный» вопрос в украинском урегулировании",
			body:  "Президент Финляндии упомянул и экспорт в разговоре с журналистами.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := enrich(t, lex, tc.title, tc.body)
			require.GreaterOrEqual(t, result.NoiseScore, noiseThreshold)
		})
	}

	// И отдельно: отрасль не должна подхватываться ложно, даже если новость не шум.
	require.NotContains(t, hintCodes(enrich(t, lex, cases[0].title, "")), "46")
	require.NotContains(t, hintCodes(enrich(t, lex, cases[1].title, cases[1].body)), "06")
}

func TestLexicalKeepsBusinessNews(t *testing.T) {
	t.Parallel()

	lex := loadRealDictionary(t)
	cases := []struct {
		title      string
		hint       string
		entityType domain.EntityType
		entity     string
	}{
		{"Нефть подорожала до пятимесячного максимума", "06", domain.EntityCommodity, "нефть"},
		{"Турция получит часть дизеля, который Россия поставит в рамках договоренностей с США", "19", domain.EntityCountry, "ТУРЦИЯ"},
		{"Страны ШОС обсудили создание новых цепочек поставок и организацию совместного банка", "52", domain.EntityRiskTag, "supply_chain"},
		{"ФАС возбудила 70 дел на топливном рынке, выдано 102 предупреждения", "19", domain.EntityCommodity, "нефтепродукты"},
		{"ЦБ оставил курс доллара на 21 октября ниже ₽83", "", domain.EntityRiskTag, "fx_risk"},
		{"Банк России сохранил ключевую ставку на уровне 14%", "64", domain.EntityRiskTag, "rate_risk"},
		// Геополитика, которая прямо бьёт по нефти и фрахту, отсеиваться не должна.
		{"Иран заявил, что военные корабли США отошли от Ормузского пролива на 700 км", "06", domain.EntityCountry, "ИРАН"},
	}

	for _, tc := range cases {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()
			result := enrich(t, lex, tc.title, "")
			require.Less(t, result.NoiseScore, noiseThreshold, "бизнес-новость не должна отсеиваться")
			if tc.hint != "" {
				require.Contains(t, hintCodes(result), tc.hint)
			}
			require.Contains(t, entityValues(result, tc.entityType), tc.entity)
			require.Equal(t, "lexical-v3", result.ModelVersion)
		})
	}
}

func TestLexicalCountriesUseFTSSpelling(t *testing.T) {
	t.Parallel()

	lex := loadRealDictionary(t)
	result := enrich(t, lex,
		"Экспорт стали в Китай и Турцию вырос",
		"Поставки проката в КНР и турецкие порты выросли, а через Дубай идёт реэкспорт в ОАЭ.")

	countries := entityValues(result, domain.EntityCountry)
	// Написание как в FTS_COUNTRY_NAME: Go #2 сравнивает простым равенством.
	require.ElementsMatch(t, []string{"КИТАЙ", "ТУРЦИЯ", "ОАЭ"}, countries)
	require.Contains(t, hintCodes(result), "24")
	require.Contains(t, entityValues(result, domain.EntityRiskTag), "export")
}

func TestLexicalHintsAreRankedAndCapped(t *testing.T) {
	t.Parallel()

	lex := loadRealDictionary(t)
	result := enrich(t, lex,
		"Металлурги нарастили выпуск стали",
		"Металлургические комбинаты, сталелитейные заводы и производители проката, а также банки, "+
			"ритейл, застройщики и нефтяники отчитались за квартал.")

	require.LessOrEqual(t, len(result.OKVEDHints), 3)
	require.Equal(t, "24", result.OKVEDHints[0].Code, "металлургия — главная отрасль новости")
	require.InDelta(t, 1.0, result.OKVEDHints[0].Weight, 1e-9, "вес лидера — 1")
	for _, hint := range result.OKVEDHints[1:] {
		require.Less(t, hint.Weight, 1.0)
	}
}

func TestLexicalExactTermsDoNotOvermatch(t *testing.T) {
	t.Parallel()

	lex := loadRealDictionary(t)
	// «газ$» — только слово целиком: «газета» не про нефть и газ.
	result := enrich(t, lex, "Газета опубликовала интервью", "")
	require.NotContains(t, hintCodes(result), "06")
}

func TestNewLexicalRequiresVersion(t *testing.T) {
	t.Parallel()

	_, err := enricher.NewLexical(enricher.Dictionary{})
	require.Error(t, err)
}
