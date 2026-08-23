package migrations

import (
	"encoding/json"
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// Keep the approved Reach module banner unchanged, while ensuring native
// Listmonk editors always render above it. Normalise native divider and field
// label surfaces so stock white rules do not leak into the dark theme.
const workMateReachEditorLayerCSS = `
.navbar{z-index:1!important}.modal{z-index:1000!important}.modal-background{z-index:999!important}.modal-card,.modal-content,.modal .animation-content{position:relative!important;z-index:1001!important}
hr,.dropdown-divider{background:#244362!important;border-color:#244362!important}.field .label,.field-label .label,legend{background:transparent!important;border:0!important;box-shadow:none!important;color:#dce9fa!important}
`

func V6_13_0(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf, lo *log.Logger) error {
	value, err := json.Marshal(workMateReachCSS + workMateReachAccountCSS + workMateReachSurfaceCSS + workMateReachContrastCSS + workMateReachModuleBannerCSS + workMateReachAccountMenuCSS + workMateReachNativeStateCSS + workMateReachEditorLayerCSS)
	if err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE settings SET value = $1::jsonb WHERE key IN ('appearance.admin.custom_css', 'appearance.public.custom_css')`, string(value))
	return err
}
