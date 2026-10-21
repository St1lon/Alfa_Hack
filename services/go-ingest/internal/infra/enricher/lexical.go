// Package enricher — обогащение новостей. Lexical — fallback внутри Go
// (G1-15): словари отраслей, стран, сырья и риск-тегов из configs/enrichment.yaml.
// Работает без моделей и без токенов — штатный режим, пока py-ml не готов.
package enricher

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"

	"github.com/st1lon/alfa-hack/internal/domain"
)

const (
	// titleWeight — совпадение в заголовке весит больше, чем в теле:
	// заголовок — самая сжатая формулировка события.
	titleWeight = 2
	// maxIndustryHints — сколько отраслей отдаём в okved_hints.
	maxIndustryHints = 3
	// noiseWithoutBusiness — шум новости без внятного бизнес-сигнала.
	noiseWithoutBusiness = 0.9
	// minBodySignal — сколько бизнес-совпадений нужно в теле, если в заголовке их нет.
	minBodySignal = 3
)

// Dictionary — содержимое configs/enrichment.yaml.
type Dictionary struct {
	Version     string        `yaml:"version"`
	Industries  []industryDef `yaml:"industries"`
	Countries   []namedDef    `yaml:"countries"`
	Commodities []namedDef    `yaml:"commodities"`
	RiskTags    []namedDef    `yaml:"risk_tags"`
	Noise       struct {
		Terms []string `yaml:"terms"`
	} `yaml:"noise"`
}

type industryDef struct {
	Code  string   `yaml:"code"`
	Label string   `yaml:"label"`
	Terms []string `yaml:"terms"`
}

type namedDef struct {
	Name  string   `yaml:"name"`
	Terms []string `yaml:"terms"`
}

// matcher — скомпилированный термин словаря.
type matcher struct {
	prefix string // префикс слова
	exact  string // слово целиком
	phrase string // подстрока нормализованного текста
}

type compiledGroup struct {
	key, label string
	matchers   []matcher
}

// Lexical — словарный обогатитель.
type Lexical struct {
	version     string
	industries  []compiledGroup
	countries   []compiledGroup
	commodities []compiledGroup
	riskTags    []compiledGroup
	noise       []matcher
}

// text — новость, подготовленная к поиску терминов.
type text struct {
	title, body           string   // нормализованный текст для фраз
	titleWords, bodyWords []string // слова для префиксов и точных совпадений
}

// LoadDictionary читает словарь и компилирует его.
func LoadDictionary(path string) (*Lexical, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read enrichment dictionary: %w", err)
	}
	var dict Dictionary
	if err = yaml.Unmarshal(raw, &dict); err != nil {
		return nil, fmt.Errorf("unmarshal enrichment dictionary: %w", err)
	}
	return NewLexical(dict)
}

// NewLexical компилирует словарь в обогатитель.
func NewLexical(dict Dictionary) (*Lexical, error) {
	if dict.Version == "" {
		return nil, errors.New("enrichment dictionary: version is required")
	}

	lex := &Lexical{version: dict.Version, noise: compileTerms(dict.Noise.Terms)}
	for _, def := range dict.Industries {
		lex.industries = append(lex.industries, compiledGroup{key: def.Code, label: def.Label, matchers: compileTerms(def.Terms)})
	}
	lex.countries = compileNamed(dict.Countries)
	lex.commodities = compileNamed(dict.Commodities)
	lex.riskTags = compileNamed(dict.RiskTags)
	return lex, nil
}

// Enrich обогащает новость. Ошибок не бывает: словарь уже скомпилирован,
// сигнатура совпадает с py-ml-клиентом, чтобы оба реализовали один порт.
func (l *Lexical) Enrich(_ context.Context, news *domain.News) (*domain.Enrichment, error) {
	doc := prepare(news.Title, news.Body)
	hints, entities, sig := l.analyze(doc)
	sig.titleNoise, sig.bodyNoise = doc.split(l.noise)

	return &domain.Enrichment{
		NewsID:       news.ID,
		OKVEDHints:   hints,
		Entities:     entities,
		NoiseScore:   sig.noiseScore(),
		ModelVersion: l.version,
	}, nil
}

// Classify размечает произвольный текст — например, чанк аналитики ЦМИ — теми же
// отраслями, странами, сырьём и риск-тегами, что и новости. Оценки шума тут нет:
// аналитика банка шумом не бывает.
func (l *Lexical) Classify(text string) ([]domain.OKVEDHint, []domain.Entity) {
	hints, entities, _ := l.analyze(prepare("", text))
	return hints, entities
}

// Version — версия словаря, которой сделана разметка.
func (l *Lexical) Version() string {
	return l.version
}

