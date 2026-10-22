package loadresearch

import (
	"strings"
	"unicode"
)

// ChunkConfig — размеры чанков в символах.
//
// ТЗ-план просит чанки на 300–600 токенов. Токенайзера моделей у нас нет,
// поэтому считаем в символах: на русском тексте у BPE-токенайзеров выходит
// около трёх символов на токен, отсюда цели по умолчанию 900/1350/1800.
type ChunkConfig struct {
	MinRunes    int // меньше — хвост доклеивается к предыдущему чанку
	TargetRunes int // набрали столько — закрываем чанк на границе предложения
	MaxRunes    int // жёсткий потолок, длиннее не бывает
	// MinLetters — чанк, в котором меньше букв, считаем мусором OCR
	// (графики, таблицы-картинки, обложки) и не сохраняем.
	MinLetters int
}

// chunkDraft — чанк до сохранения: текст и страница, с которой он взят.
type chunkDraft struct {
	page int
	text string
}

// runesPerToken — грубая оценка числа символов на токен для русского текста.
const runesPerToken = 3

// minCleanRatio — доля «чистых» слов, при которой строка OCR считается текстом.
const minCleanRatio = 0.6

// shortLineTokens — строки из стольких слов и меньше проверяем по-другому:
// это заголовки вида «Потребление на пике», у них мало слов для статистики.
const shortLineTokens = 3

// minLettersShortLine — минимум букв в короткой строке, чтобы она не была мусором.
const minLettersShortLine = 4

// minAvgWordLen — средняя длина слова, ниже которой строка считается мусором OCR.
const minAvgWordLen = 3.5

// overlapShare — перекрытие не длиннее этой доли потолка чанка (1/4).
const overlapShare = 4

// minWordsForLength — правило средней длины применяем, только когда слов хватает
// для статистики: строку таблицы «ВВП 0,3 1,2» оно зря не отбрасывает.
const minWordsForLength = 3

// estimateTokens — оценка числа токенов для поля research_chunks.tokens.
func estimateTokens(text string) int {
	return (len([]rune(text)) + runesPerToken - 1) / runesPerToken
}

// cleanPage приводит сырой вывод OCR к связному тексту:
// выкидывает мусорные строки, склеивает переносы и строки внутри абзаца.
func cleanPage(raw string) string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")

	var paragraphs []string
	var current []string

	flush := func() {
		if len(current) > 0 {
			paragraphs = append(paragraphs, joinLines(current))
			current = nil
		}
	}

	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			flush()
			continue
		}
		if !isTextLine(line) {
			// Мусорная строка рвёт абзац: по обе стороны от графика — разный текст.
			flush()
			continue
		}
		current = append(current, line)
	}
	flush()

	return strings.Join(paragraphs, "\n\n")
}

// joinLines склеивает строки одного абзаца. Перенос «слово-» + строчная буква
// склеивается без пробела: так OCR рвёт слова на границе строки.
func joinLines(lines []string) string {
	var builder strings.Builder
	for i, line := range lines {
		if i > 0 {
			prev := lines[i-1]
			if strings.HasSuffix(prev, "-") && startsLower(line) {
				builder.WriteString(line)
				continue
			}
			builder.WriteByte(' ')
		}
		builder.WriteString(line)
	}
	return strings.Join(strings.Fields(builder.String()), " ")
}

// isTextLine — эвристика «строка похожа на текст, а не на мусор OCR».
// Мусор с обложек и графиков бывает двух видов: обрывки со знаками и заглавной
// посреди слова («ноНо-ВЙ ЧА]-[») и цепочки коротких «слов» из заглавных
// («Вар НАЙ НА АНА НАНО»). Первое ловит доля чистых слов, второе — средняя
// длина слова: у связного русского текста она около пяти букв.
func isTextLine(line string) bool {
	tokens := strings.Fields(line)
	if len(tokens) == 0 {
		return false
	}

	stats := lineStatsOf(tokens)

	if len(tokens) <= shortLineTokens {
		return stats.letters >= minLettersShortLine && stats.clean == len(tokens)
	}

	if float64(stats.clean)/float64(len(tokens)) < minCleanRatio {
		return false
	}
	if stats.words >= minWordsForLength && float64(stats.wordLetters)/float64(stats.words) < minAvgWordLen {
		return false
	}
	return true
}

// lineStats — счётчики по словам строки.
type lineStats struct {
	clean       int // чистых токенов (слов и чисел)
	words       int // чистых буквенных слов
	wordLetters int // букв в чистых словах
	letters     int // букв во всей строке
}

func lineStatsOf(tokens []string) lineStats {
	var stats lineStats
	for _, token := range tokens {
		for _, r := range token {
			if unicode.IsLetter(r) {
				stats.letters++
			}
		}
		if !isCleanToken(token) {
			continue
		}
		stats.clean++
		if letters := countLetters(token); letters > 0 {
			stats.words++
			stats.wordLetters += letters
		}
	}
	return stats
}

func countLetters(s string) int {
	n := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			n++
		}
	}
	return n
}

// isCleanToken — слово из букв (без заглавной посреди слова) или число вида 14,0% / 16.3.
func isCleanToken(token string) bool {
	token = strings.Trim(token, "«»\"'()[],.;:!?—–-…")
	if token == "" {
		return false
	}
	if isNumber(token) {
		return true
	}

	letters := 0
	var prev rune
	for i, r := range []rune(token) {
		switch {
		case unicode.IsLetter(r):
			letters++
			if i > 0 && unicode.IsLower(prev) && unicode.IsUpper(r) {
				return false // «ноНо», «пНЕ» — типичный мусор OCR
			}
		case r == '-' || r == '/':
			// составные слова: «пресс-релиз», «н/н»
		default:
			return false
		}
		prev = r
	}
	return letters >= 2 || (letters == 1 && unicode.IsLower(prev))
}

