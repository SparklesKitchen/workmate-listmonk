package migrations

import (
	"encoding/json"
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// Buefy's floating-label ::before is a hard-coded white strip that masks the
// input border on the stock light theme; on the dark Reach theme it renders as
// a white line struck through every field label. Remove it and give floating
// labels the page background so the input border no longer cuts through them.
const workMateReachFloatingLabelCSS = `
.field.is-floating-label .label::before{display:none!important;background:transparent!important}
.field.is-floating-label .label{background:#071225!important;padding:0 6px!important}
`

func V6_15_0(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf, lo *log.Logger) error {
	value, err := json.Marshal(workMateReachCSS + workMateReachAccountCSS + workMateReachSurfaceCSS + workMateReachContrastCSS + workMateReachModuleBannerCSS + workMateReachAccountMenuCSS + workMateReachNativeStateCSS + workMateReachEditorLayerCSS + workMateReachFloatingLabelCSS)
	if err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE settings SET value = $1::jsonb WHERE key IN ('appearance.admin.custom_css', 'appearance.public.custom_css')`, string(value))
	return err
}
