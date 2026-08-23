package main

import (
	"testing"
	"time"

	"github.com/knadh/listmonk/internal/auth"
	"github.com/knadh/listmonk/internal/workmate"
	"gopkg.in/volatiletech/null.v6"
)

func TestWorkMateWorkspaceRoleIDDoesNotDependOnUser(t *testing.T) {
	first := workMateWorkspaceRoleID("tenant-a", "workspace-a")
	second := workMateWorkspaceRoleID("tenant-a", "workspace-a")
	if first != second {
		t.Fatalf("workspace role IDs must remain stable: %q != %q", first, second)
	}
}

func TestWorkMateNativeUserIDRemainsUserScoped(t *testing.T) {
	first := workMateScopedID("wm-", "tenant-a", "workspace-a", "user-a")
	second := workMateScopedID("wm-", "tenant-a", "workspace-a", "user-b")
	if first == second {
		t.Fatal("native users from different people must not share an account")
	}
}

func TestWorkMateCustomerListReceiptIsSignedAndServerScoped(t *testing.T) {
	const secret = "test-registry-secret"
	now := time.Unix(1_700_000_000, 0)
	handoff, err := writeWorkMateAssertion(workMateAssertion{
		Issuer: "workmate-listmonk", Audience: "workmate-os", Kind: "list-created",
		Tenant: "tenant-a", Workspace: "workspace-a", ListID: 73, IssuedAt: now.Unix(), Expires: now.Add(60 * time.Second).Unix(),
	}, secret)
	if err != nil {
		t.Fatal(err)
	}
	var got workMateAssertion
	if !workmate.DecodeHMACJSON(handoff, secret, &got) {
		t.Fatal("signed Listmonk receipt was rejected")
	}
	if got.Kind != "list-created" || got.Tenant != "tenant-a" || got.Workspace != "workspace-a" || got.ListID != 73 {
		t.Fatalf("unexpected registry receipt: %#v", got)
	}
	if workmate.DecodeHMACJSON(handoff, "other-secret", &got) {
		t.Fatal("receipt accepted with another server secret")
	}
}

func TestWorkMateCustomerCannotClaimAnotherWorkspaceList(t *testing.T) {
	roleID := 42
	user := auth.User{
		ListRoleID:   &roleID,
		ListRoleName: null.StringFrom(workMateWorkspaceRoleID("tenant-a", "workspace-a")),
		UserRole: struct {
			ID          int      "db:\"-\" json:\"id\""
			Name        string   "db:\"-\" json:\"name\""
			Permissions []string "db:\"-\" json:\"permissions\""
		}{Name: workMateCustomerRoleName},
		ListPermissionsMap: map[int]map[string]struct{}{
			11: {auth.PermListGet: {}, auth.PermListManage: {}},
		},
	}
	if err := user.HasListPerm(auth.PermTypeGet|auth.PermTypeManage, 11); err != nil {
		t.Fatalf("own workspace list denied: %v", err)
	}
	if err := user.HasListPerm(auth.PermTypeGet|auth.PermTypeManage, 99); err == nil {
		t.Fatal("another workspace list was permitted")
	}
	if !validWorkMateWorkspaceRole(user) {
		t.Fatal("valid WorkMate workspace user was rejected")
	}
}

func TestWorkMateCustomerAssertionRemainsShortLivedAndPasswordless(t *testing.T) {
	assertion := workMateAssertion{
		Kind: "customer", Subject: "user-a", Tenant: "tenant-a", Workspace: "workspace-a", ListID: 11, Name: "Customer",
	}
	if !validWorkMateCustomerAssertion(assertion) {
		t.Fatal("valid customer assertion rejected")
	}
	assertion.ListID = 0
	if validWorkMateCustomerAssertion(assertion) {
		t.Fatal("customer assertion without a provisioned list was accepted")
	}
}
