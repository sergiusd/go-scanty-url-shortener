package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	redisClient "github.com/gomodule/redigo/redis"
	"github.com/pkg/errors"

	"github.com/sergiusd/go-scanty-url-shortener/internal/model"
)

const errorDuplicate = "Duplicate"

const checkAndSetScript = `
local key = KEYS[1]
local id = ARGV[2]
local url = ARGV[3]
local expires = ARGV[4]

local exists = redis.call('EXISTS', key)

if exists == 0 then
    redis.call('HMSET', key, 'id', id, 'url', url)

    if expires then
        redis.call('EXPIREAT', key, expires)
    end

    return "Ok"
else
    return "` + errorDuplicate + `"
end
`

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

	args := []any{
		checkAndSetScript, 1, getItemKey(item.Id), item.Id, item.URL,
	}
	if item.Expires != nil {
		args = append(args, item.Expires.Unix())
	}

	result, err := redisClient.String(conn.Do("EVAL", args...))
	if err != nil {
		return errors.Wrap(err, "Error executing Lua script for check and set item")
	}
	if result == errorDuplicate {
		return model.ErrItemDuplicated
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

	return urlString, nil
}

func (r *redis) Close() error {
	return r.pool.Close()
}

func (r *redis) Stat(ctx context.Context) (any, error) {
	return "Not implemented", nil
}
