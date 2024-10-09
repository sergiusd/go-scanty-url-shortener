package redis

import (
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"strconv"
	"time"

	redisClient "github.com/gomodule/redigo/redis"
	"github.com/pkg/errors"

	"github.com/sergiusd/go-scanty-url-shortener/internal/model"
)

type redis struct {
	pool *redisClient.Pool
}

func New(host string, port int, password string) (*redis, error) {
	pool := &redisClient.Pool{
		MaxIdle:     10,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redisClient.Conn, error) {
			return redisClient.Dial("tcp", fmt.Sprintf("%s:%d", host, port), redisClient.DialPassword(password))
		},
	}

	return &redis{pool}, nil
}

func getItemKey(id uint64) string {
	return "link:" + strconv.FormatUint(id, 10)
}

func (r *redis) Create(item model.Item) (err error) {
	conn := r.pool.Get()
	defer conn.Close()

	isUsed, err := redisClient.Bool(conn.Do("EXISTS", getItemKey(item.Id)))
	if err != nil {
		return errors.Wrap(err, "Can't get item is used")
	}
	if isUsed {
		return model.ErrItemDuplicated
	}

	redisItem := Item{
		Id:  item.Id,
		URL: item.URL,
	}
	redisItem.ImportExpires(item.Expires)

	_, err = conn.Do("HMSET", redisClient.Args{getItemKey(item.Id)}.AddFlat(redisItem)...)
	if err != nil {
		log.Printf("Error on create item %v: %v", redisItem, err)
		return errors.Wrap(err, "Can't create item")
	}

	if item.Expires != nil {
		_, err = conn.Do("EXPIREAT", getItemKey(item.Id), item.Expires.Unix())
		if err != nil {
			log.Printf("Error on create set expires item %v: %v", item.Expires.Unix(), err)
			return err
		}
	}

	return nil
}

func (r *redis) Find(url string) (uint64, error) {
	return 0, fmt.Errorf("Not implemented")
}

func (r *redis) Load(decodedId uint64) (string, error) {
	conn := r.pool.Get()
	defer conn.Close()

	urlString, err := redisClient.String(conn.Do("HGET", getItemKey(decodedId), "url"))
	if err != nil {
		return "", err
	} else if len(urlString) == 0 {
		return "", model.ErrNoLink
	}

	_, err = conn.Do("HINCRBY", getItemKey(decodedId), "visits", 1)

	return urlString, nil
}

func (r *redis) Close() error {
	return r.pool.Close()
}

func (r *redis) Stat(ctx context.Context) (any, error) {
	return "Not implemented", nil
}
