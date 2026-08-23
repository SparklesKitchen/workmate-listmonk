package main

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/knadh/listmonk/internal/auth"
	"github.com/knadh/listmonk/models"
	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
)

// GetLists retrieves lists with additional metadata like subscriber counts.
func (a *App) GetLists(c echo.Context) error {
	// Get the authenticated user.
	user := auth.GetUser(c)

	// Get the list IDs (or blanket permission) the user has access to.
	hasAllPerm, permittedIDs := user.GetPermittedLists(auth.PermTypeGet)

	// Minimal query simply returns the list of all lists without JOIN subscriber counts. This is fast.
	minimal, _ := strconv.ParseBool(c.FormValue("minimal"))
	if minimal {
		status := c.FormValue("status")
		res, err := a.core.GetLists("", status, hasAllPerm, permittedIDs)
		if err != nil {
			return err
		}
		if len(res) == 0 {
			return c.JSON(http.StatusOK, okResp{[]struct{}{}})
		}

		// Meta.
		total := len(res)
		out := models.PageResults{
			Results: res,
			Total:   total,
			Page:    1,
			PerPage: total,
		}

		return c.JSON(http.StatusOK, okResp{out})
	}

	// Full list query.
	var (
		query   = strings.TrimSpace(c.FormValue("query"))
		tags    = c.QueryParams()["tag"]
		orderBy = c.FormValue("order_by")
		typ     = c.FormValue("type")
		optin   = c.FormValue("optin")
		status  = c.FormValue("status")
		order   = c.FormValue("order")

		pg = a.pg.NewFromURL(c.Request().URL.Query())
	)
	res, total, err := a.core.QueryLists(query, typ, optin, status, tags, orderBy, order, hasAllPerm, permittedIDs, pg.Offset, pg.Limit)
	if err != nil {
		return err
	}

	out := models.PageResults{
		Query:   query,
		Results: res,
		Total:   total,
		Page:    pg.Page,
		PerPage: pg.PerPage,
	}

	return c.JSON(http.StatusOK, okResp{out})
}

// GetList retrieves a single list by id.
// It's permission checked by the listPerm middleware.
func (a *App) GetList(c echo.Context) error {
	// Get the authenticated user.
	user := auth.GetUser(c)

	// Check if the user has access to the list.
	id := getID(c)
	if err := user.HasListPerm(auth.PermTypeGet, id); err != nil {
		return err
	}

	// Get the list from the DB.
	out, err := a.core.GetList(id, "")
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, okResp{out})
}

// CreateList handles list creation.
func (a *App) CreateList(c echo.Context) error {
	user := auth.GetUser(c)
	if !user.HasPerm(auth.PermListManageAll) {
		if !validWorkMateWorkspaceRole(user) || len(user.ManageListIDs) == 0 {
			return auth.ErrPermDenied
		}
	}
	l := models.List{}
	if err := c.Bind(&l); err != nil {
		return err
	}

	// Validate.
	if !strHasLen(l.Name, 1, stdInputMaxLen) {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.T("lists.invalidName"))
	}

	out, err := a.core.CreateList(l)
	if err != nil {
		return err
	}
	if validWorkMateWorkspaceRole(user) {
		if err := a.addWorkMateListRolePermission(user, out.ID); err != nil {
			_ = a.core.DeleteLists([]int{out.ID}, "", true, nil)
			return err
		}
		// The native role is deliberately granted first, then the signed receipt
		// proves to WorkMate OS that this list belongs to this authenticated
		// workspace. If the receipt cannot be recorded, revoke and remove the
		// just-created list so it never survives outside the durable registry.
		if err := a.persistWorkMateListOwnership(c, user, out); err != nil {
			_ = a.removeWorkMateListRolePermission(user, out.ID)
			_ = a.core.DeleteLists([]int{out.ID}, "", true, nil)
			return err
		}
	}

	return c.JSON(http.StatusOK, okResp{out})
}

func (a *App) removeWorkMateListRolePermission(user auth.User, listID int) error {
	if !validWorkMateWorkspaceRole(user) {
		return auth.ErrPermDenied
	}
	roles, err := a.core.GetListRoles()
	if err != nil {
		return err
	}
	for _, role := range roles {
		if role.ID != *user.ListRoleID {
			continue
		}
		if !role.Name.Valid || role.Name.String != user.ListRoleName.String {
			return auth.ErrPermDenied
		}
		permissions := role.Lists[:0]
		for _, permission := range role.Lists {
			if permission.ID != listID {
				permissions = append(permissions, permission)
			}
		}
		role.Lists = permissions
		_, err := a.core.UpdateListRole(role.ID, role)
		return err
	}
	return auth.ErrPermDenied
}

