package main

import (
	"encoding/json"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/knadh/listmonk/internal/auth"
	"github.com/knadh/listmonk/internal/messenger/brevo"
	"github.com/knadh/listmonk/internal/messenger/email"
	"github.com/knadh/listmonk/models"
	"github.com/knadh/smtppool/v2"
	"github.com/labstack/echo/v4"
)

// workMateDeliveryRequest is deliberately narrower than Listmonk's global
// Settings model. A customer can configure only the SMTP server used by their
// current WorkMate workspace; global application, privacy, OIDC, bounce and
// other workspaces' settings remain administrator-owned.
type workMateDeliveryRequest struct {
	Provider     string `json:"provider"`
	SenderName   string `json:"sender_name"`
	FromEmail    string `json:"from_email"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	AuthProtocol string `json:"auth_protocol"`
	TLSType      string `json:"tls_type"`
}

type workMateDeliveryResponse struct {
	Configured   bool   `json:"configured"`
	Verified     bool   `json:"verified"`
	Provider     string `json:"provider"`
	SenderName   string `json:"sender_name"`
	FromEmail    string `json:"from_email"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Username     string `json:"username"`
	AuthProtocol string `json:"auth_protocol"`
	TLSType      string `json:"tls_type"`
}

type workMateDeliveryVerifyRequest struct {
	Provider  string `json:"provider"`
	TestEmail string `json:"test_email"`
}

const (
	workMateDeliveryBrevoAPI  = "brevo_api"
	workMateDeliveryBrevoSMTP = "brevo_smtp"
	workMateDeliverySMTP      = "smtp"
)

func workMateDeliverySMTPName(user auth.User, provider string) string {
	if !user.ListRoleName.Valid {
		return ""
	}
	return "wm-delivery-" + provider + "-" + user.ListRoleName.String
}

func workMateDeliveryBrevoName(user auth.User) string {
	if !user.ListRoleName.Valid {
		return ""
	}
	return "wm-delivery-" + workMateDeliveryBrevoAPI + "-" + user.ListRoleName.String
}

func workMateDeliveryProvider(provider string) string {
	return strings.TrimSpace(strings.ToLower(provider))
}

func isWorkMateDeliveryProvider(provider string) bool {
	switch workMateDeliveryProvider(provider) {
	case workMateDeliveryBrevoAPI, workMateDeliveryBrevoSMTP, workMateDeliverySMTP:
		return true
	default:
		return false
	}
}

func (a *App) workMateDeliveryUser(c echo.Context) (auth.User, error) {
	user := auth.GetUser(c)
	if !isWorkMateCustomer(user) || user.ListRoleID == nil || !user.ListRoleName.Valid || !strings.HasPrefix(user.ListRoleName.String, "wm-lr-") {
		return user, auth.ErrPermDenied
	}
	return user, nil
}

// GetWorkMateDelivery returns only the current workspace's delivery metadata.
// It never returns SMTP passwords or records belonging to another workspace.
func (a *App) GetWorkMateDelivery(c echo.Context) error {
	user, err := a.workMateDeliveryUser(c)
	if err != nil {
		return err
	}
	settings, err := a.core.GetSettings()
	if err != nil {
		return err
	}
	name := workMateDeliverySMTPName(user, workMateDeliverySMTP)
	brevoSMTPName := workMateDeliverySMTPName(user, workMateDeliveryBrevoSMTP)
	brevoName := workMateDeliveryBrevoName(user)
	for _, messenger := range settings.Messengers {
		if messenger.Name == brevoName {
			return c.JSON(http.StatusOK, okResp{workMateDeliveryResponse{
				Configured: true, Verified: messenger.Enabled, Provider: workMateDeliveryBrevoAPI, SenderName: messenger.RootURL,
				FromEmail: messenger.Username,
			}})
		}
	}
	for _, smtp := range settings.SMTP {
		if smtp.Name == name || smtp.Name == brevoSMTPName {
			provider := workMateDeliverySMTP
			if smtp.Name == brevoSMTPName {
				provider = workMateDeliveryBrevoSMTP
			}
			return c.JSON(http.StatusOK, okResp{workMateDeliveryResponse{
				Configured: true, Verified: smtp.Enabled, Provider: provider,
				FromEmail: firstWorkMateFromAddress(smtp.FromAddresses), Host: smtp.Host,
				Port: smtp.Port, Username: smtp.Username, AuthProtocol: smtp.AuthProtocol, TLSType: smtp.TLSType,
			}})
		}
	}
	return c.JSON(http.StatusOK, okResp{workMateDeliveryResponse{}})
}

