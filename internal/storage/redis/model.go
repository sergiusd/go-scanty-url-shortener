package redis

import (
	"time"
)

type Item struct {
	Id      uint64 `redis:"id"`
	URL     string `redis:"url"`
	Expires string `redis:"expires"`
}

func (i *Item) ExportExpires() *time.Time {
	if i.Expires == "" {
		return nil
	}
	ret, _ := time.Parse(time.RFC3339, i.Expires)
	return &ret
}

func (i *Item) ImportExpires(val *time.Time) {
	if val == nil {
		i.Expires = ""
		return
	}
	exp := val.Format(time.RFC3339)
	i.Expires = exp
}
