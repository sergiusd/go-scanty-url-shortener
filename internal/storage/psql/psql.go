package psql

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"

	"github.com/sergiusd/go-scanty-url-shortener/internal/model"
)

type psql struct {
	ctx    context.Context
	pool   *pgxpool.Pool
	cancel context.CancelFunc
}

type emptyRow struct{}

func (er emptyRow) Scan(_ ...interface{}) error {
	return nil
}

func New(host string, port int, name, user, password string, timeout time.Duration) (*psql, error) {
	ctx, cancel := context.WithCancel(context.Background())

	dbURL := fmt.Sprintf("user=%v password=%v host=%v port=%v dbname=%v sslmode=", user, password, host, port, name)
	pool, err := pgxpool.Connect(ctx, dbURL)
	if err != nil {
		cancel()
		return nil, errors.New(fmt.Sprintf("Unable to connection to database: %v\n", err))
	}

	onError := func(message string) (*psql, error) {
		pool.Close()
		cancel()
		return nil, errors.New(fmt.Sprintf(message+": %v\n", err))
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return onError("Unable to acquire a database connection")
	}
	defer conn.Release()
	if err = migrate(ctx, conn); err != nil {
		return onError("Unable to roll migrations to database")
	}

	storage := &psql{ctx: ctx, pool: pool, cancel: cancel}

	go startCleanScheduler(storage)

	return storage, nil
}

func (pg *psql) exec(sql string, args ...interface{}) error {
	conn, err := pg.pool.Acquire(pg.ctx)
	if err != nil {
		return errors.New(fmt.Sprintf("Unable to acquire a database connection: %v\n", err))
	}
	defer conn.Release()

	_, err = conn.Exec(pg.ctx, sql, args...)
	return err
}

func (pg *psql) queryRow(sql string, args ...interface{}) (pgx.Row, error) {
	conn, err := pg.pool.Acquire(pg.ctx)
	if err != nil {
		return emptyRow{}, errors.New(fmt.Sprintf("Unable to acquire a database connection: %v\n", err))
	}
	defer conn.Release()

	return conn.QueryRow(pg.ctx, sql, args...), nil
}

func (pg *psql) IsUsed(decodedId uint64) (bool, error) {
	var isUsed bool
	row, err := pg.queryRow("SELECT EXISTS (SELECT id FROM links WHERE id = $1)", int64(decodedId))
	if err != nil {
		log.Printf("Error on check item isUsed %v: %v\n", int64(decodedId), err)
	}
	if err := row.Scan(&isUsed); err != nil {
		log.Printf("Error on scan check item isUsed %v: %v\n", int64(decodedId), err)
	}
	return isUsed, err
}

func (pg *psql) Create(item model.Item) error {
	err := pg.exec(
		"INSERT INTO links (id, url, expires) VALUES ($1, $2, $3)",
		int64(item.Id), item.URL, item.Expires,
	)
	if err != nil {
		log.Printf("Error on create item %v: %v\n", item, err)
	}
	return err
}

func (pg *psql) Load(decodedId uint64) (string, error) {
	var url string
	var expires *time.Time
	row, err := pg.queryRow("SELECT url, expires FROM links WHERE id = $1", int64(decodedId))
	if err != nil {
		log.Printf("Error on load item %v: %v\n", int64(decodedId), err)
		return "", model.ErrNoLink
	}
	if err := row.Scan(&url, &expires); err != nil {
		log.Printf("Error on scan load item %v: %v\n", int64(decodedId), err)
		return "", model.ErrNoLink
	}

	if expires != nil && expires.Local().UTC().Before(time.Now().UTC()) {
		log.Printf("Error on load item %v: expired at %v\n", int64(decodedId), *expires)
		return "", model.ErrNoLink
	}
	err = pg.exec("UPDATE links SET visits = visits + 1 WHERE id = $1", int64(decodedId))
	if err != nil {
		log.Printf("Error on incremet visits %v: %v\n", int64(decodedId), err)
	}

	return url, err
}

func (pg *psql) LoadInfo(decodedId uint64) (model.Item, error) {
	var url string
	var expires *time.Time
	var visits int
	row, err := pg.queryRow("SELECT url, expires, visits FROM links WHERE id = $1", int64(decodedId))
	if err != nil {
		log.Printf("Error on load item %v: %v\n", int64(decodedId), err)
		return model.Item{}, err
	}
	if err := row.Scan(&url, &expires, &visits); err != nil {
		log.Printf("Error on scan load item %v: %v\n", int64(decodedId), err)
		return model.Item{}, err
	}

	return model.Item{Id: decodedId, URL: url, Expires: expires, Visits: visits}, nil
}

func (pg *psql) Close() error {
	pg.cancel()
	pg.pool.Close()
	return nil
}
