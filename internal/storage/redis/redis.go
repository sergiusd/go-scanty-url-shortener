package redis

import (
	"fmt"
	"log"
	"strconv"
	"time"

	redisClient "github.com/gomodule/redigo/redis"

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

func (r *redis) IsUsed(id uint64) (bool, error) {
	conn := r.pool.Get()
	defer conn.Close()

	return redisClient.Bool(conn.Do("EXISTS", getItemKey(id)))
}

func (r *redis) Create(item model.Item) error {
	conn := r.pool.Get()
	defer conn.Close()

	redisItem := Item{
		Id:  item.Id,
		URL: item.URL,
	}
	redisItem.ImportExpires(item.Expires)

	_, err := conn.Do("HMSET", redisClient.Args{getItemKey(item.Id)}.AddFlat(redisItem)...)
	if err != nil {
		log.Printf("Error on create item %v: %v\n", redisItem, err)
		return err
	}

	if item.Expires != nil {
		_, err = conn.Do("EXPIREAT", getItemKey(item.Id), item.Expires.Unix())
		if err != nil {
			log.Printf("Error on create set expires item %v: %v\n", item.Expires.Unix(), err)
			return err
		}
	}

	return nil
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

func (r *redis) LoadInfo(decodedId uint64) (model.Item, error) {
	conn := r.pool.Get()
	defer conn.Close()

	values, err := redisClient.Values(conn.Do("HGETALL", getItemKey(decodedId)))
	if err != nil {
		return model.Item{}, err
	} else if len(values) == 0 {
		return model.Item{}, model.ErrNoLink
	}
	var redisItem Item
	err = redisClient.ScanStruct(values, &redisItem)
	if err != nil {
		return model.Item{}, err
	}

	return model.Item{
		Id:      decodedId,
		URL:     redisItem.URL,
		Expires: redisItem.ExportExpires(),
		Visits:  redisItem.Visits,
	}, nil
}

func (r *redis) Close() error {
	return r.pool.Close()
}
