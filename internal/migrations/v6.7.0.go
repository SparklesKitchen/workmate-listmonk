package migrations

import (
	"encoding/json"
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

const workMateReachSurfaceCSS = `
html,body,#app,#app .wrapper,#app .main{background:#071225!important;color:#dce9fa!important}
body{color:#dce9fa!important}.main,.main-content{background:#071225!important}
.notification,.box,.card,.table-wrapper,.modal-card,.modal-card-head,.modal-card-body,.modal-card-foot{background:#0b1b35!important;color:#dce9fa!important;border-color:#244362!important;box-shadow:none!important}
.notification hr,.box hr,.modal-card hr,hr{background:#244362!important;border-color:#244362!important}
.title,.subtitle,.label,.content,.help,.has-text-grey,.has-text-grey-light,.has-text-dark,.has-text-black{color:#dce9fa!important}
a,.a,.has-text-primary{color:#41c9ee!important}.has-text-link{color:#72e1ff!important}
.table,.table thead,.table tbody,.table tr,.table td,.table th{background:#0b1b35!important;color:#dce9fa!important;border-color:#244362!important}
.table thead th,.table th{color:#a9c8e5!important}.table tr:hover td,.table tr.is-selected td{background:#102745!important;color:#f4f8ff!important}
.field .label,.label{background:#0b1b35!important;color:#b9d7ef!important}.input,.textarea,.select select,.taginput .taginput-container,.datepicker .dropdown-trigger .input{background:#0a1730!important;color:#e7f0ff!important;border-color:#315a79!important;box-shadow:none!important}
.input::placeholder,.textarea::placeholder,.select select:invalid{color:#8ca7c3!important;opacity:1!important}.input:focus,.textarea:focus,.select select:focus,.taginput .taginput-container:focus-within{border-color:#41c9ee!important;box-shadow:0 0 0 1px #41c9ee!important}
.select:not(.is-multiple):not(.is-loading)::after{border-color:#41c9ee!important}.taginput .tag{background:#163b5b!important;color:#e7f0ff!important}.taginput .tag .delete:before,.taginput .tag .delete:after{background:#dce9fa!important}
.button{background:#102745!important;color:#dce9fa!important;border-color:#315a79!important;box-shadow:none!important}.button:hover,.button:focus{background:#163b5b!important;color:#fff!important;border-color:#41c9ee!important}.button.is-primary,.button.is-primary:hover,.button.is-primary:focus{background:#18b6dd!important;border-color:#18b6dd!important;color:#03101c!important}
.tabs a{color:#a9c8e5!important;border-color:#244362!important}.tabs li.is-active a{color:#72e1ff!important;border-color:#41c9ee!important}.tabs ul{border-color:#244362!important}
.message,.message-body,.notification.is-info,.notification.is-success,.notification.is-warning{background:#102745!important;color:#dce9fa!important;border-color:#315a79!important}.notification.is-info{border-left-color:#41c9ee!important}
.pagination-link,.pagination-next,.pagination-previous{background:#102745!important;color:#dce9fa!important;border-color:#315a79!important}.pagination-link.is-current{background:#18b6dd!important;color:#03101c!important;border-color:#18b6dd!important}
.dropdown-content,.navbar-dropdown{background:#0b1b35!important;border-color:#244362!important;box-shadow:0 12px 30px rgba(0,0,0,.35)!important}.dropdown-item,.navbar-item{color:#dce9fa!important}.dropdown-item:hover,.navbar-item:hover{background:#163b5b!important;color:#fff!important}
.modal-background{background:rgba(2,8,20,.78)!important}.modal-card-head,.modal-card-foot{border-color:#244362!important}.modal-close::before,.modal-close::after{background:#dce9fa!important}
.b-checkbox.checkbox .check,.b-radio.radio .check,.switch input[type=checkbox]+.check{border-color:#6c8ba8!important;background:#0a1730!important}.b-checkbox.checkbox input[type=checkbox]:checked+.check,.b-radio.radio input[type=radio]:checked+.check,.switch input[type=checkbox]:checked+.check{background:#18b6dd!important;border-color:#18b6dd!important}
.chart canvas{background:#0b1b35!important}.gjs-one-bg,.gjs-two-color{background-color:#0b1b35!important;color:#dce9fa!important}.gjs-pn-panel,.gjs-pn-views-container,.gjs-sm-sector,.gjs-clm-tags{background:#0b1b35!important;color:#dce9fa!important;border-color:#244362!important}
`

// V6_7_0 completes the Reach theme across native Listmonk surfaces and modal states.
func V6_7_0(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf, lo *log.Logger) error {
	value, err := json.Marshal(workMateReachCSS + workMateReachAccountCSS + workMateReachSurfaceCSS)
	if err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE settings SET value = $1::jsonb WHERE key IN ('appearance.admin.custom_css', 'appearance.public.custom_css')`, string(value))
	return err
}
