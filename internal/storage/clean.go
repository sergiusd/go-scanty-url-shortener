package storage

import (
	"context"
	log "github.com/sirupsen/logrus"
	"time"
)

func startCleanScheduler(ctx context.Context, c clientCleaner) {
	ticker := time.NewTicker(time.Hour)
	log.Infoln("Started expired items cleaner")
	for {
		select {
		case <-ticker.C:
			if err := c.CleanExpired(); err != nil {
				log.Errorf("Can't clean expires: %+v", err)
			}
		case <-ctx.Done():
			log.Infoln("Stopped expired items cleaner")
			return
		}
	}
}
