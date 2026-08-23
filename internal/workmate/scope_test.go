package workmate

import "testing"

func TestWorkspaceRoleRejectsBrowserShapedNames(t *testing.T) {
	if !IsWorkspaceRole("wm-lr-7c727e9d7c727e9d7c727e9d7c727e9d") {
		t.Fatal("valid opaque workspace role rejected")
	}
	for _, name := range []string{"", "wm-lr-", "workspace-a", "wm-lr-../workspace-b"} {
		if IsWorkspaceRole(name) {
			t.Fatalf("untrusted role name accepted: %q", name)
		}
	}
}

func TestHMACJSONRoundTripRejectsTampering(t *testing.T) {
	input := struct {
		Tenant string `json:"tenant"`
		ListID int    `json:"list_id"`
	}{Tenant: "tenant-a", ListID: 7}
	token, err := EncodeHMACJSON(input, "server-secret")
	if err != nil {
		t.Fatal(err)
	}
	var output struct {
		Tenant string `json:"tenant"`
		ListID int    `json:"list_id"`
	}
	if !DecodeHMACJSON(token, "server-secret", &output) || output != input {
		t.Fatalf("HMAC handoff did not round-trip: %#v", output)
	}
	if DecodeHMACJSON(token, "other-secret", &output) {
		t.Fatal("handoff accepted with another secret")
	}
	if DecodeHMACJSON(token+"x", "server-secret", &output) {
		t.Fatal("tampered handoff accepted")
	}
}

func TestRegistryPayloadSignatureBindsBodyAndSecret(t *testing.T) {
	payload := []byte(`{"workspace_role_id":"wm-lr-a","listmonk_list_id":7}`)
	signature := SignRegistryPayload(payload, "server-secret")
	if signature != SignRegistryPayload(payload, "server-secret") {
		t.Fatal("same registry event was not signed deterministically")
	}
	if signature == SignRegistryPayload([]byte(`{"workspace_role_id":"wm-lr-b","listmonk_list_id":7}`), "server-secret") {
		t.Fatal("registry signature did not bind the workspace role")
	}
	if signature == SignRegistryPayload(payload, "other-secret") {
		t.Fatal("registry signature did not bind the server secret")
	}
}