// analyze находит в тексте отрасли и сущности и считает бизнес-сигнал.
func (l *Lexical) analyze(doc text) ([]domain.OKVEDHint, []domain.Entity, signals) {
	var (
		sig      signals
		entities []domain.Entity
	)

	industryScores := make(map[string]int)
	for _, group := range l.industries {
		title, body := doc.split(group.matchers)
		if title+body > 0 {
			industryScores[group.key] = titleWeight*title + body
			sig.titleBusiness += title
			sig.bodyBusiness += body
		}
	}

	for _, set := range []struct {
		kind   domain.EntityType
		groups []compiledGroup
	}{
		{domain.EntityCountry, l.countries},
		{domain.EntityCommodity, l.commodities},
		{domain.EntityRiskTag, l.riskTags},
	} {
		for _, group := range set.groups {
			title, body := doc.split(group.matchers)
			if title+body == 0 {
				continue
			}
			entities = append(entities, domain.Entity{
				Type: set.kind, Value: group.key, Mentions: titleWeight*title + body,
			})
			// страна сама по себе — не бизнес-сигнал (политика её тоже упоминает)
			if set.kind != domain.EntityCountry {
				sig.titleBusiness += title
				sig.bodyBusiness += body
			}
		}
	}

	return topHints(l.industries, industryScores), entities, sig
}

// signals — счётчики бизнес-сигнала и маркеров шума отдельно по заголовку и телу.
type signals struct {
	titleBusiness, bodyBusiness int
	titleNoise, bodyNoise       int
}

// noiseScore — насколько новость похожа на шум для бизнеса, 0..1.
//
// Тему новости задаёт заголовок, поэтому он решает в первую очередь:
//   - маркеров шума в заголовке не меньше, чем бизнес-слов, — шум
//     («СКА уступил „Автомобилисту“ в матче»: клуб совпал с «автомобилем»,
//     но «матч» говорит о теме точнее);
//   - бизнеса в заголовке нет, а в теле его меньше minBodySignal — тоже шум:
//     политическая новость со случайным «экспортом» в абзаце не про бизнес клиента;
//   - иначе — 1 минус доля бизнес-сигнала среди всех сигналов (заголовок весомее).
func (s signals) noiseScore() float64 {
	switch {
	case s.titleNoise > 0 && s.titleNoise >= s.titleBusiness:
		return 1
	case s.titleBusiness == 0 && s.bodyBusiness < minBodySignal:
		return noiseWithoutBusiness
	}
	business := titleWeight*s.titleBusiness + s.bodyBusiness
	noise := titleWeight*s.titleNoise + s.bodyNoise
	return 1 - float64(business)/float64(business+noise+1)
}

// topHints — до трёх самых сильных отраслей, вес — доля от лидера.
func topHints(groups []compiledGroup, scores map[string]int) []domain.OKVEDHint {
	if len(scores) == 0 {
		return nil
	}
	hints := make([]domain.OKVEDHint, 0, len(scores))
	maxScore := 0
	for _, group := range groups {
		if score, ok := scores[group.key]; ok {
			hints = append(hints, domain.OKVEDHint{Code: group.key, Label: group.label, Weight: float64(score)})
			maxScore = max(maxScore, score)
		}
	}
	sort.SliceStable(hints, func(i, j int) bool { return hints[i].Weight > hints[j].Weight })
	if len(hints) > maxIndustryHints {
		hints = hints[:maxIndustryHints]
	}
	for i := range hints {
		hints[i].Weight /= float64(maxScore)
	}
	return hints
}

func compileNamed(defs []namedDef) []compiledGroup {
	groups := make([]compiledGroup, 0, len(defs))
	for _, def := range defs {
		groups = append(groups, compiledGroup{key: def.Name, label: def.Name, matchers: compileTerms(def.Terms)})
	}
	return groups
}

func compileTerms(terms []string) []matcher {
	matchers := make([]matcher, 0, len(terms))
	for _, term := range terms {
		term = normalize(strings.TrimSpace(term))
		switch {
		case term == "":
			continue
		case strings.HasSuffix(term, "$"):
			matchers = append(matchers, matcher{exact: strings.TrimSuffix(term, "$")})
		case strings.IndexFunc(term, isSeparator) >= 0:
			matchers = append(matchers, matcher{phrase: term})
		default:
			matchers = append(matchers, matcher{prefix: term})
		}
	}
	return matchers
}

func prepare(title, body string) text {
	t := normalize(title)
	b := normalize(body)
	return text{title: t, body: b, titleWords: words(t), bodyWords: words(b)}
}

// split — сколько раз термины группы встретились в заголовке и в теле.
func (t text) split(matchers []matcher) (int, int) {
	title, body := 0, 0
	for _, m := range matchers {
		title += m.count(t.title, t.titleWords)
		body += m.count(t.body, t.bodyWords)
	}
	return title, body
}

func (m matcher) count(normalized string, words []string) int {
	if m.phrase != "" {
		return strings.Count(normalized, m.phrase)
	}
	n := 0
	for _, word := range words {
		if (m.exact != "" && word == m.exact) || (m.prefix != "" && strings.HasPrefix(word, m.prefix)) {
			n++
		}
	}
	return n
}

// normalize — нижний регистр, «ё» → «е», схлопнутые пробелы.
func normalize(s string) string {
	s = strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(s, "ё", "е"), "Ё", "Е"))
	return strings.Join(strings.Fields(s), " ")
}

// words режет текст на слова. Дефис и «/» внутри слова сохраняем:
// «ж/д», «it-компания» — это одно слово.
func words(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '/'
	})
}

func isSeparator(r rune) bool {
	return !unicode.IsLetter(r) && !unicode.IsDigit(r)
}
