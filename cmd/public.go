package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/knadh/listmonk/internal/captcha"
	"github.com/knadh/listmonk/internal/i18n"
	"github.com/knadh/listmonk/internal/manager"
	"github.com/knadh/listmonk/internal/notifs"
	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
)

const (
	tplMessage = "message"
)

// tplRenderer wraps a template.tplRenderer for echo.
type tplRenderer struct {
	templates           *template.Template
	SiteName            string
	RootURL             string
	LogoURL             string
	FaviconURL          string
	AssetVersion        string
	EnablePublicSubPage bool
	EnablePublicArchive bool
	IndividualTracking  bool
}

// tplData is the data container that is injected
// into public templates for accessing data.
type tplData struct {
	SiteName            string
	RootURL             string
	LogoURL             string
	FaviconURL          string
	AssetVersion        string
	EnablePublicSubPage bool
	EnablePublicArchive bool
	IndividualTracking  bool
	Data                any
	L                   *i18n.I18n
}

type publicTpl struct {
	Title       string
	Description string
}

type unsubTpl struct {
	publicTpl
	Subscriber       models.Subscriber
	Subscriptions    []models.Subscription
	SubUUID          string
	AllowBlocklist   bool
	AllowExport      bool
	AllowWipe        bool
	AllowPreferences bool
	ShowManage       bool
}

type optinReq struct {
	SubUUID   string
	ListUUIDs []string      `query:"l" form:"l"`
	Lists     []models.List `query:"-" form:"-"`
}

type optinTpl struct {
	publicTpl
	optinReq
}

type msgTpl struct {
	publicTpl
	MessageTitle string
	Message      string
}

type subFormTpl struct {
	publicTpl
	Lists   []models.List
	Form    models.PublicSubscriptionForm
	Captcha struct {
		Enabled    bool
		Provider   string
		Key        string
		Complexity int
	}
}