func isNumber(token string) bool {
	digits := 0
	for _, r := range token {
		switch {
		case unicode.IsDigit(r):
			digits++
		case r == ',' || r == '.' || r == '%' || r == '+' || r == '-':
		default:
			return false
		}
	}
	return digits > 0
}

func startsLower(s string) bool {
	for _, r := range s {
		return unicode.IsLower(r)
	}
	return false
}

// splitChunks режет очищенный текст одной страницы на чанки по границам
// предложений. Чанк никогда не пересекает границу страницы — иначе
// ссылка «документ, стр. N» в карточке инсайта стала бы неправдой — и никогда
// не длиннее cfg.MaxRunes. Соседние чанки перекрываются последним
// предложением, если оно короткое, чтобы мысль на стыке не терялась при поиске.
func splitChunks(page int, text string, cfg ChunkConfig) []chunkDraft {
	overlapCap := cfg.MaxRunes / overlapShare
	// -1 на пробел между перекрытием и предложением: вместе они не длиннее потолка
	sentences := splitLong(splitSentences(text), cfg.MaxRunes-overlapCap-1)
	if len(sentences) == 0 {
		return nil
	}

	var (
		chunks  []chunkDraft
		current []string
		size    int
		fresh   int // предложений в текущем чанке сверх перекрытия
	)

	// closeChunk сохраняет текущий чанк; короткое последнее предложение
	// переходит в следующий как перекрытие. Длинное не переносим: вместе
	// со следующим предложением оно пробило бы потолок.
	closeChunk := func() {
		chunks = append(chunks, chunkDraft{page: page, text: strings.Join(current, " ")})
		last := current[len(current)-1]
		current, size, fresh = nil, 0, 0
		if runeLen(last) <= overlapCap {
			current, size = []string{last}, runeLen(last)+1
		}
	}

	for _, sentence := range sentences {
		length := runeLen(sentence) + 1
		if fresh > 0 && size+length > cfg.MaxRunes {
			closeChunk()
		}
		current = append(current, sentence)
		size += length
		fresh++
		if size >= cfg.TargetRunes {
			closeChunk()
		}
	}

	if fresh > 0 {
		chunks = appendTail(chunks, current, size, page, cfg)
	}
	return dropNoise(chunks, cfg.MinLetters)
}

// appendTail сохраняет остаток страницы. Короткий хвост доклеивается к прошлому
// чанку без повтора перекрытия — но только если склейка влезает в потолок.
func appendTail(chunks []chunkDraft, tail []string, size, page int, cfg ChunkConfig) []chunkDraft {
	if len(chunks) > 0 && size < cfg.MinRunes {
		prev := &chunks[len(chunks)-1]
		rest := tail
		if strings.HasSuffix(prev.text, rest[0]) {
			rest = rest[1:]
		}
		if len(rest) == 0 {
			return chunks
		}
		merged := prev.text + " " + strings.Join(rest, " ")
		if runeLen(merged) <= cfg.MaxRunes {
			prev.text = merged
			return chunks
		}
	}
	return append(chunks, chunkDraft{page: page, text: strings.Join(tail, " ")})
}

// splitLong режет «предложения» длиннее limit по словам. Такие получаются
// из шапок таблиц и подписей, где OCR не видит ни одной точки.
func splitLong(sentences []string, limit int) []string {
	out := make([]string, 0, len(sentences))
	for _, sentence := range sentences {
		if runeLen(sentence) <= limit {
			out = append(out, sentence)
			continue
		}
		var piece []string
		size := 0
		for _, word := range strings.Fields(sentence) {
			length := runeLen(word) + 1
			if size > 0 && size+length > limit {
				out = append(out, strings.Join(piece, " "))
				piece, size = nil, 0
			}
			piece = append(piece, word)
			size += length
		}
		if len(piece) > 0 {
			out = append(out, strings.Join(piece, " "))
		}
	}
	return out
}

func runeLen(s string) int {
	return len([]rune(s))
}

func dropNoise(chunks []chunkDraft, minLetters int) []chunkDraft {
	kept := chunks[:0]
	for _, chunk := range chunks {
		letters := 0
		for _, r := range chunk.text {
			if unicode.IsLetter(r) {
				letters++
			}
		}
		if letters >= minLetters {
			kept = append(kept, chunk)
		}
	}
	return kept
}

// splitSentences делит текст на предложения. Граница — точка, «!» или «?»,
// за которыми идёт пробел и заглавная буква, цифра или кавычка; абзацы тоже
// граница. Сокращения вида «2026 г. рост» не режутся: за точкой строчная буква.
func splitSentences(text string) []string {
	var sentences []string
	for _, paragraph := range strings.Split(text, "\n\n") {
		runes := []rune(strings.TrimSpace(paragraph))
		start := 0
		for i := range runes {
			if !isSentenceEnd(runes, i) {
				continue
			}
			if sentence := strings.TrimSpace(string(runes[start : i+1])); sentence != "" {
				sentences = append(sentences, sentence)
			}
			start = i + 1
		}
		if tail := strings.TrimSpace(string(runes[start:])); tail != "" {
			sentences = append(sentences, tail)
		}
	}
	return sentences
}

func isSentenceEnd(runes []rune, i int) bool {
	r := runes[i]
	if r != '.' && r != '!' && r != '?' {
		return false
	}
	if i+2 >= len(runes) || runes[i+1] != ' ' {
		return false
	}
	next := runes[i+2]
	return unicode.IsUpper(next) || unicode.IsDigit(next) || next == '«' || next == '"'
}
