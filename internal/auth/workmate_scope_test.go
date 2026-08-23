package auth

import (
	"testing"

	"github.com/labstack/echo/v4"
)

func TestWorkMateScopeFromContextOnlyUsesServerLoadedScope(t *testing.T) {
	c := echo.New().NewContext(nil, nil)
	if _, ok := WorkMateScopeFromContext(c); ok {
		t.Fatal("missing scope accepted")
	}
	c.Set(WorkMateScopeHTTPCtxKey, WorkMateScope{Tenant: "tenant-a", Workspace: "workspace-a"})
	scope, ok := WorkMateScopeFromContext(c)
	if !ok || scope.Tenant != "tenant-a" || scope.Workspace != "workspace-a" {
		t.Fatalf("loaded scope = %#v, %t", scope, ok)
	}
}

func TestListPermissionsRejectAnotherWorkspaceList(t *testing.T) {
	user := User{ListPermissionsMap: map[int]map[string]struct{}{
		11: {PermListGet: {}, PermListManage: {}},
	}}
	if err := user.HasListPerm(PermTypeGet|PermTypeManage, 11); err != nil {
		t.Fatalf("own list denied: %v", err)
	}
	if err := user.HasListPerm(PermTypeGet|PermTypeManage, 12); err == nil {
		t.Fatal("another workspace list permitted")
	}
}
