package migrations

import (
	"encoding/json"
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

const workMateReachCSS = `body,#app,#app .main{background:#071225!important;color:#e7f0ff!important}.navbar{height:72px!important;min-height:72px!important;padding:0!important;background:linear-gradient(100deg,#062d3d,#111548 72%,#071225)!important;border-bottom:1px solid #1f5c77!important;box-shadow:none!important}.navbar:after{content:"WORKMATE / REACH";position:absolute;left:34px;top:26px;color:#e8f6ff;font:700 12px/1 Inter,sans-serif;letter-spacing:.24em}.navbar-brand{opacity:0!important}.sidebar,#app .sidebar{top:72px!important;height:calc(100vh - 72px)!important;background:#09152b!important;border-right:1px solid #244362!important;box-shadow:none!important}.sidebar a,#app .sidebar a{color:#d8e9fb!important}.sidebar .router-link-exact-active{background:#123b57!important;color:#72e1ff!important}.main,.main-content{padding-top:98px!important;background:#071225!important}.card,.box,.modal-card-body,.table-wrapper{background:#0b1b35!important;color:#e7f0ff!important;border:1px solid #244362!important;box-shadow:none!important}.table{background:#0b1b35!important;color:#e7f0ff!important}.table th,.table td{color:#dcecff!important;border-color:#244362!important}.input,.textarea,.select select{background:#0a1730!important;border-color:#315a79!important;color:#e7f0ff!important}.label,.title,.subtitle,.content,.help{color:#dcecff!important}.button.is-primary{background:#0db7df!important;border-color:#0db7df!important;color:#03101c!important}.pagination-link.is-current{background:#0db7df!important;border-color:#0db7df!important;color:#03101c!important}`

// V6_5_0 replaces the temporary high-contrast appearance with the Reach dark-blue product skin.
func V6_5_0(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf, lo *log.Logger) error {
	value, err := json.Marshal(workMateReachCSS)
	if err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE settings SET value = $1::jsonb WHERE key IN ('appearance.admin.custom_css', 'appearance.public.custom_css')`, string(value))
	return err
}
