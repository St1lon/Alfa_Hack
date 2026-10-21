// Package ocr извлекает текст из PDF-сканов: pdftoppm рендерит страницу
// в картинку, tesseract распознаёт её. Аналитика ЦМИ в кейсе — сканы без
// текстового слоя, поэтому обычный pdftotext возвращает по ним ноль символов.
package ocr

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"golang.org/x/sync/errgroup"
)

// cacheFilePerm — права на файлы кэша распознанных страниц.
const cacheFilePerm = 0o600

// cacheDirPerm — права на каталоги кэша.
const cacheDirPerm = 0o750

// maxStderr — сколько байт stderr внешней утилиты попадает в текст ошибки.
const maxStderr = 300

var pagesRe = regexp.MustCompile(`(?m)^Pages:\s+(\d+)`)

// Config — параметры распознавания.
type Config struct {
	DPI         int
	Lang        string
	Workers     int
	PageTimeout time.Duration
	// CacheDir — куда складывать распознанный текст страниц. Пусто — без кэша.
	// OCR одной страницы занимает 10–30 секунд, поэтому кэш экономит минуты
	// на каждом перезапуске загрузчика.
	CacheDir string
}

// Extractor распознаёт PDF постранично.
type Extractor struct {
	cfg Config
	log *slog.Logger
}

// New создаёт экстрактор текста.
func New(cfg Config, log *slog.Logger) *Extractor {
	return &Extractor{cfg: cfg, log: log}
}

// ExtractPages возвращает текст каждой страницы документа; элемент i — страница i+1.
// Страницы распознаются параллельно, не больше cfg.Workers одновременно.
func (e *Extractor) ExtractPages(ctx context.Context, path string) ([]string, error) {
	count, err := e.pageCount(ctx, path)
	if err != nil {
		return nil, err
	}

	cacheDir, err := e.cacheDirFor(path)
	if err != nil {
		return nil, err
	}

	tmp, err := os.MkdirTemp("", "ocr-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	texts := make([]string, count)
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(e.cfg.Workers)

	for page := 1; page <= count; page++ {
		group.Go(func() error {
			text, pageErr := e.page(groupCtx, path, page, tmp, cacheDir)
			if pageErr != nil {
				return fmt.Errorf("page %d: %w", page, pageErr)
			}
			texts[page-1] = text
			return nil
		})
	}

	if err = group.Wait(); err != nil {
		return nil, fmt.Errorf("ocr %s: %w", filepath.Base(path), err)
	}
	return texts, nil
}

// page распознаёт одну страницу или берёт её из кэша.
func (e *Extractor) page(ctx context.Context, path string, page int, tmp, cacheDir string) (string, error) {
	cacheFile := ""
	if cacheDir != "" {
		cacheFile = filepath.Join(cacheDir, fmt.Sprintf("%03d.txt", page))
		if cached, err := os.ReadFile(cacheFile); err == nil {
			return string(cached), nil
		}
	}

	pageCtx, cancel := context.WithTimeout(ctx, e.cfg.PageTimeout)
	defer cancel()

	started := time.Now()
	prefix := filepath.Join(tmp, fmt.Sprintf("p%03d", page))
	pageArg := strconv.Itoa(page)

	if _, err := run(pageCtx, "pdftoppm",
		"-r", strconv.Itoa(e.cfg.DPI), "-f", pageArg, "-l", pageArg, "-png", "-singlefile", path, prefix,
	); err != nil {
		return "", fmt.Errorf("render: %w", err)
	}

	image := prefix + ".png"
	defer func() { _ = os.Remove(image) }()

	text, err := run(pageCtx, "tesseract", image, "stdout", "-l", e.cfg.Lang)
	if err != nil {
		return "", fmt.Errorf("recognize: %w", err)
	}

	if cacheFile != "" {
		if writeErr := os.WriteFile(cacheFile, text, cacheFilePerm); writeErr != nil {
			e.log.Warn("write ocr cache failed", slog.String("error", writeErr.Error()))
		}
	}

	e.log.Debug("ocr page done",
		slog.String("file", filepath.Base(path)),
		slog.Int("page", page),
		slog.Int64("duration_ms", time.Since(started).Milliseconds()),
	)
	return string(text), nil
}

// pageCount читает число страниц через pdfinfo.
func (e *Extractor) pageCount(ctx context.Context, path string) (int, error) {
	out, err := run(ctx, "pdfinfo", path)
	if err != nil {
		return 0, fmt.Errorf("pdfinfo: %w", err)
	}
	match := pagesRe.FindSubmatch(out)
	if match == nil {
		return 0, errors.New("pdfinfo: page count not found")
	}
	count, err := strconv.Atoi(string(match[1]))
	if err != nil || count <= 0 {
		return 0, fmt.Errorf("pdfinfo: bad page count %q", match[1])
	}
	return count, nil
}

// cacheDirFor — каталог кэша конкретного файла. В имени размер файла, DPI и
// языки: поменялся файл или параметры распознавания — кэш не подхватится.
func (e *Extractor) cacheDirFor(path string) (string, error) {
	if e.cfg.CacheDir == "" {
		return "", nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", path, err)
	}
	dir := filepath.Join(e.cfg.CacheDir,
		fmt.Sprintf("%s-%d-%d-%s", filepath.Base(path), info.Size(), e.cfg.DPI, e.cfg.Lang))
	if err = os.MkdirAll(dir, cacheDirPerm); err != nil {
		return "", fmt.Errorf("create cache dir: %w", err)
	}
	return dir, nil
}

// run запускает внешнюю утилиту и возвращает stdout. В ошибку кладём хвост
// stderr — без него «exit status 1» от tesseract ничего не объясняет.
func run(ctx context.Context, name string, args ...string) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := stderr.Bytes()
		if len(msg) > maxStderr {
			msg = msg[len(msg)-maxStderr:]
		}
		return nil, fmt.Errorf("%s: %w: %s", name, err, bytes.TrimSpace(msg))
	}
	return stdout.Bytes(), nil
}
