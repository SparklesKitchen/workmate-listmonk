package migrations

import (
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V6_19_0 adds the optional public subscription form configuration.
func V6_19_0(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf, lo *log.Logger) error {
	_, err := db.Exec(`INSERT INTO settings (key, value) VALUES ('app.public_subscription_form', '{}') ON CONFLICT (key) DO NOTHING`)
	return err
}
