package psql

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v4"

	"github.com/sergiusd/go-scanty-url-shortener/internal/model"
)

type psql struct {
	ctx    context.Context
	conn   *pgx.Conn
	cancel context.CancelFunc
}

func New(host string, port int, name, user, password string, timeout time.Duration) (*psql, error) {
	ctx, cancel := context.WithCancel(context.Background())
	conf, err := pgx.ParseConfig(fmt.Sprintf("user=%v password=%v host=%v port=%v dbname=%v sslmode=",
		user, password, host, port, name))
	if err != nil {
		cancel()
		return nil, err
	}
	conn, err := pgx.ConnectConfig(ctx, conf)
	if err != nil {
		cancel()
		return nil, errors.New(fmt.Sprintf("Unable to connect to database: %v\n", err))
	}
	if err = migrate(ctx, conn); err != nil {
		cancel()
		return nil, errors.New(fmt.Sprintf("Unable to roll migrations to database: %v\n", err))
	}

	storage := &psql{ctx: ctx, conn: conn, cancel: cancel}

	go startCleanScheduler(storage)

	return storage, nil
}

func (pg *psql) IsUsed(decodedId uint64) (bool, error) {
	var isUsed bool
	err := pg.conn.QueryRow(pg.ctx, "SELECT EXISTS (SELECT id FROM links WHERE id = $1)", int64(decodedId)).Scan(&isUsed)
	if err != nil {
		log.Printf("Error on check item isUsed %v: %v\n", int64(decodedId), err)
	}
	return isUsed, err
}

func (pg *psql) Create(item model.Item) error {
	_, err := pg.conn.Exec(pg.ctx, "INSERT INTO links (id, url, expires) VALUES ($1, $2, $3)",
		int64(item.Id), item.URL, item.Expires)
	if err != nil {
		log.Printf("Error on create item %v: %v\n", item, err)
	}
	return err
}

func (pg *psql) Load(decodedId uint64) (string, error) {
	var url string
	var expires *time.Time
	if err := pg.conn.QueryRow(pg.ctx, "SELECT url, expires FROM links WHERE id = $1", int64(decodedId)).
		Scan(&url, &expires); err != nil {
		log.Printf("Error on load item %v: %v\n", int64(decodedId), err)
		return "", model.ErrNoLink
	}
	if expires != nil && expires.Local().UTC().Before(time.Now().UTC()) {
		log.Printf("Error on load item %v: expired at %v\n", int64(decodedId), *expires)
		return "", model.ErrNoLink
	}
	if _, err := pg.conn.Exec(pg.ctx, "UPDATE links SET visits = visits + 1 WHERE id = $1", int64(decodedId)); err != nil {
		log.Printf("Error on incremet visits %v: %v\n", int64(decodedId), err)
	}

	return url, nil
}

func (pg *psql) LoadInfo(decodedId uint64) (model.Item, error) {
	var url string
	var expires *time.Time
	var visits int
	if err := pg.conn.QueryRow(pg.ctx, "SELECT url, expires, visits FROM links WHERE id = $1", int64(decodedId)).
		Scan(&url, &expires, &visits); err != nil {
		log.Printf("Error on load item %v: %v\n", int64(decodedId), err)
		return model.Item{}, err
	}

	return model.Item{Id: decodedId, URL: url, Expires: expires, Visits: visits}, nil
}

func (pg *psql) Close() error {
	pg.cancel()
	return pg.conn.Close(pg.ctx)
}