// UpdateWorkMateDelivery upserts a single namespaced SMTP record. The shared
// Listmonk runtime owns the encrypted-at-rest runtime configuration; the
// workspace identity is enforced by the signed SSO session and list role.
func (a *App) UpdateWorkMateDelivery(c echo.Context) error {
	user, err := a.workMateDeliveryUser(c)
	if err != nil {
		return err
	}
	var req workMateDeliveryRequest
	if err := c.Bind(&req); err != nil {
		return err
	}
	if err := validateWorkMateDeliveryRequest(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	settings, err := a.core.GetSettings()
	if err != nil {
		return err
	}
	removeOtherWorkMateDeliveryConnections(user, req.Provider, &settings)
	disableWorkMateDeliveryConnections(user, &settings)
	name := workMateDeliverySMTPName(user, req.Provider)
	from := email.NormalizeAddr(req.FromEmail)
	if req.Provider == workMateDeliveryBrevoAPI {
		brevoName := workMateDeliveryBrevoName(user)
		found := false
		for i := range settings.Messengers {
			if settings.Messengers[i].Name != brevoName {
				continue
			}
			found = true
			if req.Password == "" {
				req.Password = settings.Messengers[i].Password
			}
			settings.Messengers[i].Enabled = false
			settings.Messengers[i].Name = brevoName
			settings.Messengers[i].Username = from
			settings.Messengers[i].Password = req.Password
			settings.Messengers[i].RootURL = req.SenderName
			settings.Messengers[i].MaxConns = 5
			settings.Messengers[i].Timeout = "30s"
			settings.Messengers[i].MaxMsgRetries = 2
			break
		}
		if !found {
			if req.Password == "" {
				return echo.NewHTTPError(http.StatusBadRequest, "a Brevo API key is required for a new delivery connection")
			}
			settings.Messengers = append(settings.Messengers, struct {
				UUID          string `json:"uuid"`
				Enabled       bool   `json:"enabled"`
				Name          string `json:"name"`
				RootURL       string `json:"root_url"`
				Username      string `json:"username"`
				Password      string `json:"password,omitempty"`
				MaxConns      int    `json:"max_conns"`
				Timeout       string `json:"timeout"`
				MaxMsgRetries int    `json:"max_msg_retries"`
			}{Enabled: false, Name: brevoName, RootURL: req.SenderName, Username: from, Password: req.Password, MaxConns: 5, Timeout: "30s", MaxMsgRetries: 2})
		}
		if err := a.updateWorkMateDeliverySettings(settings); err != nil {
			return err
		}
		return a.handleSettingsRestart(c)
	}
	found := false
	for i := range settings.SMTP {
		if settings.SMTP[i].Name != name {
			continue
		}
		found = true
		if req.Password == "" {
			req.Password = settings.SMTP[i].Password
		}
		settings.SMTP[i].Enabled = false
		settings.SMTP[i].Name = name
		settings.SMTP[i].Host = req.Host
		settings.SMTP[i].Port = req.Port
		settings.SMTP[i].Username = req.Username
		settings.SMTP[i].Password = req.Password
		settings.SMTP[i].AuthProtocol = req.AuthProtocol
		settings.SMTP[i].TLSType = req.TLSType
		settings.SMTP[i].TLSSkipVerify = false
		settings.SMTP[i].FromAddresses = []string{from}
		settings.SMTP[i].MaxConns = 5
		settings.SMTP[i].MaxMsgRetries = 2
		settings.SMTP[i].MsgRetryDelay = "5m"
		settings.SMTP[i].IdleTimeout = "15s"
		settings.SMTP[i].WaitTimeout = "5s"
		break
	}
	if !found {
		if req.Password == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "an SMTP password or key is required for a new delivery connection")
		}
		settings.SMTP = append(settings.SMTP, struct {
			Name          string              `json:"name"`
			UUID          string              `json:"uuid"`
			Enabled       bool                `json:"enabled"`
			Host          string              `json:"host"`
			HelloHostname string              `json:"hello_hostname"`
			Port          int                 `json:"port"`
			AuthProtocol  string              `json:"auth_protocol"`
			Username      string              `json:"username"`
			Password      string              `json:"password,omitempty"`
			EmailHeaders  []map[string]string `json:"email_headers"`
			MaxConns      int                 `json:"max_conns"`
			MaxMsgRetries int                 `json:"max_msg_retries"`
			MsgRetryDelay string              `json:"msg_retry_delay"`
			IdleTimeout   string              `json:"idle_timeout"`
			WaitTimeout   string              `json:"wait_timeout"`
			TLSType       string              `json:"tls_type"`
			TLSSkipVerify bool                `json:"tls_skip_verify"`
			FromAddresses []string            `json:"from_addresses"`
		}{
			Name: name, Enabled: false, Host: req.Host, Port: req.Port, Username: req.Username,
			Password: req.Password, AuthProtocol: req.AuthProtocol, TLSType: req.TLSType,
			EmailHeaders: []map[string]string{}, MaxConns: 5, MaxMsgRetries: 2,
			MsgRetryDelay: "5m", IdleTimeout: "15s", WaitTimeout: "5s", FromAddresses: []string{from},
		})
	}
	if err := a.updateWorkMateDeliverySettings(settings); err != nil {
		return err
	}
	return a.handleSettingsRestart(c)
}

