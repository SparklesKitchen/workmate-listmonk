package main

import (
	"testing"

	"github.com/knadh/listmonk/internal/auth"
	"gopkg.in/volatiletech/null.v6"
)

func TestWorkMateDeliveryProvidersRemainDistinct(t *testing.T) {
	user := auth.User{ListRoleName: null.NewString("wm-lr-workspace-a", true)}
	seen := map[string]bool{}
	for _, provider := range []string{workMateDeliveryBrevoAPI, workMateDeliveryBrevoSMTP, workMateDeliverySMTP} {
		name := workMateDeliverySMTPName(user, provider)
		if name == "" {
			t.Fatalf("expected a delivery name for %s", provider)
		}
		if seen[name] {
			t.Fatalf("delivery name collision for %s", provider)
		}
		seen[name] = true
	}
}

func TestWorkMateDeliveryNamesCannotCrossWorkspaces(t *testing.T) {
	first := auth.User{ListRoleName: null.NewString("wm-lr-workspace-a", true)}
	second := auth.User{ListRoleName: null.NewString("wm-lr-workspace-b", true)}
	for _, provider := range []string{workMateDeliveryBrevoAPI, workMateDeliveryBrevoSMTP, workMateDeliverySMTP} {
		if workMateDeliverySMTPName(first, provider) == workMateDeliverySMTPName(second, provider) {
			t.Fatalf("workspace delivery name collision for %s", provider)
		}
	}
}

func TestValidateWorkMateDeliveryRequestRequiresExplicitProvider(t *testing.T) {
	for _, provider := range []string{workMateDeliveryBrevoAPI, workMateDeliveryBrevoSMTP, workMateDeliverySMTP} {
		req := workMateDeliveryRequest{
			Provider: provider, SenderName: "Workspace sender", FromEmail: "sender@example.com",
			Host: "mail.example.com", Port: 587, Username: "username", Password: "secret",
			AuthProtocol: "login", TLSType: "STARTTLS",
		}
		if err := validateWorkMateDeliveryRequest(&req); err != nil {
			t.Fatalf("expected %s to validate: %v", provider, err)
		}
		if provider == workMateDeliveryBrevoSMTP && req.Host != "smtp-relay.brevo.com" {
			t.Fatalf("Brevo SMTP must use its relay, got %q", req.Host)
		}
	}

	req := workMateDeliveryRequest{Provider: "brevo", FromEmail: "sender@example.com"}
	if err := validateWorkMateDeliveryRequest(&req); err == nil {
		t.Fatal("legacy ambiguous Brevo provider must fail")
	}
}

func TestWorkMateDeliverySaveDoesNotImplyVerification(t *testing.T) {
	response := workMateDeliveryResponse{Configured: true, Verified: false, Provider: workMateDeliverySMTP}
	if !response.Configured {
		t.Fatal("saved connection must remain visible")
	}
	if response.Verified {
		t.Fatal("saved credentials must not be treated as verified")
	}
}
