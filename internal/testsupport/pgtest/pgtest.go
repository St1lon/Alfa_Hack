//go:build integration

// Package pgtest поднимает настоящий Postgres с pgvector в контейнере
// (testcontainers-go) и накатывает все миграции проекта. Общий для всех
// Go-сервисов: интеграционные тесты репозиториев идут против той же схемы,
// что и прод, а не против моков.
package pgtest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	image    = "pgvector/pgvector:pg16"
	database = "alfa"
	user     = "alfa"
	password = "alfa"
	// startupTimeout — сколько ждать готовности Postgres в контейнере.
	startupTimeout = 60 * time.Second
)

// Env — поднятая база с накатанными миграциями.
type Env struct {
	Pool      *pgxpool.Pool
	DSN       string
	container *tcpostgres.PostgresContainer
}

// New поднимает контейнер, накатывает миграции и открывает пул от владельца схемы.
func New(ctx context.Context) (*Env, error) {
	container, err := tcpostgres.Run(ctx, image,
		tcpostgres.WithDatabase(database),
		tcpostgres.WithUsername(user),
		tcpostgres.WithPassword(password),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(startupTimeout),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("start postgres container: %w", err)
	}

	env := &Env{container: container}
	if env.DSN, err = container.ConnectionString(ctx, "sslmode=disable"); err != nil {
		env.Close(ctx)
		return nil, fmt.Errorf("connection string: %w", err)
	}
	if env.Pool, err = pgxpool.New(ctx, env.DSN); err != nil {
		env.Close(ctx)
		return nil, fmt.Errorf("open pool: %w", err)
	}
	if err = env.migrate(ctx); err != nil {
		env.Close(ctx)
		return nil, err
	}
	return env, nil
}

// Close закрывает пул и останавливает контейнер.
func (e *Env) Close(ctx context.Context) {
	if e.Pool != nil {
		e.Pool.Close()
	}
	if e.container != nil {
		_ = e.container.Terminate(ctx)
	}
}

// Truncate очищает таблицы между тестами (справочники не трогаем).
func (e *Env) Truncate(ctx context.Context, tables ...string) error {
	if len(tables) == 0 {
		return nil
	}
	if _, err := e.Pool.Exec(ctx, "TRUNCATE "+strings.Join(tables, ", ")+" RESTART IDENTITY CASCADE"); err != nil {
		return fmt.Errorf("truncate: %w", err)
	}
	return nil
}

// ConnectAs включает вход для роли сервиса и открывает пул от её имени —
// так тесты проверяют права ролей из миграции 00004, как в compose.
func (e *Env) ConnectAs(ctx context.Context, role string) (*pgxpool.Pool, error) {
	const rolePassword = "test"
	if _, err := e.Pool.Exec(ctx, fmt.Sprintf("ALTER ROLE %s LOGIN PASSWORD '%s'", role, rolePassword)); err != nil {
		return nil, fmt.Errorf("enable login for %s: %w", role, err)
	}
	cfg, err := pgxpool.ParseConfig(e.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	cfg.ConnConfig.User = role
	cfg.ConnConfig.Password = rolePassword
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect as %s: %w", role, err)
	}
	return pool, nil
}

// migrate накатывает все *.up.sql по порядку. Через простой протокол pgconn:
// в одной миграции несколько выражений и DO-блоки.
func (e *Env) migrate(ctx context.Context) error {
	dir, err := migrationsDir()
	if err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	if len(files) == 0 {
		return errors.New("no migrations found in " + dir)
	}
	sort.Strings(files)

	conn, err := e.Pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire conn: %w", err)
	}
	defer conn.Release()

	for _, file := range files {
		sql, readErr := os.ReadFile(file)
		if readErr != nil {
			return fmt.Errorf("read %s: %w", file, readErr)
		}
		if _, execErr := conn.Conn().PgConn().Exec(ctx, string(sql)).ReadAll(); execErr != nil {
			return fmt.Errorf("apply %s: %w", filepath.Base(file), execErr)
		}
	}
	return nil
}

// migrationsDir — каталог миграций относительно этого файла, а не рабочей папки теста.
func migrationsDir() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("cannot resolve pgtest source path")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "infra", "db", "postgres", "migrations"), nil
}
