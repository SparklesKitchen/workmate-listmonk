package migrations

import (
	"encoding/json"
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// The public pages were inheriting the pale admin dark-theme text colors
// because earlier migrations dumped the admin CSS into the public key too.
// Give public pages their own small, strong stylesheet instead.
const workMateReachPublicCSS = `
h1,h2,h3,h4{color:#0b1b35!important;font-weight:700!important}
.button,button[type=submit]{background:#0db7df!important;border-color:#0db7df!important;color:#fff!important;font-weight:600!important}
`

func V6_18_0(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf, lo *log.Logger) error {
	value, err := json.Marshal(workMateReachPublicCSS)
	if err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE settings SET value = $1::jsonb WHERE key = 'appearance.public.custom_css'`, string(value))
	return err
}
