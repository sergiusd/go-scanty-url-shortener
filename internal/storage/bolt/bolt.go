package bolt

import (
	"encoding/json"
	"strconv"
	"time"

	boltClient "github.com/boltdb/bolt"
	"github.com/pkg/errors"

	"github.com/sergiusd/go-scanty-url-shortener/internal/model"
)

type bolt struct {
	db        *boltClient.DB
	bucket    []byte
	bucketTTL []byte
}

func New(path string, bucket string, timeout time.Duration) (*bolt, error) {
	db, err := boltClient.Open(path, 0600, &boltClient.Options{Timeout: timeout})
	if err != nil {
		return nil, errors.Wrap(err, "Can't open bolt connection")
	}
	bucketTTL := bucket + "_ttl"
	err = db.Update(func(tx *boltClient.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte(bucket)); err != nil {
			return errors.Wrapf(err, "Can't create %s bucket", bucket)
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(bucketTTL)); err != nil {
			return errors.Wrapf(err, "Can't create %s bucket", bucketTTL)
		}
		return err
	})
	if err != nil {
		return nil, errors.Wrap(err, "Can't create buckets")
	}
	return &bolt{db: db, bucket: []byte(bucket)}, nil
}

func getItemKey(decodedId uint64) string {
	return strconv.FormatUint(decodedId, 10)
}

func (b *bolt) bucketData(tx *boltClient.Tx) *boltClient.Bucket {
	return tx.Bucket(b.bucket)
}

func (b *bolt) bucketTtl(tx *boltClient.Tx) *boltClient.Bucket {
	return tx.Bucket(b.bucketTTL)
}

func (b *bolt) IsUsed(id uint64) (bool, error) {
	isUsed := false
	err := b.db.View(func(tx *boltClient.Tx) error {
		v := b.bucketData(tx).Get([]byte(getItemKey(id)))
		if v != nil {
			isUsed = true
		}
		return nil
	})
	return isUsed, errors.Wrap(err, "Can't get item is used")
}

func (b *bolt) Create(item model.Item) error {
	itemRaw, err := json.Marshal(item)
	if err != nil {
		return errors.Wrap(err, "Can't marshal item")
	}
	err = b.db.Update(func(tx *boltClient.Tx) error {
		key := []byte(getItemKey(item.Id))
		if err := b.bucketData(tx).Put(key, itemRaw); err != nil {
			return errors.Wrap(err, "Can't put data into bucket")
		}
		if item.Expires != nil {
			ttlKey := []byte(strconv.FormatInt(item.Expires.Unix(), 10))
			if err := b.bucketData(tx).Put(ttlKey, key); err != nil {
				return errors.Wrap(err, "Can't put data into ttl bucket")
			}
		}
		return nil
	})
	return errors.Wrap(err, "Can't create item")
}

func (b *bolt) Load(decodedId uint64) (string, error) {
	var url string
	err := b.db.View(func(tx *boltClient.Tx) error {
		v := b.bucketData(tx).Get([]byte(getItemKey(decodedId)))
		if v == nil {
			return model.ErrNoLink
		}
		var item model.Item
		if err := json.Unmarshal(v, &item); err != nil {
			return errors.Wrapf(err, "Can't found item by key %v", decodedId)
		}
		url = item.URL
		return nil
	})
	return url, errors.Wrapf(err, "Can't load item %v", decodedId)
}

func (b *bolt) LoadInfo(decodedId uint64) (model.Item, error) {
	var item model.Item
	err := b.db.View(func(tx *boltClient.Tx) error {
		v := b.bucketData(tx).Get([]byte(getItemKey(decodedId)))
		if v == nil {
			return model.ErrNoLink
		}
		if err := json.Unmarshal(v, &item); err != nil {
			return errors.Wrapf(err, "Can't found item by key %v", decodedId)
		}
		return nil
	})
	return item, errors.Wrapf(err, "Can't loadInfo item %v", decodedId)
}

func (b *bolt) Close() error {
	return b.db.Close()
}
