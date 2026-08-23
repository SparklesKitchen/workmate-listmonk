package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/knadh/listmonk/internal/auth"
	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
	"gopkg.in/volatiletech/null.v6"
)

const workMateCustomerRoleName = "WorkMate Customer"

// Listmonk is one shared WorkMate runtime. Serialise first-workspace list
// creation inside that runtime so parallel launches cannot create two audience
// lists before WorkMate OS persists the mapping receipt.
var workMateProvisionMu sync.Mutex

// WorkMateAdminSSO accepts a short-lived assertion issued only by the
// authenticated WorkMate SaaS Admin service. It deliberately logs in the
// existing Listmonk super-admin account; no Listmonk credential is exposed to
// the browser or stored by WorkMate OS.
func (a *App) WorkMateAdminSSO(c echo.Context) error {
	secret := strings.TrimSpace(os.Getenv("WORKMATE_ADMIN_SSO_SECRET"))
	assertion, ok := readWorkMateAssertion(c.QueryParam("handoff"), secret, time.Now())
	if !ok || assertion.Kind != "saas-admin" {
		return echo.NewHTTPError(http.StatusForbidden, "invalid WorkMate SaaS Admin handoff")
	}

	user, err := a.core.GetUser(1, "", "")
	if err != nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "Listmonk administrator is unavailable")
	}
	if err := a.auth.SaveSession(user, "", c); err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	c.Response().Header().Set("Referrer-Policy", "no-referrer")
	return c.Redirect(http.StatusFound, uriAdmin)
}

func (a *App) WorkMateProvision(c echo.Context) error {
	secret := strings.TrimSpace(os.Getenv("WORKMATE_CUSTOMER_SSO_SECRET"))
	if secret == "" {
		secret = strings.TrimSpace(os.Getenv("WORKMATE_ADMIN_SSO_SECRET"))
	}
	assertion, ok := readWorkMateAssertion(c.QueryParam("handoff"), secret, time.Now())
	if !ok || assertion.Kind != "workspace-provision" || assertion.Tenant == "" || assertion.Workspace == "" || assertion.Name == "" {
		return echo.NewHTTPError(http.StatusForbidden, "invalid WorkMate provisioning handoff")
	}
	if assertion.ListID > 0 {
		if err := a.ensureWorkMateAudiencePublic(assertion.ListID); err != nil {
			return err
		}
		return c.JSON(http.StatusOK, okResp{map[string]int{"listmonk_list_id": assertion.ListID}})
	}

	workMateProvisionMu.Lock()
	defer workMateProvisionMu.Unlock()
	tag := "workmate-workspace-" + workMateWorkspaceRoleID(assertion.Tenant, assertion.Workspace)
	lists, err := a.core.GetLists("", "", true, nil)
	if err != nil {
		return err
	}
	for _, list := range lists {
		for _, existing := range list.Tags {
			if existing == tag {
				return c.JSON(http.StatusOK, okResp{map[string]int{"listmonk_list_id": list.ID}})
			}
		}
	}
	list, err := a.core.CreateList(models.List{Name: assertion.Name + " audience", Type: "public", Optin: "single", Status: "active", Tags: []string{tag}})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, okResp{map[string]int{"listmonk_list_id": list.ID}})
}

// WorkMateCustomerSSO creates a native, passwordless Listmonk user scoped to
// one WorkMate workspace and starts its normal Listmonk session. Every
// workspace gets a distinct list role; individual users get separate native
// accounts under that shared workspace role. A session can never widen itself
// by switching an active WorkMate workspace.
func (a *App) WorkMateCustomerSSO(c echo.Context) error {
	secret := strings.TrimSpace(os.Getenv("WORKMATE_CUSTOMER_SSO_SECRET"))
	if secret == "" {
		// The already deployed WorkMate assertion secret is the safe transition
		// key until the customer key is configured on both server processes.
		secret = strings.TrimSpace(os.Getenv("WORKMATE_ADMIN_SSO_SECRET"))
	}
	assertion, ok := readWorkMateAssertion(c.QueryParam("handoff"), secret, time.Now())
	if !ok || !validWorkMateCustomerAssertion(assertion) {
		return echo.NewHTTPError(http.StatusForbidden, "invalid WorkMate Reach handoff")
	}
	if err := a.ensureWorkMateAudiencePublic(assertion.ListID); err != nil {
		return err
	}

	userRole, err := a.workMateCustomerRole()
	if err != nil {
		return err
	}
	listRole, err := a.workMateWorkspaceListRole(assertion)
	if err != nil {
		return err
	}
	user, err := a.workMateWorkspaceUser(assertion, userRole.ID, listRole.ID)
	if err != nil {
		return err
	}
	if err := a.auth.SaveSession(user, "", c); err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	c.Response().Header().Set("Referrer-Policy", "no-referrer")
	return c.Redirect(http.StatusFound, uriAdmin)
}

func (a *App) ensureWorkMateAudiencePublic(listID int) error {
	list, err := a.core.GetList(listID, "")
	if err != nil {
		return err
	}
	// WorkMate seeds one usable audience per workspace. It must be public so
	// native Listmonk Forms can generate an embeddable subscribe form while
	// the workspace list role continues to enforce all admin-side isolation.
	if list.Type != "public" {
		list.Type = "public"
		_, err = a.core.UpdateList(list.ID, list)
	}
	return err
}

