package domain_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/st1lon/alfa-hack/internal/domain"
)

func TestCanonicalURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "пустая ссылка — вброс новости текстом",
			raw:  "",
			want: "",
		},
		{
			name: "www, регистр и фрагмент отбрасываются",
			raw:  "HTTPS://WWW.Interfax.ru/business/123#top",
			want: "https://interfax.ru/business/123",
		},
		{
			name: "трекинговые параметры отбрасываются, остальные сортируются",
			raw:  "https://rbc.ru/news?utm_source=tg&b=2&a=1&yclid=99",
			want: "https://rbc.ru/news?a=1&b=2",
		},
		{
			name: "хвостовой слеш не создаёт второй URL",
			raw:  "https://tass.ru/ekonomika/555/",
			want: "https://tass.ru/ekonomika/555",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := domain.CanonicalURL(tc.raw)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestCanonicalURLRejectsNonHTTP(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{"ftp://example.com/a", "javascript:alert(1)", "not a url at all"} {
		_, err := domain.CanonicalURL(raw)
		require.ErrorIs(t, err, domain.ErrInvalidNewsURL, raw)
	}
}

func TestTextHashIgnoresCosmetics(t *testing.T) {
	t.Parallel()

	// Перепечатка с другой пунктуацией и регистром — тот же сюжет, тот же хэш.
	first := domain.TextHash("Ставка ЦБ — 14%", "Совет директоров  сохранил ключевую ставку.")
	second := domain.TextHash("ставка цб 14%", "Совет директоров сохранил ключевую ставку")
	require.Equal(t, first, second)

	other := domain.TextHash("Ставка ЦБ — 14%", "Совет директоров снизил ключевую ставку.")
	require.NotEqual(t, first, other)
}

func TestURLHashEmptyStaysEmpty(t *testing.T) {
	t.Parallel()

	require.Empty(t, domain.URLHash(""))
	require.Len(t, domain.URLHash("https://rbc.ru/news"), 64)
}

func TestPromptHashDependsOnEveryPart(t *testing.T) {
	t.Parallel()

	base := domain.PromptHash("gemma-4-26b-a4b", "classify", "текст новости")
	otherModel := domain.PromptHash("qwen3.6-35b-a3b", "classify", "текст новости")
	otherPrompt := domain.PromptHash("gemma-4-26b-a4b", "classify", "другой текст")

	require.NotEqual(t, base, otherModel)
	require.NotEqual(t, base, otherPrompt)
}
