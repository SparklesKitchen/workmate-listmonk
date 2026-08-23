package migrations

import (
	"encoding/json"
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

const workMateReachContrastCSS = `
.main h1,.main h2,.main h3,.main h4,.main h5,.main h6{color:#e7f0ff!important}
.tabs.is-boxed li.is-active a,.tabs.is-toggle li.is-active a{background:#0b1b35!important;color:#72e1ff!important;border-color:#41c9ee!important}
.tab-content{background:#071225!important;border-color:#244362!important}.tabs.is-boxed a:hover{background:#102745!important}
`

// V6_8_0 fixes native boxed tabs and analytics headings left outside the first Reach surface pass.
func V6_8_0(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf, lo *log.Logger) error {
	value, err := json.Marshal(workMateReachCSS + workMateReachAccountCSS + workMateReachSurfaceCSS + workMateReachContrastCSS)
	if err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE settings SET value = $1::jsonb WHERE key IN ('appearance.admin.custom_css', 'appearance.public.custom_css')`, string(value))
	return err
}