// VerifyWorkMateDelivery sends an explicit test message through the workspace's
// saved provider. Only a successful provider send marks the connection and
// sender as verified and enables it for campaigns.
func (a *App) VerifyWorkMateDelivery(c echo.Context) error {
	user, err := a.workMateDeliveryUser(c)
	if err != nil {
		return err
	}
	var req workMateDeliveryVerifyRequest
	if err := c.Bind(&req); err != nil {
		return err
	}
	req.Provider = workMateDeliveryProvider(req.Provider)
	if !isWorkMateDeliveryProvider(req.Provider) {
		return echo.NewHTTPError(http.StatusBadRequest, "select a saved delivery provider")
	}
	req.TestEmail = email.NormalizeAddr(req.TestEmail)
	if parsed, err := mail.ParseAddress(req.TestEmail); err != nil || req.TestEmail == "" || parsed.Address != req.TestEmail {
		return echo.NewHTTPError(http.StatusBadRequest, "a valid test recipient email is required")
	}

	settings, err := a.core.GetSettings()
	if err != nil {
		return err
	}
	message := models.Message{
		To:          []string{req.TestEmail},
		Subject:     "WorkMate delivery verification",
		Body:        []byte("<p>Your delivery connection and sender were verified.</p>"),
		ContentType: "html",
	}

	switch req.Provider {
	case workMateDeliveryBrevoAPI:
		name := workMateDeliveryBrevoName(user)
		for i := range settings.Messengers {
			messenger := &settings.Messengers[i]
			if messenger.Name != name {
				continue
			}
			message.From = messenger.Username
			client, err := brevo.New(brevo.Options{Name: name, APIKey: messenger.Password, SenderEmail: messenger.Username, SenderName: messenger.RootURL})
			if err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, "saved Brevo API connection is invalid")
			}
			if err := client.Push(message); err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, "Brevo API or sender verification failed: "+err.Error())
			}
			disableWorkMateDeliveryConnections(user, &settings)
			messenger.Enabled = true
			if err := a.updateWorkMateDeliverySettings(settings); err != nil {
				return err
			}
			return a.handleSettingsRestart(c)
		}
	case workMateDeliveryBrevoSMTP, workMateDeliverySMTP:
		name := workMateDeliverySMTPName(user, req.Provider)
		for i := range settings.SMTP {
			smtp := &settings.SMTP[i]
			if smtp.Name != name {
				continue
			}
			message.From = firstWorkMateFromAddress(smtp.FromAddresses)
			server := email.Server{
				Name: name, Username: smtp.Username, Password: smtp.Password, AuthProtocol: smtp.AuthProtocol,
				TLSType: smtp.TLSType, TLSSkipVerify: false, FromAddresses: smtp.FromAddresses,
				Opt: smtppool.Opt{Host: smtp.Host, Port: smtp.Port, MaxConns: 1, MaxMessageRetries: 1, MsgRetryDelay: 5 * time.Second, IdleTimeout: 15 * time.Second, PoolWaitTimeout: 5 * time.Second},
			}
			client, err := email.New(name, server)
			if err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, "saved SMTP connection is invalid")
			}
			defer client.Close()
			if err := client.Push(message); err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, "SMTP or sender verification failed: "+err.Error())
			}
			disableWorkMateDeliveryConnections(user, &settings)
			smtp.Enabled = true
			if err := a.updateWorkMateDeliverySettings(settings); err != nil {
				return err
			}
			return a.handleSettingsRestart(c)
		}
	}

	return echo.NewHTTPError(http.StatusBadRequest, "save this workspace delivery connection before verifying it")
}