type workMateAssertion struct {
	Issuer    string `json:"iss"`
	Audience  string `json:"aud"`
	Kind      string `json:"kind"`
	Subject   string `json:"sub"`
	Tenant    string `json:"tenant"`
	Workspace string `json:"workspace"`
	ListID    int    `json:"list_id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	IssuedAt  int64  `json:"iat"`
	Expires   int64  `json:"exp"`
}

func readWorkMateAssertion(value, secret string, now time.Time) (workMateAssertion, bool) {
	if secret == "" {
		return workMateAssertion{}, false
	}
	parts := strings.Split(value, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return workMateAssertion{}, false
	}
	expected := hmac.New(sha256.New, []byte(secret))
	expected.Write([]byte(parts[0]))
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(signature, expected.Sum(nil)) {
		return workMateAssertion{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return workMateAssertion{}, false
	}
	var assertion workMateAssertion
	if err := json.Unmarshal(payload, &assertion); err != nil {
		return workMateAssertion{}, false
	}
	current := now.Unix()
	return assertion, assertion.Issuer == "workmate-os" && assertion.Audience == "workmate-listmonk" && assertion.IssuedAt <= current && assertion.Expires > current && assertion.Expires-assertion.IssuedAt <= 60
}

func validWorkMateCustomerAssertion(assertion workMateAssertion) bool {
	return assertion.Kind == "customer" && assertion.Subject != "" && assertion.Tenant != "" && assertion.Workspace != "" && assertion.ListID > 0 && assertion.Name != ""
}

func workMateScopedID(prefix, tenant, workspace, subject string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{tenant, workspace, subject}, "\x00")))
	return fmt.Sprintf("%s%x", prefix, sum[:16])
}

func workMateWorkspaceRoleID(tenant, workspace string) string {
	return workMateScopedID("wm-lr-", tenant, workspace, "")
}

func (a *App) workMateCustomerRole() (auth.Role, error) {
	permissions := pq.StringArray{
		auth.PermSubscribersGet, auth.PermSubscribersGetAll, auth.PermSubscribersManage, auth.PermSubscribersImport,
		auth.PermCampaignsGet, auth.PermCampaignsManage, auth.PermCampaignsGetAnalytics, auth.PermCampaignsSend,
		auth.PermTemplatesGet, auth.PermTemplatesManage,
		auth.PermBouncesGet,
		auth.PermMediaGet, auth.PermMediaManage,
	}
	roles, err := a.core.GetRoles()
	if err != nil {
		return auth.Role{}, err
	}
	for _, role := range roles {
		if role.Name.Valid && role.Name.String == workMateCustomerRoleName {
			return a.core.UpdateUserRole(role.ID, auth.Role{Name: null.StringFrom(workMateCustomerRoleName), Permissions: permissions})
		}
	}
	return a.core.CreateRole(auth.Role{Name: null.StringFrom(workMateCustomerRoleName), Permissions: permissions})
}

func isWorkMateCustomer(user auth.User) bool {
	return user.UserRole.Name == workMateCustomerRoleName
}

func (a *App) workMateWorkspaceListRole(assertion workMateAssertion) (auth.ListRole, error) {
	// A list role is a workspace boundary, not a person boundary. This lets
	// every member of the same WorkMate workspace work with the same lists and
	// delivery identity while keeping another workspace completely separate.
	name := workMateWorkspaceRoleID(assertion.Tenant, assertion.Workspace)
	role := auth.ListRole{Name: null.StringFrom(name), Lists: []auth.ListPermission{{ID: assertion.ListID, Permissions: pq.StringArray{auth.PermListGet, auth.PermListManage}}}}
	roles, err := a.core.GetListRoles()
	if err != nil {
		return auth.ListRole{}, err
	}
	for _, existing := range roles {
		if existing.Name.Valid && existing.Name.String == name {
			role.Lists = existing.Lists
			for _, list := range role.Lists {
				if list.ID == assertion.ListID {
					return a.core.UpdateListRole(existing.ID, role)
				}
			}
			role.Lists = append(role.Lists, auth.ListPermission{ID: assertion.ListID, Permissions: pq.StringArray{auth.PermListGet, auth.PermListManage}})
			return a.core.UpdateListRole(existing.ID, role)
		}
	}
	return a.core.CreateListRole(role)
}

func (a *App) workMateWorkspaceUser(assertion workMateAssertion, userRoleID, listRoleID int) (auth.User, error) {
	username := workMateScopedID("wm-", assertion.Tenant, assertion.Workspace, assertion.Subject)
	user, err := a.core.GetUser(0, username, "")
	if err == nil {
		user.UserRoleID = userRoleID
		user.ListRoleID = &listRoleID
		user.Name = assertion.Name
		user.Status = auth.UserStatusEnabled
		user.PasswordLogin = false
		return a.core.UpdateUser(user.ID, user)
	}
	if httpErr, ok := err.(*echo.HTTPError); !ok || httpErr.Code != http.StatusNotFound {
		return auth.User{}, err
	}
	emailID := workMateScopedID("", assertion.Tenant, assertion.Workspace, assertion.Subject)
	return a.core.CreateUser(auth.User{
		Username: username, PasswordLogin: false, Email: null.StringFrom(emailID + "@sessions.workmateos.co.uk"), Name: assertion.Name,
		Type: auth.UserTypeUser, UserRoleID: userRoleID, ListRoleID: &listRoleID, Status: auth.UserStatusEnabled,
	})
}
