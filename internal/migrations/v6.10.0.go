package migrations

import (
	"encoding/json"
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// Keep the account control inside the Cannonolian banner and above the native
// page stack. The stock dropdown otherwise inherits a white active tile and
// is painted below Listmonk's main content on some routes.
const workMateReachAccountMenuCSS = `
.navbar{z-index:1000!important;overflow:visible!important}
.navbar:after,.navbar:before{pointer-events:none!important}
.navbar-end{height:190px!important;align-items:center!important;background:transparent!important}
.navbar-item.has-dropdown.user{position:relative!important;z-index:1002!important;height:190px!important;margin:0!important;padding:0 28px!important;background:#0b1b35!important;border-left:1px solid rgba(103,232,249,.18)!important}
.navbar-item.has-dropdown.user:hover,.navbar-item.has-dropdown.user.is-active{background:#102b4c!important}
.navbar-item.has-dropdown.user .navbar-link{height:190px!important;padding:0!important;color:#dce9fa!important}
.navbar-item.has-dropdown.user .navbar-link:after{border-color:#72e1ff!important}
.navbar-item.has-dropdown.user .user-avatar{background:#e7f0ff!important;color:#123b57!important}
.navbar-item.has-dropdown.user .navbar-dropdown{display:block!important;z-index:1003!important;min-width:250px!important;top:190px!important;right:0!important;padding:8px!important;background:#0b1b35!important;border:1px solid #244362!important;box-shadow:0 18px 42px rgba(0,0,0,.48)!important}
.navbar-item.has-dropdown.user .navbar-dropdown .navbar-item{display:flex!important;min-height:42px!important;padding:10px 12px!important;background:transparent!important;color:#dce9fa!important}
.navbar-item.has-dropdown.user .navbar-dropdown .navbar-item:hover{background:#163b5b!important;color:#fff!important}
`

// V6_10_0 fixes the native account dropdown as one layered banner surface.
func V6_10_0(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf, lo *log.Logger) error {
	value, err := json.Marshal(workMateReachCSS + workMateReachAccountCSS + workMateReachSurfaceCSS + workMateReachContrastCSS + workMateReachModuleBannerCSS + workMateReachAccountMenuCSS)
	if err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE settings SET value = $1::jsonb WHERE key IN ('appearance.admin.custom_css', 'appearance.public.custom_css')`, string(value))
	return err
}
