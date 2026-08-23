package migrations

import (
	"encoding/json"
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

const workMateReachAccountCSS = `.navbar-dropdown .user-name strong{display:none!important}.navbar-dropdown .user-name .is-size-7{font-size:14px!important;font-weight:700!important;color:#e7f0ff!important}`

// V6_6_0 keeps the WorkMate workspace name and hides the internal scoped username.
func V6_6_0(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf, lo *log.Logger) error {
	value, err := json.Marshal(workMateReachCSS + workMateReachAccountCSS)
	if err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE settings SET value = $1::jsonb WHERE key IN ('appearance.admin.custom_css', 'appearance.public.custom_css')`, string(value))
	return err
}
