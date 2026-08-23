package migrations

import (
	"encoding/json"
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// Keep every native Listmonk navigation state and card surface on the Reach
// palette. Buefy's stock selectors otherwise restore white on hover/focus.
const workMateReachNativeStateCSS = `
.sidebar .b-sidebar .menu-list a,.sidebar .menu-list a{background:transparent!important;color:#dce9fa!important}
.sidebar .b-sidebar .menu-list a:hover,.sidebar .b-sidebar .menu-list a:focus,.sidebar .b-sidebar .menu-list a.is-active,.sidebar .b-sidebar .menu-list a.router-link-active,.sidebar .b-sidebar .menu-list a.router-link-exact-active,.sidebar .menu-list a:hover,.sidebar .menu-list a:focus,.sidebar .menu-list a.is-active,.sidebar .menu-list a.router-link-active,.sidebar .menu-list a.router-link-exact-active{background:#123b57!important;color:#72e1ff!important}
.sidebar .menu-list li ul{border-left-color:#315a79!important}.sidebar .menu-list a .icon{color:inherit!important}
.panel,.panel-heading,.panel-block,.message-header,.message-body,.dropdown-content,.autocomplete .dropdown-menu,.datepicker .dropdown-content,.tag,.pagination-ellipsis{background:#0b1b35!important;color:#dce9fa!important;border-color:#244362!important}
.panel-block:hover,.panel-block:focus,.dropdown-item:focus,.autocomplete .dropdown-item:hover{background:#163b5b!important;color:#fff!important}
.modal-content,.modal-card,.modal-card-head,.modal-card-body,.modal-card-foot,.modal .animation-content{background:#0b1b35!important;color:#dce9fa!important;border-color:#244362!important}
.chart text,.chart .label,.chart .title,.chart .tick text{fill:#dce9fa!important;color:#dce9fa!important}.chart line,.chart path.domain{stroke:#315a79!important}
`

// V6_12_0 eliminates remaining stock white interaction states and restores
// legible native analytics labels without replacing Listmonk's UI.
func V6_12_0(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf, lo *log.Logger) error {
	value, err := json.Marshal(workMateReachCSS + workMateReachAccountCSS + workMateReachSurfaceCSS + workMateReachContrastCSS + workMateReachModuleBannerCSS + workMateReachAccountMenuCSS + workMateReachNativeStateCSS)
	if err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE settings SET value = $1::jsonb WHERE key IN ('appearance.admin.custom_css', 'appearance.public.custom_css')`, string(value))
	return err
}
