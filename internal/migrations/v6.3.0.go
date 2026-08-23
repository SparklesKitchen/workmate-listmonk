package migrations

import (
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V6_3_0 introduces native workspace ownership for customer templates. The
// nullable column preserves existing administrator templates while every
// WorkMate customer template is bound to its single permitted Listmonk list.
func V6_3_0(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf, lo *log.Logger) error {
	_, err := db.Exec(`
		ALTER TABLE templates ADD COLUMN IF NOT EXISTS list_id INTEGER REFERENCES lists(id) ON DELETE CASCADE;
		CREATE INDEX IF NOT EXISTS idx_templates_list_id ON templates(list_id);
	`)
	return err
}
