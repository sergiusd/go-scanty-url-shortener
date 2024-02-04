package storage

import (
	"context"
	"math/rand"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/sergiusd/go-scanty-url-shortener/internal/base62"
	"github.com/sergiusd/go-scanty-url-shortener/internal/config"
	"github.com/sergiusd/go-scanty-url-shortener/internal/model"
	"github.com/sergiusd/go-scanty-url-shortener/internal/storage/bolt"
	"github.com/sergiusd/go-scanty-url-shortener/internal/storage/psql"
	"github.com/sergiusd/go-scanty-url-shortener/internal/storage/redis"
)

type storage struct {
	ctx    context.Context
	cancel context.CancelFunc
	client client
}

type client interface {
	Create(item model.Item) error
	Load(decodedId uint64) (string, error)
	LoadInfo(decodedId uint64) (model.Item, error)
	Close() error
	Stat(ctx context.Context) (interface{}, error)
}

type clientCleaner interface {
	CleanExpired() error
}

func New(conf config.Storage) (*storage, error) {
	var err error
	var client client
	ctx, cancel := context.WithCancel(context.Background())
	switch conf.Kind {
	case "redis":
		log.Infof("Use redis on %v:%v", conf.Redis.Host, conf.Redis.Port)
		client, err = redis.New(conf.Redis.Host, conf.Redis.Port, conf.Redis.Password)
	case "psql":
		log.Infof("Use postgres on %v@%v:%v/%v, timeout %v", conf.Psql.User, conf.Psql.Host, conf.Psql.Port, conf.Psql.Name, conf.Psql.Timeout.Duration)
		client, err = psql.New(ctx, conf.Psql.Host, conf.Psql.Port, conf.Psql.Name, conf.Psql.User, conf.Psql.Password, conf.Psql.Timeout.Duration)
	case "bolt":
		log.Infof("Use bolt on %v:%v, timeout %v", conf.Bolt.Path, conf.Bolt.Bucket, conf.Psql.Timeout.Duration)
		client, err = bolt.New(conf.Bolt.Path, conf.Bolt.Bucket, conf.Psql.Timeout.Duration)

	default:
		cancel()
		log.Fatalf("Unknown kind of storage %v", conf.Kind)
	}
	if err != nil {
		cancel()
		return nil, errors.Wrap(err, "Can't initialize storage")
	}

	if cleaner, ok := client.(clientCleaner); ok {
		go startCleanScheduler(ctx, cleaner)
	}

	return &storage{client: client, ctx: ctx, cancel: cancel}, nil
}

var r = rand.New(rand.NewSource(time.Now().Unix()))

func (s *storage) Save(url string, expires *time.Time) (string, error) {
	item := model.Item{URL: url, Expires: expires}
	collisionCount := 0

	for {
		item.Id = r.Uint64()
		err := s.client.Create(item)
		if err == nil {
			break
		}
		if errors.Is(err, model.ErrItemDuplicated) {
			collisionCount += 1
			continue
		}
		return "", errors.Wrap(err, "Can't storage save")
	}

	if collisionCount != 0 {
		log.Warnf("Collision on save unique short URL name: %v times", collisionCount)
	}

	return base62.Encode(item.Id), nil
}

func (s *storage) Load(code string) (string, error) {
	decodedId, err := base62.Decode(code)
	if err != nil {
		return "", errors.Wrap(err, "Can't storage load")
	}

	return s.client.Load(decodedId)
}

func (s *storage) LoadInfo(code string) (model.Item, error) {
	decodedId, err := base62.Decode(code)
	if err != nil {
		return model.Item{}, errors.Wrap(err, "Can't storage loadInfo")
	}

	return s.client.LoadInfo(decodedId)
}

func (s *storage) Close() error {
	s.cancel()
	return s.client.Close()
}

func (s *storage) Stat(ctx context.Context) (any, error) {
	return s.client.Stat(ctx)
}
