package main

import "testing"

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
