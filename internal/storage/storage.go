package storage

import (
	"log"
	"math/rand"
	"time"

	"github.com/sergiusd/go-scanty-url-shortener/internal/base62"
	"github.com/sergiusd/go-scanty-url-shortener/internal/config"
	"github.com/sergiusd/go-scanty-url-shortener/internal/model"
	"github.com/sergiusd/go-scanty-url-shortener/internal/storage/psql"
	"github.com/sergiusd/go-scanty-url-shortener/internal/storage/redis"
)

type client interface {
	IsUsed(id uint64) (bool, error)
	Create(item model.Item) error
	Load(decodedId uint64) (string, error)
	LoadInfo(decodedId uint64) (model.Item, error)
	Close() error
}

func New(conf config.Storage) (*storage, error) {
	var err error
	var client client
	switch conf.Kind {
	case "redis":
		log.Printf("Use redis on %v:%v", conf.Redis.Host, conf.Redis.Port)
		client, err = redis.New(conf.Redis.Host, conf.Redis.Port, conf.Redis.Password)
	case "psql":
		log.Printf("Use postgres on %v@%v:%v/%v, timeout %v", conf.Psql.User, conf.Psql.Host, conf.Psql.Port, conf.Psql.Name, conf.Psql.Timeout.Duration)
		client, err = psql.New(conf.Psql.Host, conf.Psql.Port, conf.Psql.Name, conf.Psql.User, conf.Psql.Password, conf.Psql.Timeout.Duration)
	default:
		log.Fatalf("Unknown kind of storage %v\n", conf.Kind)
	}
	if err != nil {
		return nil, err
	}
	return &storage{client: client}, nil
}

type storage struct {
	client client
}

func (s *storage) Save(url string, expires *time.Time) (string, error) {
	var id uint64

	for {
		id = rand.Uint64()
		isUsed, err := s.client.IsUsed(id)
		if err != nil {
			return "", err
		}
		if !isUsed {
			break
		}
	}

	item := model.Item{Id: id, URL: url, Expires: expires}

	if err := s.client.Create(item); err != nil {
		return "", err
	}

	return base62.Encode(id), nil
}

func (s *storage) Load(code string) (string, error) {
	decodedId, err := base62.Decode(code)
	if err != nil {
		return "", err
	}

	return s.client.Load(decodedId)
}

func (s *storage) LoadInfo(code string) (model.Item, error) {
	decodedId, err := base62.Decode(code)
	if err != nil {
		return model.Item{}, err
	}

	return s.client.LoadInfo(decodedId)
}

func (s *storage) Close() error {
	return s.client.Close()
}
