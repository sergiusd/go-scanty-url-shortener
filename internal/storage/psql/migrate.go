package psql

import (
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
	log "github.com/sirupsen/logrus"
)

func migrate(ctx context.Context, conn *pgxpool.Conn) error {
	var tableExists bool
	if err := conn.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", "public.links").Scan(&tableExists); err != nil {
		return err
	}

	if tableExists {
		return nil
	}

	log.Infoln("Postgresql migrates...")

	if _, err := conn.Exec(ctx, `
		CREATE TABLE public.links (
			id BIGINT NOT NULL,
			url VARCHAR NOT NULL,
			expires TIMESTAMPTZ,
			visits INT NOT NULL DEFAULT 0
		)
	`); err != nil {
		return err
	}

	if _, err := conn.Exec(ctx, `
		CREATE UNIQUE INDEX links_id_uniq ON public.links (id)
	`); err != nil {
		return err
	}

	if _, err := conn.Exec(ctx, `
		CREATE INDEX links_expires_idx ON public.links (expires) WHERE expires IS NOT NULL
	`); err != nil {
		return err
	}

	return nil
}
