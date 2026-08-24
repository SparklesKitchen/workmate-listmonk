package migrations

import (
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// Reach analytics must count per-subscriber views and clicks; the stock
// default keeps individual tracking off, which renders campaign analytics
// as non-unique totals with a warning banner.
func V6_16_0(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf, lo *log.Logger) error {
	if _, err := db.Exec(`UPDATE settings SET value = 'true'::jsonb WHERE key = 'privacy.individual_tracking'`); err != nil {
		return err
	}
	// Reach now serves customers from its own subdomain with native paths.
	_, err := db.Exec(`UPDATE settings SET value = '"https://reach.workmateos.co.uk"'::jsonb WHERE key = 'app.root_url'`)
	return err
}
