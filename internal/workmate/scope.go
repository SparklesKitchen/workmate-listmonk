// Package workmate contains the small, non-product-specific primitives used by
// the maintained Listmonk fork to enforce WorkMate workspace boundaries.
package workmate

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
)

const WorkspaceRolePrefix = "wm-lr-"

// IsWorkspaceRole recognizes only the opaque workspace-role identifiers minted
// from a verified WorkMate assertion. It intentionally rejects arbitrary list
// role names supplied by a browser.
func IsWorkspaceRole(name string) bool {
	const hashLength = 32 // first 16 bytes of a SHA-256 digest, hex encoded.
	if len(name) != len(WorkspaceRolePrefix)+hashLength || name[:len(WorkspaceRolePrefix)] != WorkspaceRolePrefix {
		return false
	}
	_, err := hex.DecodeString(name[len(WorkspaceRolePrefix):])
	return err == nil
}

// SignRegistryPayload makes the native-to-WorkMate ownership callback
// tamper-evident. WorkMate verifies the same HMAC before accepting a mapping.
func SignRegistryPayload(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// EncodeHMACJSON creates the compact short-lived handoff format shared by
// WorkMate OS and Listmonk: base64url(JSON) + "." + base64url(HMAC-SHA256).
func EncodeHMACJSON(value any, secret string) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(encoded))
	return encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

// DecodeHMACJSON verifies the exact compact handoff before unmarshalling it.
func DecodeHMACJSON(token, secret string, target any) bool {
	if secret == "" {
		return false
	}
	parts := splitToken(token)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(parts[0]))
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(signature, mac.Sum(nil)) {
		return false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	return err == nil && json.Unmarshal(payload, target) == nil
}

func splitToken(token string) []string {
	for i := range token {
		if token[i] == '.' {
			return []string{token[:i], token[i+1:]}
		}
	}
	return nil
}