// addWorkMateListRolePermission adds a native customer-created list to exactly
// the authenticated user's existing workspace list role. It does not accept a
// role ID from the request, which prevents a customer from assigning a list to
// another workspace.
func (a *App) addWorkMateListRolePermission(user auth.User, listID int) error {
	if !validWorkMateWorkspaceRole(user) {
		return auth.ErrPermDenied
	}
	roles, err := a.core.GetListRoles()
	if err != nil {
		return err
	}
	for _, role := range roles {
		if role.ID != *user.ListRoleID {
			continue
		}
		if !role.Name.Valid || role.Name.String != user.ListRoleName.String {
			return auth.ErrPermDenied
		}
		for _, permission := range role.Lists {
			if permission.ID == listID {
				return nil
			}
		}
		role.Lists = append(role.Lists, auth.ListPermission{ID: listID, Permissions: pq.StringArray{auth.PermListGet, auth.PermListManage}})
		_, err := a.core.UpdateListRole(role.ID, role)
		return err
	}
	return auth.ErrPermDenied
}

// UpdateList handles list modification.
// It's permission checked by the listPerm middleware.
func (a *App) UpdateList(c echo.Context) error {
	// Get the authenticated user.
	user := auth.GetUser(c)

	// Check if the user has access to the list.
	id := getID(c)
	if err := user.HasListPerm(auth.PermTypeManage, id); err != nil {
		return err
	}

	// Incoming params.
	var l models.List
	if err := c.Bind(&l); err != nil {
		return err
	}

	// Validate.
	if !strHasLen(l.Name, 1, stdInputMaxLen) {
		return echo.NewHTTPError(http.StatusBadRequest, a.i18n.T("lists.invalidName"))
	}

	// Update the list in the DB.
	out, err := a.core.UpdateList(id, l)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, okResp{out})
}

// DeleteList deletes a single list by ID.
func (a *App) DeleteList(c echo.Context) error {
	id := getID(c)

	// Check if the user has manage permission for the list.
	user := auth.GetUser(c)
	if err := user.HasListPerm(auth.PermTypeManage, id); err != nil {
		return err
	}

	// Delete the list from the DB.
	// Pass getAll=true since we've already verified permissions above.
	if err := a.core.DeleteLists([]int{id}, "", true, nil); err != nil {
		return err
	}

	return c.JSON(http.StatusOK, okResp{true})
}

// DeleteLists deletes multiple lists by IDs or by query.
func (a *App) DeleteLists(c echo.Context) error {
	user := auth.GetUser(c)

	var (
		ids   []int
		query string
		all   bool
	)

	// Check for IDs in query params.
	if len(c.Request().URL.Query()["id"]) > 0 {
		var err error
		ids, err = parseStringIDs(c.Request().URL.Query()["id"])
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest,
				a.i18n.Ts("globals.messages.errorInvalidIDs", "error", err.Error()))
		}
	} else {
		// Check for query param.
		query = strings.TrimSpace(c.FormValue("query"))
		all = c.FormValue("all") == "true"
	}

	// Validate that either IDs or query is provided.
	if len(ids) == 0 && (query == "" && !all) {
		return echo.NewHTTPError(http.StatusBadRequest,
			a.i18n.Ts("globals.messages.errorInvalidIDs", "error", "id or query required"))
	}

	// For ID deletion, check if the user has manage permission for the specific lists.
	if len(ids) > 0 {
		if err := user.HasListPerm(auth.PermTypeManage, ids...); err != nil {
			return err
		}

		// Delete the lists from the DB.
		// Pass getAll=true since we've already verified permissions above.
		if err := a.core.DeleteLists(ids, "", true, nil); err != nil {
			return err
		}
	} else {
		// For query deletion, get the list IDs the user has manage permission for.
		hasAllPerm, permittedIDs := user.GetPermittedLists(auth.PermTypeManage)

		// Delete the lists from the DB with permission filtering.
		if err := a.core.DeleteLists(nil, query, hasAllPerm, permittedIDs); err != nil {
			return err
		}
	}

	return c.JSON(http.StatusOK, okResp{true})
}
