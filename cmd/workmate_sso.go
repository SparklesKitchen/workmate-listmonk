package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

// WorkMateAdminSSO accepts a short-lived assertion issued only by the
// authenticated WorkMate SaaS Admin service. It deliberately logs in the
// existing Listmonk super-admin account; no Listmonk credential is exposed to
// the browser or stored by WorkMate OS.
func (a *App) WorkMateAdminSSO(c echo.Context) error {
	secret := strings.TrimSpace(os.Getenv("WORKMATE_ADMIN_SSO_SECRET"))
	if secret == "" || !validWorkMateAssertion(c.QueryParam("handoff"), secret, time.Now()) {
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

type workMateAssertion struct {
	Issuer   string `json:"iss"`
	Audience string `json:"aud"`
	Kind     string `json:"kind"`
	IssuedAt int64  `json:"iat"`
	Expires  int64  `json:"exp"`
}

func validWorkMateAssertion(value, secret string, now time.Time) bool {
	parts := strings.Split(value, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false
	}
	expected := hmac.New(sha256.New, []byte(secret))
	expected.Write([]byte(parts[0]))
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(signature, expected.Sum(nil)) {
		return false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	var assertion workMateAssertion
	if err := json.Unmarshal(payload, &assertion); err != nil {
		return false
	}
	current := now.Unix()
	return assertion.Issuer == "workmate-os" && assertion.Audience == "workmate-listmonk" && assertion.Kind == "saas-admin" && assertion.IssuedAt <= current && assertion.Expires > current && assertion.Expires-assertion.IssuedAt <= 60
}
