package psql

import (
	"github.com/pkg/errors"
	"time"
)

func (pg *Psql) CleanExpired() error {
	err := pg.exec("DELETE FROM links WHERE expires IS NOT NULL AND expires < $1", time.Now())
	return errors.Wrap(err, "Can't delete expires links")
}
