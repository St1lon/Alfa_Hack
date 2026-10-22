// Package app содержит общие части composition root сервисов.
package app

import (
	"io"
	"log/slog"
	"os"
	"regexp"
)

// syntheticIDRe — идентификаторы компаний и физлиц из данных кейса:
// U_67, F_64255 (и вариант с префиксом SYNTHETIC_ из описания полей в ТЗ).
var syntheticIDRe = regexp.MustCompile(`\b(?:SYNTHETIC_)?([UF])_\d+\b`)

// NewLogger создаёт структурный JSON-логгер в stdout с заданным уровнем.
func NewLogger(level string) *slog.Logger {
	return NewLoggerTo(os.Stdout, level)
}

// NewLoggerTo — то же, что NewLogger, но в произвольный writer (для тестов).
//
// Идентификаторы компаний и физлиц маскируются во всех строках лога, включая
// сообщение и тексты ошибок (Ограничение 06 кейса: защита чувствительных
// данных). Маскирование на уровне логгера, а не в местах вызова: забыть его
// в новом коде невозможно.
func NewLoggerTo(w io.Writer, level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: lvl, ReplaceAttr: redactAttr})
	return slog.New(handler)
}

// redactAttr маскирует идентификаторы в строковых значениях и ошибках.
// Числа, время и булевы значения идентификаторов не содержат — их не трогаем.
func redactAttr(_ []string, attr slog.Attr) slog.Attr {
	if attr.Value.Kind() == slog.KindString {
		attr.Value = slog.StringValue(RedactIDs(attr.Value.String()))
		return attr
	}
	if err, ok := attr.Value.Any().(error); ok {
		attr.Value = slog.StringValue(RedactIDs(err.Error()))
	}
	return attr
}

// RedactIDs заменяет U_67 → U_***, F_64255 → F_***. Префикс оставляем:
// по логу должно быть видно, что речь о компании или о физлице.
func RedactIDs(s string) string {
	return syntheticIDRe.ReplaceAllString(s, "${1}_***")
}
