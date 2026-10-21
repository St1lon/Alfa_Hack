package app_test

import (
	"bytes"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/st1lon/alfa-hack/internal/app"
)

func TestRedactIDs(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"компания U_67 не найдена":           "компания U_*** не найдена",
		"/v1/companies/U_146717":             "/v1/companies/U_***",
		"связь F_64255 → U_40653":            "связь F_*** → U_***",
		"SYNTHETIC_U_000417 из примера в ТЗ": "U_*** из примера в ТЗ",
		"обычный текст без идентификаторов":  "обычный текст без идентификаторов",
		"UF_67 и U_ABC не трогаем":           "UF_67 и U_ABC не трогаем",
	}
	for in, want := range cases {
		require.Equal(t, want, app.RedactIDs(in), in)
	}
}

func TestLoggerMasksIDsEverywhere(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	log := app.NewLoggerTo(&buf, "info")

	log.Error("score failed for U_67",
		slog.String("company", "U_67"),
		slog.Any("error", errors.New("agent: company U_67 timeout")),
		slog.Group("pair", slog.String("person", "F_64255")),
		slog.Int("attempt", 3),
	)

	out := buf.String()
	require.NotContains(t, out, "U_67", "идентификатор компании не должен попасть в лог")
	require.NotContains(t, out, "F_64255")
	require.Contains(t, out, "U_***")
	require.Contains(t, out, "F_***")
	require.Contains(t, out, `"attempt":3`, "числа не трогаем")
}

func TestLoggerRespectsLevel(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	log := app.NewLoggerTo(&buf, "warn")
	log.Info("не должно попасть")
	log.Warn("должно попасть")

	require.NotContains(t, buf.String(), "не должно попасть")
	require.Contains(t, buf.String(), "должно попасть")
}
