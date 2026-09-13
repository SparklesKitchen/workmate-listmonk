package migrations

import (
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V6_20_0 adds list-scoped public subscription form configuration.
func V6_20_0(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf, lo *log.Logger) error {
	_, err := db.Exec(`ALTER TABLE lists ADD COLUMN IF NOT EXISTS attribs JSONB NOT NULL DEFAULT '{}'`)
	return err
}
