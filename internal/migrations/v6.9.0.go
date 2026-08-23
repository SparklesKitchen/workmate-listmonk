package migrations

import (
	"encoding/json"
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

const workMateReachModuleBannerCSS = `
.navbar{position:relative!important;height:190px!important;min-height:190px!important;padding:0!important;overflow:hidden!important;background:linear-gradient(120deg,#3b071a 0%,#160713 38%,#050817 62%,#071c4a 100%)!important;border-bottom:1px solid rgba(103,232,249,.24)!important}
.navbar:after{content:"REACH"!important;position:absolute!important;inset:0 320px 0 0!important;display:flex!important;align-items:center!important;justify-content:center!important;font:900 clamp(76px,14vw,190px)/.8 Anton,Impact,sans-serif!important;letter-spacing:.02em!important;color:rgba(255,255,255,.055)!important;pointer-events:none!important}
.navbar:before{content:""!important;position:absolute!important;z-index:1!important;right:62px!important;top:50%!important;width:220px!important;height:220px!important;transform:translateY(-50%)!important;border-radius:50%!important;background-image:url("/workmate-assets/agents/team-web/copy-carl.webp"),radial-gradient(circle at 36% 26%,rgba(97,217,255,.22),transparent 22%),radial-gradient(circle at 44% 62%,rgba(64,100,214,.24),transparent 28%),linear-gradient(145deg,#061225 0%,#0a1d3d 48%,#03060e 100%)!important;background-size:190px auto,cover,cover,cover!important;background-position:center bottom,center,center,center!important;background-repeat:no-repeat!important;box-shadow:0 0 0 30px rgba(2,9,24,.96),0 0 0 64px rgba(3,16,42,.94),0 0 36px 72px rgba(67,97,238,.34),inset 0 0 60px rgba(97,217,255,.16)!important}
.navbar-brand{position:relative!important;z-index:3!important;opacity:1!important;padding:0 0 0 34px!important}.navbar-brand .logo{display:none!important}.navbar-brand:before{content:"WORKMATE / REACH"!important;display:block!important;margin-top:80px!important;color:#e8f6ff!important;font:700 12px/1 Inter,sans-serif!important;letter-spacing:.24em!important}
.navbar-end{position:relative!important;z-index:3!important;padding-right:28px!important}.navbar .navbar-item{background:transparent!important}.navbar .navbar-item:hover{background:rgba(255,255,255,.06)!important}
.sidebar,#app .sidebar{top:190px!important;height:calc(100vh - 190px)!important}.main,.main-content{padding-top:220px!important}
.sidebar .menu-list a:hover,.sidebar .menu-list a.is-active,.sidebar .menu-list a.router-link-active,.sidebar .menu-list .router-link-exact-active{background:#123b57!important;color:#72e1ff!important}.sidebar .menu-list a:hover .icon,.sidebar .menu-list a.is-active .icon,.sidebar .menu-list a.router-link-active .icon{color:#72e1ff!important}
`

// V6_9_0 applies the canonical WorkMate Reach banner and removes native white menu states.
func V6_9_0(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf, lo *log.Logger) error {
	value, err := json.Marshal(workMateReachCSS + workMateReachAccountCSS + workMateReachSurfaceCSS + workMateReachContrastCSS + workMateReachModuleBannerCSS)
	if err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE settings SET value = $1::jsonb WHERE key IN ('appearance.admin.custom_css', 'appearance.public.custom_css')`, string(value))
	return err
}
