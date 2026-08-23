package migrations

import (
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// Public Listmonk forms must generate the customer-facing WorkMate Reach URL,
// never the private loopback address used by the shared XXL container.
func V6_14_0(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf, lo *log.Logger) error {
	_, err := db.Exec(`UPDATE settings SET value = '"https://app.workmateos.co.uk/saas-admin/listmonk"'::jsonb WHERE key = 'app.root_url'`)
	return err
}
