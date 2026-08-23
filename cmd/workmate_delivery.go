package main

import (
	"net/http"
	"net/mail"
	"strings"

	"github.com/knadh/listmonk/internal/auth"
	"github.com/knadh/listmonk/internal/messenger/email"
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
	Provider     string `json:"provider"`
	SenderName   string `json:"sender_name"`
	FromEmail    string `json:"from_email"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Username     string `json:"username"`
	AuthProtocol string `json:"auth_protocol"`
	TLSType      string `json:"tls_type"`
}

func workMateDeliverySMTPName(user auth.User) string {
	if !user.ListRoleName.Valid {
		return ""
	}
	return "email-" + user.ListRoleName.String
}

func workMateDeliveryBrevoName(user auth.User) string {
	if !user.ListRoleName.Valid {
		return ""
	}
	return "brevo-" + user.ListRoleName.String
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
	name := workMateDeliverySMTPName(user)
	brevoName := workMateDeliveryBrevoName(user)
	for _, messenger := range settings.Messengers {
		if messenger.Name == brevoName && messenger.Enabled {
			return c.JSON(http.StatusOK, okResp{workMateDeliveryResponse{
				Configured: true, Provider: "brevo", SenderName: messenger.RootURL,
				FromEmail: messenger.Username,
			}})
		}
	}
	for _, smtp := range settings.SMTP {
		if smtp.Name == name && smtp.Enabled {
			provider := "smtp"
			if smtp.Host == "smtp-relay.brevo.com" {
				provider = "brevo"
			}
			return c.JSON(http.StatusOK, okResp{workMateDeliveryResponse{
				Configured: true, Provider: provider, SenderName: smtp.Name,
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
	name := workMateDeliverySMTPName(user)
	from := email.NormalizeAddr(req.FromEmail)
	if req.Provider == "brevo" {
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
			settings.Messengers[i].Enabled = true
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
			}{Enabled: true, Name: brevoName, RootURL: req.SenderName, Username: from, Password: req.Password, MaxConns: 5, Timeout: "30s", MaxMsgRetries: 2})
		}
		if err := a.core.UpdateSettings(settings); err != nil {
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
		settings.SMTP[i].Enabled = true
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
			Name: name, Enabled: true, Host: req.Host, Port: req.Port, Username: req.Username,
			Password: req.Password, AuthProtocol: req.AuthProtocol, TLSType: req.TLSType,
			EmailHeaders: []map[string]string{}, MaxConns: 5, MaxMsgRetries: 2,
			MsgRetryDelay: "5m", IdleTimeout: "15s", WaitTimeout: "5s", FromAddresses: []string{from},
		})
	}
	if err := a.core.UpdateSettings(settings); err != nil {
		return err
	}
	return a.handleSettingsRestart(c)
}

func firstWorkMateFromAddress(addresses []string) string {
	if len(addresses) == 0 {
		return ""
	}
	return addresses[0]
}

func validateWorkMateDeliveryRequest(req *workMateDeliveryRequest) error {
	req.Provider = strings.TrimSpace(strings.ToLower(req.Provider))
	if req.Provider != "smtp" && req.Provider != "brevo" {
		return echo.NewHTTPError(http.StatusBadRequest, "delivery provider must be Brevo API or SMTP")
	}
	req.Host = strings.TrimSpace(req.Host)
	req.Username = strings.TrimSpace(req.Username)
	req.FromEmail = email.NormalizeAddr(req.FromEmail)
	parsed, err := mail.ParseAddress(req.FromEmail)
	if err != nil || req.FromEmail == "" || parsed.Address != req.FromEmail {
		return echo.NewHTTPError(http.StatusBadRequest, "a valid sender email is required")
	}
	if req.Provider == "brevo" {
		req.SenderName = strings.TrimSpace(req.SenderName)
		if req.SenderName == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "a Brevo sender name is required")
		}
		return nil
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
	name := workMateDeliverySMTPName(user)
	brevoName := workMateDeliveryBrevoName(user)
	for _, messenger := range settings.Messengers {
		if messenger.Name == brevoName && messenger.Enabled && messenger.Username != "" {
			campaign.Messenger = brevoName
			campaign.FromEmail = messenger.Username
			return nil
		}
	}
	for _, smtp := range settings.SMTP {
		if smtp.Name == name && smtp.Enabled && len(smtp.FromAddresses) == 1 {
			campaign.Messenger = name
			campaign.FromEmail = smtp.FromAddresses[0]
			return nil
		}
	}
	return echo.NewHTTPError(http.StatusBadRequest, "set up Delivery for this workspace before creating or sending a campaign")
}
