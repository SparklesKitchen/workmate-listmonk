package migrations

import (
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// Public subscription pages must carry the Reach identity, not stock listmonk.
func V6_17_0(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf, lo *log.Logger) error {
	_, err := db.Exec(`UPDATE settings SET value = '"Reach"'::jsonb WHERE key = 'app.site_name'`)
	return err
}
