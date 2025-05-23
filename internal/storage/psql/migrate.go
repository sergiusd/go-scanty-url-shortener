package psql

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	log "github.com/sirupsen/logrus"
)

func migrate(ctx context.Context, conn *pgxpool.Conn) error {

	if err := migrationV1(ctx, conn); err != nil {
		return err
	}
	if err := migrationV2(ctx, conn); err != nil {
		return err
	}
	if err := migrationV3(ctx, conn); err != nil {
		return err
	}

	return nil
}

func migrationV1(ctx context.Context, conn *pgxpool.Conn) error {
	var tableExists bool
	if err := conn.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", "public.links").Scan(&tableExists); err != nil {
		return err
	}

	if tableExists {
		return nil
	}

	log.Infoln("Postgresql migrates V1...")

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

	log.Infoln("Migrate finished")

	return nil
}

func migrationV2(ctx context.Context, conn *pgxpool.Conn) error {
	var indexExists bool
	if err := conn.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", "public.links_url_idx").Scan(&indexExists); err != nil {
		return err
	}

	if indexExists {
		return nil
	}

	log.Infoln("Postgresql migrates V2...")

	if _, err := conn.Exec(ctx, `
		CREATE INDEX links_url_idx ON public.links USING HASH (url)
	`); err != nil {
		return err
	}

	log.Infoln("Migrate finished")

	return nil
}

func migrationV3(ctx context.Context, conn *pgxpool.Conn) error {
	var columnExists bool
	if err := conn.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = $1 AND column_name = $2)",
		"links", "visits",
	).Scan(&columnExists); err != nil {
		return err
	}

	if !columnExists {
		return nil
	}

	log.Infoln("Postgresql migrates V3...")

	if _, err := conn.Exec(ctx, `
		ALTER TABLE public.links DROP COLUMN visits
	`); err != nil {
		return err
	}

	log.Infoln("Migrate finished")

	return nil
}