// updateWorkMateDeliverySettings changes only delivery setting keys. It never
// rewrites unrelated global Listmonk settings.
func (a *App) updateWorkMateDeliverySettings(settings models.Settings) error {
	if err := a.updateWorkMateDeliverySMTP(settings); err != nil {
		return err
	}
	return a.updateWorkMateDeliveryMessengers(settings)
}

func (a *App) updateWorkMateDeliverySMTP(settings models.Settings) error {
	b, err := json.Marshal(settings.SMTP)
	if err != nil {
		return err
	}
	return a.core.UpdateSettingsByKey("smtp", b)
}

func disableWorkMateDeliveryConnections(user auth.User, settings *models.Settings) {
	brevoName := workMateDeliveryBrevoName(user)
	for i := range settings.Messengers {
		if settings.Messengers[i].Name == brevoName {
			settings.Messengers[i].Enabled = false
		}
	}
	for _, provider := range []string{workMateDeliverySMTP, workMateDeliveryBrevoSMTP} {
		name := workMateDeliverySMTPName(user, provider)
		for i := range settings.SMTP {
			if settings.SMTP[i].Name == name {
				settings.SMTP[i].Enabled = false
			}
		}
	}
}

// A workspace has one selected delivery connection. Replacing it removes only
// that workspace's prior namespaced connection, so a disabled old provider can
// never be accidentally selected after a page reload.
func removeOtherWorkMateDeliveryConnections(user auth.User, provider string, settings *models.Settings) {
	brevoName := workMateDeliveryBrevoName(user)
	smtpName := workMateDeliverySMTPName(user, workMateDeliverySMTP)
	brevoSMTPName := workMateDeliverySMTPName(user, workMateDeliveryBrevoSMTP)

	messengers := settings.Messengers[:0]
	for _, messenger := range settings.Messengers {
		if messenger.Name == brevoName && provider != workMateDeliveryBrevoAPI {
			continue
		}
		messengers = append(messengers, messenger)
	}
	settings.Messengers = messengers

	smtpServers := settings.SMTP[:0]
	for _, smtp := range settings.SMTP {
		if (smtp.Name == smtpName && provider != workMateDeliverySMTP) || (smtp.Name == brevoSMTPName && provider != workMateDeliveryBrevoSMTP) {
			continue
		}
		smtpServers = append(smtpServers, smtp)
	}
	settings.SMTP = smtpServers
}

func (a *App) updateWorkMateDeliveryMessengers(settings models.Settings) error {
	b, err := json.Marshal(settings.Messengers)
	if err != nil {
		return err
	}
	return a.core.UpdateSettingsByKey("messengers", b)
}

func firstWorkMateFromAddress(addresses []string) string {
	if len(addresses) == 0 {
		return ""
	}
	return addresses[0]
}

