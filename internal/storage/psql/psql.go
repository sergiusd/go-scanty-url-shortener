package psql

import (
	"context"
	"fmt"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"
	"github.com/sergiusd/go-scanty-url-shortener/internal/model"
)

type Psql struct {
	ctx  context.Context
	pool *pgxpool.Pool
}

type emptyRow struct{}

func (er emptyRow) Scan(_ ...interface{}) error {
	return nil
}

func New(ctx context.Context, host string, port int, name, user, password string, poolSize int32, timeout time.Duration) (*Psql, error) {
	dbURL := fmt.Sprintf("user=%v password=%v host=%v port=%v dbname=%v sslmode=", user, password, host, port, name)
	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		panic(err)
	}
	if poolSize != 0 {
		config.MaxConns = poolSize
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to connection to database: %v", err))
	}

	onError := func(message string) (*Psql, error) {
		pool.Close()
		return nil, errors.New(fmt.Sprintf(message+": %v", err))
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return onError("Unable to acquire a database connection")
	}
	defer conn.Release()
	if err = migrate(ctx, conn); err != nil {
		return onError("Unable to roll migrations to database")
	}

	storage := &Psql{ctx: ctx, pool: pool}

	return storage, nil
}

func (pg *Psql) exec(sql string, args ...interface{}) error {
	conn, err := pg.pool.Acquire(pg.ctx)
	if err != nil {
		return errors.New(fmt.Sprintf("Unable to acquire a database connection: %v", err))
	}
	defer conn.Release()

	_, err = conn.Exec(pg.ctx, sql, args...)
	return err
}

func (pg *Psql) queryRow(sql string, args ...interface{}) (pgx.Row, error) {
	conn, err := pg.pool.Acquire(pg.ctx)
	if err != nil {
		return emptyRow{}, errors.New(fmt.Sprintf("Unable to acquire a database connection: %v", err))
	}
	defer conn.Release()

	return conn.QueryRow(pg.ctx, sql, args...), nil
}

func (pg *Psql) Create(item model.Item) error {
	err := pg.exec(
		"INSERT INTO links (id, url, expires) VALUES ($1, $2, $3)",
		int64(item.Id), item.URL, item.Expires,
	)
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok {
			if pgErr.Code == "23505" && pgErr.ConstraintName == "links_id_uniq" {
				return model.ErrItemDuplicated
			}
		}
	}
	return err
}

func (pg *Psql) Find(url string) (uint64, error) {
	row, err := pg.queryRow("SELECT id FROM links WHERE url = $1", url)
	if err != nil {
		return 0, errors.Wrapf(err, "Can't find %v", url)
	}
	var id int64
	if err := row.Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, errors.Wrapf(err, "Can't scan id %v", url)
	}
	return uint64(id), nil
}

func (pg *Psql) Load(decodedId uint64) (string, error) {
	var url string
	var expires *time.Time
	row, err := pg.queryRow("SELECT url, expires FROM links WHERE id = $1", int64(decodedId))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", model.ErrNoLink
		}
		return "", errors.Wrapf(err, "Can't select %v", int64(decodedId))
	}
	if err := row.Scan(&url, &expires); err != nil {
		return "", errors.Wrapf(err, "Can't scan url and expires %v", int64(decodedId))
	}
	if expires != nil && expires.Local().UTC().Before(time.Now().UTC()) {
		return "", model.ErrNoLink
	}

	return url, nil
}

func (pg *Psql) Close() error {
	pg.pool.Close()
	return nil
}

func (pg *Psql) ping() (time.Duration, error) {
	conn, err := pg.pool.Acquire(pg.ctx)
	if err != nil {
		return 0, errors.New(fmt.Sprintf("Unable to acquire a database connection: %v", err))
	}
	defer conn.Release()
	t := time.Now()
	row, err := pg.queryRow("SELECT 1")
	result := 0
	if err := row.Scan(&result); err != nil {
		return 0, errors.Wrapf(err, "Can't scan on ping")
	}
	return t.Sub(t), err
}

func (pg *Psql) Stat(ctx context.Context) (any, error) {
	pingDuration, err := pg.ping()
	if err != nil {
		return nil, errors.Wrapf(err, "Can't ping postgres")
	}

	s := pg.pool.Stat()
	return struct {
		PingDuration         time.Duration `json:"pingDuration"`
		AcquireCount         int64         `json:"acquireCount"`
		AcquireDuration      time.Duration `json:"acquireDuration"`
		AcquiredConns        int32         `json:"acquiredConns"`
		CanceledAcquireCount int64         `json:"canceledAcquireCount"`
		EmptyAcquireCount    int64         `json:"emptyAcquireCount"`
		ConstructingConns    int32         `json:"constructingConns"`
		IdleConns            int32         `json:"idleConns"`
		MaxConns             int32         `json:"maxConns"`
		TotalConns           int32         `json:"totalConns"`
	}{
		PingDuration:         pingDuration,
		AcquireCount:         s.AcquireCount(),
		AcquireDuration:      s.AcquireDuration(),
		AcquiredConns:        s.AcquiredConns(),
		CanceledAcquireCount: s.CanceledAcquireCount(),
		EmptyAcquireCount:    s.EmptyAcquireCount(),
		ConstructingConns:    s.ConstructingConns(),
		IdleConns:            s.IdleConns(),
		MaxConns:             s.MaxConns(),
		TotalConns:           s.TotalConns(),
	}, nil
}
