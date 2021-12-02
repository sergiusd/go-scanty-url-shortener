package psql

import (
	"log"
	"time"
)

func startCleanScheduler(pg *psql) {
	ticker := time.NewTicker(time.Hour)
	log.Println("Started expired items cleaner")
	for {
		select {
		case <-ticker.C:
			_ = pg.cleanExpired()
		case <-pg.ctx.Done():
			log.Println("Stopped expired items cleaner")
			return
		}
	}
}

func (pg *psql) cleanExpired() error {
	_, err := pg.conn.Exec(pg.ctx, "DELETE FROM links WHERE expires IS NOT NULL AND expires < $1", time.Now())
	if err != nil {
		log.Printf("Error on clean expires: %v\n", err)
	}
	return err
}
