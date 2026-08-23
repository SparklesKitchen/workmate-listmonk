package main

import (
	"net/http/httptest"
	"testing"

	"github.com/knadh/listmonk/internal/auth"
	"github.com/labstack/echo/v4"
)

func TestTemplateScopeKeepsWorkMateCustomerResourcesInWorkspace(t *testing.T) {
	e := echo.New()
	c := e.NewContext(httptest.NewRequest("GET", "/", nil), httptest.NewRecorder())
	c.Set(auth.UserHTTPCtxKey, auth.User{UserRole: struct {
		ID          int      `db:"-" json:"id"`
		Name        string   `db:"-" json:"name"`
		Permissions []string `db:"-" json:"permissions"`
	}{Name: "WorkMate Customer"}, GetListIDs: []int{9, 4}})

	if got := templateScope(c); got != 4 {
		t.Fatalf("template scope = %d, want stable workspace owner list 4", got)
	}
}

func TestTemplateScopeRejectsAmbiguousNonWorkMateListRoles(t *testing.T) {
	e := echo.New()
	c := e.NewContext(httptest.NewRequest("GET", "/", nil), httptest.NewRecorder())
	c.Set(auth.UserHTTPCtxKey, auth.User{GetListIDs: []int{4, 9}})

	if got := templateScope(c); got != -1 {
		t.Fatalf("template scope = %d, want denied scope", got)
	}
}
