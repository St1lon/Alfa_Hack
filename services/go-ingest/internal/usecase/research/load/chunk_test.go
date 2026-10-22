package loadresearch

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var testChunking = ChunkConfig{MinRunes: 120, TargetRunes: 200, MaxRunes: 300, MinLetters: 40}

func TestCleanPageDropsCoverGarbage(t *testing.T) {
	t.Parallel()

	// Реальный вывод OCR с обложки «Тактического взгляда на 3-й квартал»:
	// заголовок поверх фотографии превращается в мусор.
	raw := strings.Join([]string{
		"Долговой рынок",
		"Г. ааНа За пНЕ НН н ноНо-ВЙ ЧА]-[.;';ЬД:‘ ч ”Э. ч ра еег ЧГ оГ, 1У АНИ",
		"и яя 4 ВнЪ° Ъ, Вар оЩ НАЙ НА АНА НАНО",
		"*",
		"‚ Время собирать камни",
	}, "\n")

	cleaned := cleanPage(raw)

	require.Contains(t, cleaned, "Долговой рынок")
	require.Contains(t, cleaned, "Время собирать камни")
	require.NotContains(t, cleaned, "ноНо")
	require.NotContains(t, cleaned, "ВнЪ")
}

func TestCleanPageJoinsWrappedLinesAndHyphens(t *testing.T) {
	t.Parallel()

	raw := "ЦБ РФ принял решение взять паузу и оставить ставку неизменной. Формально сигнал пресс-\n" +
		"релиза остался нейтральным, однако тональность комментария стала свидетельст-\n" +
		"вующей об ужесточении.\n\n" +
		"Новый абзац про инфляцию."

	cleaned := cleanPage(raw)

	paragraphs := strings.Split(cleaned, "\n\n")
	require.Len(t, paragraphs, 2)
	require.Contains(t, paragraphs[0], "пресс-релиза остался")
	require.Contains(t, paragraphs[0], "свидетельст-вующей")
	require.NotContains(t, paragraphs[0], "\n")
	require.Equal(t, "Новый абзац про инфляцию.", paragraphs[1])
}

func TestIsTextLineKeepsHeadingsAndTables(t *testing.T) {
	t.Parallel()

	require.True(t, isTextLine("Потребление на пике"))
	require.True(t, isTextLine("Ключевая ставка ЦБ РФ"))
	require.True(t, isTextLine("ВВП 0,3 1,2 1,0 14,0%"), "строка таблицы с цифрами — это данные")
	require.True(t, isTextLine("Рост ВВП составил всего 0,3% за 1П26 несмотря на поддержку"))
	require.False(t, isTextLine("*"))
	require.False(t, isTextLine("3 ‚ АВГУСТ 2026°"), "обрывок обложки")
}

func TestSplitSentencesKeepsAbbreviations(t *testing.T) {
	t.Parallel()

	sentences := splitSentences("Данные за 2026 г. оставались сильными. Ставка 14,0%! Что дальше? 3 месяца паузы.")

	require.Equal(t, []string{
		"Данные за 2026 г. оставались сильными.",
		"Ставка 14,0%!",
		"Что дальше?",
		"3 месяца паузы.",
	}, sentences)
}

func TestSplitChunksRespectsSizeAndOverlaps(t *testing.T) {
	t.Parallel()

	// Разные предложения, иначе проверка перекрытия выполнялась бы всегда.
	sentences := make([]string, 0, 20)
	for i := range 20 {
		sentences = append(sentences, fmt.Sprintf("Тезис номер %d про ставку и инфляцию.", i+1))
	}

	chunks := splitChunks(7, strings.Join(sentences, " "), testChunking)

	require.GreaterOrEqual(t, len(chunks), 3)
	for i, chunk := range chunks {
		require.Equal(t, 7, chunk.page, "чанк не должен терять страницу")
		require.LessOrEqual(t, len([]rune(chunk.text)), testChunking.MaxRunes, "чанк %d длиннее потолка", i)
		if i == 0 {
			continue
		}
		prevSentences := splitSentences(chunks[i-1].text)
		last := prevSentences[len(prevSentences)-1]
		require.True(t, strings.HasPrefix(chunk.text, last),
			"чанк %d должен начинаться с последнего предложения предыдущего: %q", i, last)
	}

	// Ни одно предложение не потеряно при нарезке.
	joined := strings.Join(chunkTexts(chunks), " ")
	for _, sentence := range sentences {
		require.Contains(t, joined, sentence)
	}
}

func TestSplitChunksMergesShortTail(t *testing.T) {
	t.Parallel()

	long := "Банк России сохранил ключевую ставку на уровне четырнадцати процентов годовых, " +
		"сославшись на ускорение устойчивой инфляции и сильный рынок труда."
	text := long + " " + long + " Пауза."

	chunks := splitChunks(1, text, testChunking)

	require.NotEmpty(t, chunks)
	last := chunks[len(chunks)-1].text
	require.True(t, strings.HasSuffix(last, "Пауза."), "короткий хвост доклеен к последнему чанку")
	require.Equal(t, 1, strings.Count(strings.Join(chunkTexts(chunks), " "), "Пауза."), "хвост не продублирован")
}

func TestSplitChunksDropsNoise(t *testing.T) {
	t.Parallel()

	require.Empty(t, splitChunks(1, "Рис. 1. 2024 2025 2026", testChunking), "подпись к графику — не чанк")
	require.Empty(t, splitChunks(1, "", testChunking))
}

func TestEstimateTokens(t *testing.T) {
	t.Parallel()

	require.Equal(t, 0, estimateTokens(""))
	require.Equal(t, 1, estimateTokens("абв"))
	require.Equal(t, 2, estimateTokens("абвг"))
}

func chunkTexts(chunks []chunkDraft) []string {
	texts := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		texts = append(texts, chunk.text)
	}
	return texts
}

func TestSplitChunksNeverExceedsMax(t *testing.T) {
	t.Parallel()

	// Шапка таблицы из OCR: длинная «фраза» без единой точки.
	noPunct := strings.TrimSpace(strings.Repeat("Корпоративные новости рейтинг Эксперт РА прогноз стабильный ", 12))
	// Хвост чуть короче минимума после почти полного чанка — склейка не должна пробить потолок.
	sentence := "Регулятор отметил ускорение роста цен в устойчивых компонентах инфляции до пяти процентов."
	nearFull := strings.TrimSpace(strings.Repeat(sentence+" ", 3))
	tail := "Короткий хвост страницы про паузу в цикле снижения ставки и жесткую риторику."

	for name, text := range map[string]string{
		"длинная фраза без точек": noPunct,
		"хвост после полного":     nearFull + " " + tail,
		"смесь":                   nearFull + " " + noPunct + " " + tail,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			chunks := splitChunks(1, text, testChunking)
			require.NotEmpty(t, chunks)
			for i, chunk := range chunks {
				require.LessOrEqual(t, len([]rune(chunk.text)), testChunking.MaxRunes, "чанк %d длиннее потолка", i)
			}
		})
	}
}