func validateWorkMateDeliveryRequest(req *workMateDeliveryRequest) error {
	req.Provider = strings.TrimSpace(strings.ToLower(req.Provider))
	if !isWorkMateDeliveryProvider(req.Provider) {
		return echo.NewHTTPError(http.StatusBadRequest, "delivery provider must be Brevo API, Brevo SMTP, or generic SMTP")
	}
	req.Host = strings.TrimSpace(req.Host)
	req.Username = strings.TrimSpace(req.Username)
	req.FromEmail = email.NormalizeAddr(req.FromEmail)
	parsed, err := mail.ParseAddress(req.FromEmail)
	if err != nil || req.FromEmail == "" || parsed.Address != req.FromEmail {
		return echo.NewHTTPError(http.StatusBadRequest, "a valid sender email is required")
	}
	if req.Provider == workMateDeliveryBrevoAPI {
		req.SenderName = strings.TrimSpace(req.SenderName)
		if req.SenderName == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "a Brevo sender name is required")
		}
		return nil
	}
	if req.Provider == workMateDeliveryBrevoSMTP {
		req.Host = "smtp-relay.brevo.com"
		if req.Port == 0 {
			req.Port = 587
		}
	}
	if req.Host == "" || req.Port < 1 || req.Port > 65535 || req.Username == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "SMTP host, port and username are required")
	}
	if req.AuthProtocol != "plain" && req.AuthProtocol != "login" && req.AuthProtocol != "cram" && req.AuthProtocol != "none" {
		return echo.NewHTTPError(http.StatusBadRequest, "unsupported SMTP authentication protocol")
	}
	if req.TLSType != "none" && req.TLSType != "TLS" && req.TLSType != "STARTTLS" {
		return echo.NewHTTPError(http.StatusBadRequest, "unsupported SMTP TLS mode")
	}
	return nil
}

// applyWorkMateCampaignDelivery prevents a customer from selecting another
// workspace's sender or a shared fallback SMTP server.
func (a *App) applyWorkMateCampaignDelivery(user auth.User, campaign *campReq) error {
	if !isWorkMateCustomer(user) {
		return nil
	}
	settings, err := a.core.GetSettings()
	if err != nil {
		return err
	}
	name := workMateDeliverySMTPName(user, workMateDeliverySMTP)
	brevoSMTPName := workMateDeliverySMTPName(user, workMateDeliveryBrevoSMTP)
	brevoName := workMateDeliveryBrevoName(user)
	for _, messenger := range settings.Messengers {
		if messenger.Name == brevoName && messenger.Enabled && messenger.Username != "" {
			campaign.Messenger = brevoName
			campaign.FromEmail = messenger.Username
			return nil
		}
	}
	for _, smtp := range settings.SMTP {
		if (smtp.Name == name || smtp.Name == brevoSMTPName) && smtp.Enabled && len(smtp.FromAddresses) == 1 {
			campaign.Messenger = smtp.Name
			campaign.FromEmail = smtp.FromAddresses[0]
			return nil
		}
	}
	return echo.NewHTTPError(http.StatusBadRequest, "set up Delivery for this workspace before creating or sending a campaign")
}

// workMateCampaignDeliveryVerified re-checks a persisted campaign immediately
// before schedule or send. A saved-but-unverified connection is disabled, and
// changing a connection invalidates the old campaign messenger until it is
// re-verified.
func (a *App) workMateCampaignDeliveryVerified(user auth.User, campaign *models.Campaign) (bool, error) {
	if !isWorkMateCustomer(user) {
		return true, nil
	}
	settings, err := a.core.GetSettings()
	if err != nil {
		return false, err
	}
	brevoName := workMateDeliveryBrevoName(user)
	if campaign.Messenger == brevoName {
		for _, messenger := range settings.Messengers {
			if messenger.Name == brevoName && messenger.Enabled && messenger.Username == campaign.FromEmail {
				return true, nil
			}
		}
		return false, nil
	}
	for _, provider := range []string{workMateDeliverySMTP, workMateDeliveryBrevoSMTP} {
		name := workMateDeliverySMTPName(user, provider)
		if campaign.Messenger != name {
			continue
		}
		for _, smtp := range settings.SMTP {
			if smtp.Name == name && smtp.Enabled && len(smtp.FromAddresses) == 1 && smtp.FromAddresses[0] == campaign.FromEmail {
				return true, nil
			}
		}
		return false, nil
	}
	return false, nil
}