var publicFormFieldKeyRE = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,63}$`)

func normalizeSubscriptionForm(in models.PublicSubscriptionForm) (models.PublicSubscriptionForm, bool) {
	out := in
	out.Fields = nil
	seen := map[string]bool{}
	for _, inField := range in.Fields {
		field := inField
		field.Key = strings.TrimSpace(field.Key)
		field.Type = strings.ToLower(strings.TrimSpace(field.Type))
		field.Label = strings.TrimSpace(field.Label)
		if field.Type != "text" && field.Type != "email" && field.Type != "select" && field.Type != "checkbox" && field.Type != "consent" {
			return models.PublicSubscriptionForm{}, false
		}
		if field.Type == "consent" {
			field.Required = true
		}
		if field.Label == "" || (field.Key == "" && field.Type != "consent") || (field.Key != "" && (!publicFormFieldKeyRE.MatchString(field.Key) || seen[field.Key])) {
			return models.PublicSubscriptionForm{}, false
		}
		if field.Key != "" {
			seen[field.Key] = true
		}
		if field.Type == "select" {
			options := make([]string, 0, len(field.Options))
			values := map[string]bool{}
			for _, option := range field.Options {
				option = strings.TrimSpace(option)
				if option == "" || values[option] {
					return models.PublicSubscriptionForm{}, false
				}
				values[option] = true
				options = append(options, option)
			}
			if len(options) == 0 {
				return models.PublicSubscriptionForm{}, false
			}
			field.Options = options
		} else {
			field.Options = nil
		}
		out.Fields = append(out.Fields, field)
	}
	return out, true
}

func selectPublicLists(lists []models.List, requested []string) []models.List {
	if len(requested) == 0 {
		return lists
	}
	byUUID := make(map[string]models.List, len(lists))
	for _, list := range lists {
		byUUID[list.UUID] = list
	}
	out := make([]models.List, 0, len(requested))
	for _, uuid := range requested {
		if list, ok := byUUID[uuid]; ok {
			out = append(out, list)
		}
	}
	return out
}

func resolveSubscriptionForm(global models.PublicSubscriptionForm, lists []models.List, requested []string) models.PublicSubscriptionForm {
	for _, uuid := range requested {
		for _, list := range lists {
			if list.UUID != uuid || list.Attribs == nil {
				continue
			}
			raw, ok := list.Attribs["hosted_form"]
			if !ok {
				break
			}
			data, err := json.Marshal(raw)
			if err != nil {
				break
			}
			var form models.PublicSubscriptionForm
			if err := json.Unmarshal(data, &form); err == nil {
				if form, ok := normalizeSubscriptionForm(form); ok {
					return form
				}
			}
			break
		}
	}
	return global
}

func collectSubscriptionFormAttribs(values url.Values, jsonAttribs map[string]string, fields []models.PublicSubscriptionFormField) (models.JSON, error) {
	allowed := make(map[string]models.PublicSubscriptionFormField, len(fields))
	keylessConsent := false
	for _, field := range fields {
		if field.Key != "" {
			allowed[field.Key] = field
		} else if field.Type == "consent" {
			keylessConsent = true
		}
	}
	if jsonAttribs != nil {
		for key := range jsonAttribs {
			if _, ok := allowed[key]; !ok && (key != "" || !keylessConsent) {
				return nil, echo.NewHTTPError(http.StatusBadRequest, "Please use the fields on this form.")
			}
		}
	} else {
		for key := range values {
			if strings.HasPrefix(key, "attribs.") {
				if _, ok := allowed[strings.TrimPrefix(key, "attribs.")]; !ok {
					return nil, echo.NewHTTPError(http.StatusBadRequest, "Please use the fields on this form.")
				}
			}
		}
	}

	attribs := models.JSON{}
	for _, field := range fields {
		value := ""
		if jsonAttribs != nil {
			value = strings.TrimSpace(jsonAttribs[field.Key])
		} else if field.Key == "" {
			value = strings.TrimSpace(values.Get("consent"))
		} else {
			value = strings.TrimSpace(values.Get("attribs." + field.Key))
		}
		switch field.Type {
		case "checkbox", "consent":
			checked := value == "true" || value == "on" || value == "1"
			if field.Required && !checked {
				return nil, echo.NewHTTPError(http.StatusBadRequest, "Please complete the required fields.")
			}
			if field.Key != "" {
				attribs[field.Key] = checked
			}
		case "select":
			if value == "" && !field.Required {
				continue
			}
			valid := false
			for _, option := range field.Options {
				if value == option {
					valid = true
					break
				}
			}
			if !valid {
				return nil, echo.NewHTTPError(http.StatusBadRequest, "Please choose an option from the list.")
			}
			attribs[field.Key] = value
		default:
			if value == "" {
				if field.Required {
					return nil, echo.NewHTTPError(http.StatusBadRequest, "Please complete the required fields.")
				}
				continue
			}
			if len(value) > stdInputMaxLen || (field.Type == "email" && !strings.Contains(value, "@")) {
				return nil, echo.NewHTTPError(http.StatusBadRequest, "Please enter a valid answer.")
			}
			attribs[field.Key] = value
		}
	}
	return attribs, nil
}

func mergeSubscriptionAttribs(existing, accepted models.JSON) models.JSON {
	out := models.JSON{}
	for key, value := range existing {
		out[key] = value
	}
	for key, value := range accepted {
		out[key] = value
	}
	return out
}

var (
	// pixelPNG is a 1x1 transparent PNG served as the tracking pixel.
	// Using the canonical 1x1 size (industry standard for tracking pixels).
	pixelPNG = drawTransparentImage(1, 1)
)

// Render executes and renders a template for echo.
func (t *tplRenderer) Render(w io.Writer, name string, data any, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, tplData{
		SiteName:            t.SiteName,
		RootURL:             t.RootURL,
		LogoURL:             t.LogoURL,
		FaviconURL:          t.FaviconURL,
		AssetVersion:        t.AssetVersion,
		EnablePublicSubPage: t.EnablePublicSubPage,
		EnablePublicArchive: t.EnablePublicArchive,
		IndividualTracking:  t.IndividualTracking,
		Data:                data,
		L:                   c.Get("app").(*App).i18n,
	})
}

// GetPublicLists returns the list of public lists with minimal fields
// required to submit a subscription.
func (a *App) GetPublicLists(c echo.Context) error {
	// Get all public lists.
	lists, err := a.core.GetLists(models.ListTypePublic, models.ListStatusActive, true, nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.T("public.errorFetchingLists"))
	}

	type list struct {
		UUID string `json:"uuid"`
		Name string `json:"name"`
	}

	out := make([]list, 0, len(lists))
	for _, l := range lists {
		out = append(out, list{
			UUID: l.UUID,
			Name: l.Name,
		})
	}

	return c.JSON(http.StatusOK, out)
}

// ViewCampaignMessage renders the HTML view of a campaign message.
// This is the view the {{ MessageURL }} template tag links to in e-mail campaigns.
func (a *App) ViewCampaignMessage(c echo.Context) error {
	// Get the campaign.
	campUUID := c.Param("campUUID")
	camp, err := a.core.GetCampaign(0, campUUID, "")
	if err != nil {
		if er, ok := err.(*echo.HTTPError); ok {
			if er.Code == http.StatusBadRequest {
				return c.Render(http.StatusNotFound, tplMessage,
					makeMsgTpl(a.i18n.T("public.notFoundTitle"), "", a.i18n.T("public.campaignNotFound")))
			}
		}

		return c.Render(http.StatusInternalServerError, tplMessage,
			makeMsgTpl(a.i18n.T("public.errorTitle"), "", a.i18n.Ts("public.errorFetchingCampaign")))
	}

	// Get the subscriber.
	subUUID := c.Param("subUUID")
	sub, err := a.core.GetSubscriber(0, subUUID, "")
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Render(http.StatusNotFound, tplMessage,
				makeMsgTpl(a.i18n.T("public.notFoundTitle"), "", a.i18n.T("public.errorFetchingEmail")))
		}

		return c.Render(http.StatusInternalServerError, tplMessage,
			makeMsgTpl(a.i18n.T("public.errorTitle"), "", a.i18n.Ts("public.errorFetchingCampaign")))
	}

	// Compile the template.
	if err := camp.CompileTemplate(a.manager.TemplateFuncs(&camp)); err != nil {
		a.log.Printf("error compiling template: %v", err)
		return c.Render(http.StatusInternalServerError, tplMessage,
			makeMsgTpl(a.i18n.T("public.errorTitle"), "", a.i18n.Ts("public.errorFetchingCampaign")))
	}

	// Render the message body.
	msg, err := a.manager.NewCampaignMessage(&camp, sub)
	if err != nil {
		a.log.Printf("error rendering message: %v", err)
		return c.Render(http.StatusInternalServerError, tplMessage,
			makeMsgTpl(a.i18n.T("public.errorTitle"), "", a.i18n.Ts("public.errorFetchingCampaign")))
	}

	return c.HTML(http.StatusOK, string(msg.Body()))
}

// SubscriptionPage renders the subscription management page and handles unsubscriptions.
// This is the view that {{ UnsubscribeURL }} in campaigns link to.
func (a *App) SubscriptionPage(c echo.Context) error {
	var (
		subUUID       = c.Param("subUUID")
		showManage, _ = strconv.ParseBool(c.FormValue("manage"))
	)

	// Get the subscriber from the DB.
	s, err := a.core.GetSubscriber(0, subUUID, "")
	if err != nil {
		return c.Render(http.StatusInternalServerError, tplMessage,
			makeMsgTpl(a.i18n.T("public.errorTitle"), "", a.i18n.Ts("public.errorProcessingRequest")))
	}

	// Prepare the public template.
	out := unsubTpl{
		Subscriber:       s,
		SubUUID:          subUUID,
		publicTpl:        publicTpl{Title: a.i18n.T("public.unsubscribeTitle")},
		AllowBlocklist:   a.cfg.Privacy.AllowBlocklist,
		AllowExport:      a.cfg.Privacy.AllowExport,
		AllowWipe:        a.cfg.Privacy.AllowWipe,
		AllowPreferences: a.cfg.Privacy.AllowPreferences,
	}

	// If the subscriber is blocklisted, throw an error.
	if s.Status == models.SubscriberStatusBlockListed {
		return c.Render(http.StatusOK, tplMessage, makeMsgTpl(a.i18n.T("public.noSubTitle"), "", a.i18n.Ts("public.blocklisted")))
	}

	// Only show preference management if it's enabled in settings.
	if a.cfg.Privacy.AllowPreferences {
		out.ShowManage = showManage

		// Get the subscriber's lists from the DB to render in the template.
		subs, err := a.core.GetSubscriptions(0, subUUID, false)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, a.i18n.T("public.errorFetchingLists"))
		}

		out.Subscriptions = make([]models.Subscription, 0, len(subs))
		for _, s := range subs {
			// Private lists shouldn't be rendered in the template.
			if s.Type == models.ListTypePrivate {
				continue
			}

			out.Subscriptions = append(out.Subscriptions, s)
		}
	}

	return c.Render(http.StatusOK, "subscription", out)
}

// SubscriptionPrefs renders the subscription management page and
// s unsubscriptions. This is the view that {{ UnsubscribeURL }} in
// campaigns link to.
func (a *App) SubscriptionPrefs(c echo.Context) error {
	// Read the form.
	var req struct {
		Name      string   `form:"name" json:"name"`
		ListUUIDs []string `form:"l" json:"list_uuids"`
		Blocklist bool     `form:"blocklist" json:"blocklist"`
		Manage    bool     `form:"manage" json:"manage"`
	}
	if err := c.Bind(&req); err != nil {
		return c.Render(http.StatusBadRequest, tplMessage,
			makeMsgTpl(a.i18n.T("public.errorTitle"), "", a.i18n.T("globals.messages.invalidData")))
	}

	// Simple unsubscribe.
	var (
		campUUID  = c.Param("campUUID")
		subUUID   = c.Param("subUUID")
		blocklist = a.cfg.Privacy.AllowBlocklist && req.Blocklist
	)
	if !req.Manage || blocklist {
		if err := a.core.UnsubscribeByCampaign(subUUID, campUUID, blocklist); err != nil {
			return c.Render(http.StatusInternalServerError, tplMessage,
				makeMsgTpl(a.i18n.T("public.errorTitle"), "", a.i18n.T("public.errorProcessingRequest")))
		}

		return c.Render(http.StatusOK, tplMessage,
			makeMsgTpl(a.i18n.T("public.unsubbedTitle"), "", a.i18n.T("public.unsubbedInfo")))
	}

	// Is preference management enabled?
	if !a.cfg.Privacy.AllowPreferences {
		return c.Render(http.StatusBadRequest, tplMessage,
			makeMsgTpl(a.i18n.T("public.errorTitle"), "", a.i18n.T("public.invalidFeature")))
	}

	// Manage preferences.
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 256 {
		return c.Render(http.StatusBadRequest, tplMessage,
			makeMsgTpl(a.i18n.T("public.errorTitle"), "", a.i18n.T("subscribers.invalidName")))
	}

	// Get the subscriber from the DB.
	sub, err := a.core.GetSubscriber(0, subUUID, "")
	if err != nil {
		return c.Render(http.StatusInternalServerError, tplMessage,
			makeMsgTpl(a.i18n.T("public.errorTitle"), "", a.i18n.Ts("globals.messages.pFound",
				"name", a.i18n.T("globals.terms.subscriber"))))
	}
	sub.Name = req.Name

	// Update the subscriber properties in the DB.
	if _, err := a.core.UpdateSubscriber(sub.ID, sub); err != nil {
		return c.Render(http.StatusInternalServerError, tplMessage,
			makeMsgTpl(a.i18n.T("public.errorTitle"), "", a.i18n.T("public.errorProcessingRequest")))
	}

	// Get the subscriber's lists and whatever is not sent in the request (unchecked),
	// unsubscribe them.
	reqUUIDs := make(map[string]struct{})
	for _, u := range req.ListUUIDs {
		reqUUIDs[u] = struct{}{}
	}

	// Get subscription from teh DB.
	subs, err := a.core.GetSubscriptions(0, subUUID, false)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.T("public.errorFetchingLists"))
	}

	// Filter the lists in the request against the subscriptions in the DB.
	unsubUUIDs := make([]string, 0, len(req.ListUUIDs))
	for _, s := range subs {
		if s.Type == models.ListTypePrivate {
			continue
		}
		if _, ok := reqUUIDs[s.UUID]; !ok {
			unsubUUIDs = append(unsubUUIDs, s.UUID)
		}
	}

	// Unsubscribe from lists.
	if err := a.core.UnsubscribeLists([]int{sub.ID}, nil, unsubUUIDs); err != nil {
		return c.Render(http.StatusInternalServerError, tplMessage,
			makeMsgTpl(a.i18n.T("public.errorTitle"), "", a.i18n.T("public.errorProcessingRequest")))

	}

	return c.Render(http.StatusOK, tplMessage,
		makeMsgTpl(a.i18n.T("globals.messages.done"), "", a.i18n.T("public.prefsSaved")))
}

// OptinPage renders the double opt-in confirmation page that subscribers
// see when they click on the "Confirm subscription" button in double-optin
// notifications.
func (a *App) OptinPage(c echo.Context) error {
	var (
		subUUID    = c.Param("subUUID")
		confirm, _ = strconv.ParseBool(c.FormValue("confirm"))
		req        optinReq
	)
	if err := c.Bind(&req); err != nil {
		return err
	}

	// Validate list UUIDs if there are incoming UUIDs in the request.
	if len(req.ListUUIDs) > 0 {
		for _, l := range req.ListUUIDs {
			if !reUUID.MatchString(l) {
				return c.Render(http.StatusBadRequest, tplMessage,
					makeMsgTpl(a.i18n.T("public.errorTitle"), "", a.i18n.T("globals.messages.invalidUUID")))
			}
		}
	}

	// Get the list of subscription lists where the subscriber hasn't confirmed.
	lists, err := a.core.GetSubscriberLists(0, subUUID, nil, req.ListUUIDs, models.SubscriptionStatusUnconfirmed, "")
	if err != nil {
		return c.Render(http.StatusInternalServerError, tplMessage,
			makeMsgTpl(a.i18n.T("public.errorTitle"), "", a.i18n.Ts("public.errorFetchingLists")))
	}

	// There are no lists to confirm.
	if len(lists) == 0 {
		return c.Render(http.StatusOK, tplMessage,
			makeMsgTpl(a.i18n.T("public.noSubTitle"), "", a.i18n.Ts("public.noSubInfo")))
	}

	if confirm || !a.cfg.ShowOptinPage {
		return a.confirmOptinSubscription(c, subUUID, req.ListUUIDs, lists)
	}

	var out optinTpl
	out.Lists = lists
	out.SubUUID = subUUID
	out.Title = a.i18n.T("public.confirmOptinSubTitle")

	return c.Render(http.StatusOK, "optin", out)
}

func (a *App) confirmOptinSubscription(c echo.Context, subUUID string, listUUIDs []string, lists []models.List) error {
	if len(listUUIDs) == 0 {
		listUUIDs = make([]string, 0, len(lists))
		for _, l := range lists {
			listUUIDs = append(listUUIDs, l.UUID)
		}
	}

	meta := models.JSON{}
	if a.cfg.Privacy.RecordOptinIP {
		if h := c.Request().Header.Get("X-Forwarded-For"); h != "" {
			meta["optin_ip"] = h
		} else if h := c.Request().RemoteAddr; h != "" {
			meta["optin_ip"] = strings.Split(h, ":")[0]
		}
	}

	if err := a.core.ConfirmOptionSubscription(subUUID, listUUIDs, meta); err != nil {
		a.log.Printf("error confirming opt-in subscription: %v", err)
		return c.Render(http.StatusInternalServerError, tplMessage,
			makeMsgTpl(a.i18n.T("public.errorTitle"), "", a.i18n.Ts("public.errorProcessingRequest")))
	}

	return c.Render(http.StatusOK, tplMessage,
		makeMsgTpl(a.i18n.T("public.subConfirmedTitle"), "", a.i18n.Ts("public.subConfirmed")))
}

// SubscriptionFormPage handles subscription requests coming from public
// HTML subscription forms.
func (a *App) SubscriptionFormPage(c echo.Context) error {
	if !a.cfg.EnablePublicSubPage {
		return c.Render(http.StatusNotFound, tplMessage,
			makeMsgTpl(a.i18n.T("public.errorTitle"), "", a.i18n.Ts("public.invalidFeature")))
	}

	// Get all public lists from the DB.
	lists, err := a.core.GetLists(models.ListTypePublic, models.ListStatusActive, true, nil)
	if err != nil {
		return c.Render(http.StatusInternalServerError, tplMessage,
			makeMsgTpl(a.i18n.T("public.errorTitle"), "", a.i18n.Ts("public.errorFetchingLists")))
	}

	// There are no public lists available for subscription.
	if len(lists) == 0 {
		return c.Render(http.StatusInternalServerError, tplMessage,
			makeMsgTpl(a.i18n.T("public.errorTitle"), "", a.i18n.Ts("public.noListsAvailable")))
	}

	out := subFormTpl{}
	out.Title = a.i18n.T("public.sub")
	settings, err := a.core.GetSettings()
	if err != nil {
		return c.Render(http.StatusInternalServerError, tplMessage,
			makeMsgTpl(a.i18n.T("public.errorTitle"), "", a.i18n.Ts("public.errorFetchingLists")))
	}
	globalForm, _ := normalizeSubscriptionForm(settings.AppPublicSubscriptionForm)
	requested := c.QueryParams()["l"]
	out.Lists = selectPublicLists(lists, requested)
	if len(out.Lists) != len(requested) && len(requested) > 0 {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.T("globals.messages.invalidUUID"))
	}
	out.Form = resolveSubscriptionForm(globalForm, out.Lists, requested)

	// Captcha configuration for template rendering.
	if a.cfg.Security.Captcha.Altcha.Enabled {
		out.Captcha.Enabled = true
		out.Captcha.Provider = "altcha"
		out.Captcha.Complexity = a.cfg.Security.Captcha.Altcha.Complexity
	} else if a.cfg.Security.Captcha.HCaptcha.Enabled {
		out.Captcha.Enabled = true
		out.Captcha.Provider = "hcaptcha"
		out.Captcha.Key = a.cfg.Security.Captcha.HCaptcha.Key
	}

	return c.Render(http.StatusOK, "subscription-form", out)
}

// SubscriptionForm handles subscription requests coming from public
// HTML subscription forms.
func (a *App) SubscriptionForm(c echo.Context) error {
	if !a.cfg.EnablePublicSubPage {
		return echo.NewHTTPError(http.StatusNotFound, a.i18n.T("public.invalidFeature"))

	}

	// If there's a nonce value, a bot could've filled the form.
	if c.FormValue("nonce") != "" {
		return echo.NewHTTPError(http.StatusBadGateway, a.i18n.T("public.invalidFeature"))
	}

	hasOptin, err := a.processSubForm(c)
	if err != nil {
		e, ok := err.(*echo.HTTPError)
		if !ok {
			return err
		}

		return c.Render(e.Code, tplMessage, makeMsgTpl(a.i18n.T("public.errorTitle"), "", fmt.Sprintf("%s", e.Message)))
	}

	// Redirect to a custom page if a trusted '?next' is set.
	if nextURL := strings.TrimSpace(c.FormValue("next")); nextURL != "" {
		for _, d := range a.cfg.Security.TrustedURLs {
			if d != "*" && nextURL == d {
				return c.Redirect(http.StatusSeeOther, nextURL)
			}
		}
	}

	// If there were double optin lists, show the opt-in pending message instead of
	// the subscription confirmation message.
	msg := "public.subConfirmed"
	if hasOptin {
		msg = "public.subOptinPending"
	}

	return c.Render(http.StatusOK, tplMessage, makeMsgTpl(a.i18n.T("public.subTitle"), "", a.i18n.Ts(msg)))
}

// PublicSubscription handles subscription requests coming from public
// API calls.
func (a *App) PublicSubscription(c echo.Context) error {
	if !a.cfg.EnablePublicSubPage {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.T("public.invalidFeature"))
	}

	hasOptin, err := a.processSubForm(c)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, okResp{struct {
		HasOptin bool `json:"has_optin"`
	}{hasOptin}})
}

// LinkRedirect redirects a link UUID to its original underlying link
// after recording the link click for a particular subscriber in the particular
// campaign. These links are generated by {{ TrackLink }} tags in campaigns.
func (a *App) LinkRedirect(c echo.Context) error {
	var (
		linkUUID = c.Param("linkUUID")
		campUUID = c.Param("campUUID")
	)

	// If tracking is globally disabled, resolve the URL without recording a click.
	if a.cfg.Privacy.DisableTracking {
		url, err := a.core.GetLinkURL(linkUUID)
		if err != nil {
			e := err.(*echo.HTTPError)
			return c.Render(e.Code, tplMessage, makeMsgTpl(a.i18n.T("public.errorTitle"), "", e.Error()))
		}
		return c.Redirect(http.StatusTemporaryRedirect, url)
	}

	// If individual tracking is disabled, do not record the subscriber ID.
	subUUID := c.Param("subUUID")
	if !a.cfg.Privacy.IndividualTracking {
		subUUID = ""
	}

	url, err := a.core.RegisterCampaignLinkClick(linkUUID, campUUID, subUUID)
	if err != nil {
		e := err.(*echo.HTTPError)
		return c.Render(e.Code, tplMessage, makeMsgTpl(a.i18n.T("public.errorTitle"), "", e.Error()))
	}

	return c.Redirect(http.StatusTemporaryRedirect, url)
}

// RegisterCampaignView registers a campaign view which comes in
// the form of an pixel image request. Regardless of errors, this handler
// should always render the pixel image bytes. The pixel URL is generated by
// the {{ TrackView }} template tag in campaigns.
func (a *App) RegisterCampaignView(c echo.Context) error {
	// If tracking is globally disabled, return the pixel without recording.
	if a.cfg.Privacy.DisableTracking {
		c.Response().Header().Set("Cache-Control", "no-cache")
		return c.Blob(http.StatusOK, "image/png", pixelPNG)
	}

	// If individual tracking is disabled, do not record the subscriber ID.
	subUUID := c.Param("subUUID")
	if !a.cfg.Privacy.IndividualTracking {
		subUUID = ""
	}

	// Exclude dummy hits from template previews.
	campUUID := c.Param("campUUID")
	if campUUID != dummyUUID && subUUID != dummyUUID {
		if err := a.core.RegisterCampaignView(campUUID, subUUID); err != nil {
			a.log.Printf("error registering campaign view: %s", err)
		}
	}

	c.Response().Header().Set("Cache-Control", "no-cache")
	return c.Blob(http.StatusOK, "image/png", pixelPNG)
}

// SelfExportSubscriberData pulls the subscriber's profile, list subscriptions,
// campaign views and clicks and produces a JSON report that is then e-mailed
// to the subscriber. This is a privacy feature and the data that's exported
// is dependent on the configuration.
func (a *App) SelfExportSubscriberData(c echo.Context) error {
	// Is export allowed?
	if !a.cfg.Privacy.AllowExport {
		return c.Render(http.StatusBadRequest, tplMessage,
			makeMsgTpl(a.i18n.T("public.errorTitle"), "", a.i18n.Ts("public.invalidFeature")))
	}

	// Get the subscriber's data. A single query that gets the profile,
	// list subscriptions, campaign views, and link clicks. Names of
	// private lists are replaced with "Private list".
	subUUID := c.Param("subUUID")
	data, b, err := a.exportSubscriberData(0, subUUID, a.cfg.Privacy.Exportable)
	if err != nil {
		a.log.Printf("error exporting subscriber data: %s", err)
		return c.Render(http.StatusInternalServerError, tplMessage,
			makeMsgTpl(a.i18n.T("public.errorTitle"), "", a.i18n.Ts("public.errorProcessingRequest")))
	}

	// Prepare the attachment e-mail.
	var msg bytes.Buffer
	if err := notifs.Tpls.ExecuteTemplate(&msg, notifs.TplSubscriberData, data); err != nil {
		a.log.Printf("error compiling notification template '%s': %v", notifs.TplSubscriberData, err)
		return c.Render(http.StatusInternalServerError, tplMessage,
			makeMsgTpl(a.i18n.T("public.errorTitle"), "", a.i18n.Ts("public.errorProcessingRequest")))
	}

	// TODO: GetTplSubject should be moved to a utils package.
	subject, body := notifs.GetTplSubject(a.i18n.Ts("email.data.title"), msg.Bytes())

	// E-mail the data as a JSON attachment to the subscriber.
	const fname = "data.json"
	if err := a.emailMsgr.Push(models.Message{
		From:    a.cfg.FromEmail,
		To:      []string{data.Email},
		Subject: subject,
		Body:    body,
		Attachments: []models.Attachment{
			{
				Name:    fname,
				Content: b,
				Header:  manager.MakeAttachmentHeader(fname, "base64", "application/json"),
			},
		},
	}); err != nil {
		a.log.Printf("error e-mailing subscriber profile: %s", err)
		return c.Render(http.StatusInternalServerError, tplMessage,
			makeMsgTpl(a.i18n.T("public.errorTitle"), "", a.i18n.Ts("public.errorProcessingRequest")))
	}

	return c.Render(http.StatusOK, tplMessage,
		makeMsgTpl(a.i18n.T("public.dataSentTitle"), "", a.i18n.T("public.dataSent")))
}

// WipeSubscriberData allows a subscriber to delete their data. The
// profile and subscriptions are deleted, while the campaign_views and link
// clicks remain as orphan data unconnected to any subscriber.
func (a *App) WipeSubscriberData(c echo.Context) error {
	// Is wiping allowed?
	if !a.cfg.Privacy.AllowWipe {
		return c.Render(http.StatusBadRequest, tplMessage,
			makeMsgTpl(a.i18n.T("public.errorTitle"), "", a.i18n.Ts("public.invalidFeature")))
	}

	subUUID := c.Param("subUUID")
	if err := a.core.DeleteSubscribers(nil, []string{subUUID}); err != nil {
		a.log.Printf("error wiping subscriber data: %s", err)
		return c.Render(http.StatusInternalServerError, tplMessage,
			makeMsgTpl(a.i18n.T("public.errorTitle"), "", a.i18n.Ts("public.errorProcessingRequest")))
	}

	return c.Render(http.StatusOK, tplMessage,
		makeMsgTpl(a.i18n.T("public.dataRemovedTitle"), "", a.i18n.T("public.dataRemoved")))
}

// AltchaChallenge generates a challenge for Altcha captcha.
func (a *App) AltchaChallenge(c echo.Context) error {
	// Check if Altcha is enabled.
	if !a.captcha.IsEnabled() || a.captcha.GetProvider() != captcha.ProviderAltcha {
		return echo.NewHTTPError(http.StatusNotFound, "captcha not enabled")
	}

	// Generate challenge.
	out, err := a.captcha.GenerateChallenge()
	if err != nil {
		a.log.Printf("error generating altcha challenge: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error generating challenge")
	}

	// Return the challenge as JSON.
	c.Response().Header().Set("Content-Type", "application/json")
	return c.String(http.StatusOK, out)
}

// drawTransparentImage draws a transparent PNG of given dimensions
// and returns the PNG bytes.
func drawTransparentImage(h, w int) []byte {
	var (
		img = image.NewRGBA(image.Rect(0, 0, w, h))
		out = &bytes.Buffer{}
	)
	_ = png.Encode(out, img)

	return out.Bytes()
}

// processSubForm processes an incoming form/public API subscription request.
// The bool indicates whether there was subscription to an optin list so that
// an appropriate message can be shown.
func (a *App) processSubForm(c echo.Context) (bool, error) {
	// Get and validate fields.
	var req struct {
		Name          string            `form:"name" json:"name"`
		Email         string            `form:"email" json:"email"`
		FormListUUIDs []string          `form:"l" json:"list_uuids"`
		Attribs       map[string]string `json:"attribs"`
		HCaptcha      string            `form:"h-captcha-response" json:"h-captcha-response"`
		Altcha        string            `form:"altcha" json:"altcha"`
	}
	if err := c.Bind(&req); err != nil {
		return false, err
	}

	if a.captcha.IsEnabled() {
		var val string
		switch a.captcha.GetProvider() {
		case captcha.ProviderHCaptcha:
			val = req.HCaptcha
		case captcha.ProviderAltcha:
			val = req.Altcha
		default:
			return false, echo.NewHTTPError(http.StatusBadRequest, a.i18n.T("public.invalidCaptcha"))
		}
		if val == "" {
			return false, echo.NewHTTPError(http.StatusBadRequest, a.i18n.T("public.invalidCaptcha"))
		}
		if err, ok := a.captcha.Verify(val); err != nil || !ok {
			if err != nil {
				a.log.Printf("captcha request failed: %v", err)
			}
			return false, echo.NewHTTPError(http.StatusBadRequest, a.i18n.T("public.invalidCaptcha"))
		}
	}

	if len(req.FormListUUIDs) == 0 {
		return false, echo.NewHTTPError(http.StatusBadRequest, a.i18n.T("public.noListsSelected"))
	}

	// Validate fields.
	if len(req.Email) > 1000 {
		return false, echo.NewHTTPError(http.StatusBadRequest, a.i18n.T("subscribers.invalidEmail"))
	}

	em, err := a.importer.SanitizeEmail(req.Email)
	if err != nil {
		return false, echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	req.Email = em

	req.Name = strings.TrimSpace(req.Name)
	if len(req.Name) == 0 {
		// If there's no name, use the name bit from the e-mail.
		req.Name = strings.Split(req.Email, "@")[0]
	} else if len(req.Name) > stdInputMaxLen {
		return false, echo.NewHTTPError(http.StatusBadRequest, a.i18n.T("subscribers.invalidName"))
	}

	listUUIDs := pq.StringArray(req.FormListUUIDs)

	// Fetch the list types and ensure that they are not private.
	listTypes, err := a.core.GetListTypes(nil, req.FormListUUIDs)
	if err != nil {
		return false, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("%s", err.(*echo.HTTPError).Message))
	}

	for _, t := range listTypes {
		if t == models.ListTypePrivate {
			return false, echo.NewHTTPError(http.StatusBadRequest, a.i18n.T("globals.messages.invalidUUID"))
		}
	}

	lists, err := a.core.GetLists(models.ListTypePublic, models.ListStatusActive, true, nil)
	if err != nil {
		return false, err
	}
	selectedLists := selectPublicLists(lists, req.FormListUUIDs)
	if len(selectedLists) != len(req.FormListUUIDs) {
		return false, echo.NewHTTPError(http.StatusBadRequest, a.i18n.T("globals.messages.invalidUUID"))
	}

	settings, err := a.core.GetSettings()
	if err != nil {
		return false, err
	}
	globalForm, _ := normalizeSubscriptionForm(settings.AppPublicSubscriptionForm)
	form := resolveSubscriptionForm(globalForm, selectedLists, req.FormListUUIDs)
	attribs := models.JSON{}
	if len(form.Fields) > 0 {
		values, err := c.FormParams()
		if err != nil {
			return false, err
		}
		attribs, err = collectSubscriptionFormAttribs(values, req.Attribs, form.Fields)
		if err != nil {
			return false, err
		}
	}

	// Insert the subscriber into the DB.
	_, hasOptin, err := a.core.InsertSubscriber(models.Subscriber{
		Name:    req.Name,
		Email:   req.Email,
		Attribs: attribs,
		Status:  models.SubscriberStatusEnabled,
	}, nil, listUUIDs, false, true)
	if err == nil {
		return hasOptin, nil
	}

	// Insert returned an error. Examine it.
	var lastErr = err

	// Subscriber already exists. Update subscriptions in the DB.
	if e, ok := err.(*echo.HTTPError); ok && e.Code == http.StatusConflict {
		// Get the subscriber from the DB by their email.
		sub, err := a.core.GetSubscriber(0, "", req.Email)
		if err != nil {
			return false, err
		}
		sub.Attribs = mergeSubscriptionAttribs(sub.Attribs, attribs)

		// Update the subscriber's subscriptions in the DB.
		_, hasOptin, err := a.core.UpdateSubscriberWithLists(sub.ID, sub, nil, listUUIDs, false, false, true, nil, true)
		if err == nil {
			return hasOptin, nil
		}
		lastErr = err
	}

	// Something else went wrong.
	if e, ok := lastErr.(*echo.HTTPError); ok {
		return false, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("%s", e.Message))
	}
	return false, echo.NewHTTPError(http.StatusInternalServerError, a.i18n.T("public.errorProcessingRequest"))
}
