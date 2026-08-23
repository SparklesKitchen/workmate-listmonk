package core

import (
	"net/http"

	"github.com/jmoiron/sqlx/types"
	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
)

// GetDashboardCharts returns chart data points to render on the dashboard.
func (c *Core) GetDashboardCharts() (types.JSONText, error) {
	_ = c.refreshCache(matDashboardCharts, false)

	var out types.JSONText
	if err := c.q.GetDashboardCharts.Get(&out); err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorFetching", "name", "dashboard charts", "error", pqErrMsg(err)))
	}

	return out, nil
}

// GetDashboardCounts returns stats counts to show on the dashboard.
func (c *Core) GetDashboardCounts() (types.JSONText, error) {
	_ = c.refreshCache(matDashboardCounts, false)

	var out types.JSONText
	if err := c.q.GetDashboardCounts.Get(&out); err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorFetching", "name", "dashboard stats", "error", pqErrMsg(err)))
	}

	return out, nil
}

// GetDashboardChartsForLists returns chart aggregates only for campaigns that
// belong to one of listIDs. It is deliberately uncached: the stock dashboard
// materialized view is global and must never be returned to a WorkMate
// workspace-scoped customer session.
func (c *Core) GetDashboardChartsForLists(listIDs []int) (types.JSONText, error) {
	const query = `
		WITH scoped_campaigns AS (
			SELECT DISTINCT campaigns.id
			FROM campaigns
			JOIN campaign_lists ON campaign_lists.campaign_id = campaigns.id
			WHERE campaign_lists.list_id = ANY($1)
		),
		clicks AS (
			SELECT COALESCE(JSON_AGG(ROW_TO_JSON(row)), '[]'::json) AS data
			FROM (
				SELECT COUNT(*) AS count, link_clicks.created_at::DATE AS date
				FROM link_clicks
				JOIN scoped_campaigns ON scoped_campaigns.id = link_clicks.campaign_id
				WHERE link_clicks.created_at >= CURRENT_DATE - INTERVAL '30 days'
				GROUP BY link_clicks.created_at::DATE
				ORDER BY date
			) row
		),
		views AS (
			SELECT COALESCE(JSON_AGG(ROW_TO_JSON(row)), '[]'::json) AS data
			FROM (
				SELECT COUNT(*) AS count, campaign_views.created_at::DATE AS date
				FROM campaign_views
				JOIN scoped_campaigns ON scoped_campaigns.id = campaign_views.campaign_id
				WHERE campaign_views.created_at >= CURRENT_DATE - INTERVAL '30 days'
				GROUP BY campaign_views.created_at::DATE
				ORDER BY date
			) row
		)
		SELECT JSON_BUILD_OBJECT('link_clicks', (SELECT data FROM clicks), 'campaign_views', (SELECT data FROM views));`

	var out types.JSONText
	if err := c.db.QueryRowx(query, pq.Array(listIDs)).Scan(&out); err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorFetching", "name", "workspace dashboard charts", "error", pqErrMsg(err)))
	}
	return out, nil
}

// GetDashboardCountsForLists returns dashboard counts only for one workspace's
// permitted lists. A subscriber or campaign is included only when it is linked
// to at least one of those lists, preventing shared Listmonk global totals
// from leaking between WorkMate workspaces.
func (c *Core) GetDashboardCountsForLists(listIDs []int) (types.JSONText, error) {
	const query = `
		WITH scoped_lists AS (
			SELECT id, type, optin FROM lists WHERE id = ANY($1)
		),
		scoped_subscribers AS (
			SELECT DISTINCT subscribers.id, subscribers.status
			FROM subscribers
			JOIN subscriber_lists ON subscriber_lists.subscriber_id = subscribers.id
			JOIN scoped_lists ON scoped_lists.id = subscriber_lists.list_id
		),
		scoped_campaigns AS (
			SELECT DISTINCT campaigns.id, campaigns.status, campaigns.sent
			FROM campaigns
			JOIN campaign_lists ON campaign_lists.campaign_id = campaigns.id
			JOIN scoped_lists ON scoped_lists.id = campaign_lists.list_id
		)
		SELECT JSON_BUILD_OBJECT(
			'subscribers', JSON_BUILD_OBJECT(
				'total', (SELECT COUNT(*) FROM scoped_subscribers),
				'blocklisted', (SELECT COUNT(*) FROM scoped_subscribers WHERE status = 'blocklisted'),
				'orphans', 0
			),
			'lists', JSON_BUILD_OBJECT(
				'total', (SELECT COUNT(*) FROM scoped_lists),
				'private', (SELECT COUNT(*) FROM scoped_lists WHERE type = 'private'),
				'public', (SELECT COUNT(*) FROM scoped_lists WHERE type = 'public'),
				'optin_single', (SELECT COUNT(*) FROM scoped_lists WHERE optin = 'single'),
				'optin_double', (SELECT COUNT(*) FROM scoped_lists WHERE optin = 'double')
			),
			'campaigns', JSON_BUILD_OBJECT(
				'total', (SELECT COUNT(*) FROM scoped_campaigns),
				'by_status', COALESCE((
					SELECT JSON_OBJECT_AGG(status, count)
					FROM (SELECT status, COUNT(*) AS count FROM scoped_campaigns GROUP BY status) grouped_campaigns
				), '{}'::json)
			),
			'messages', (SELECT COALESCE(SUM(sent), 0) FROM scoped_campaigns)
		);`

	var out types.JSONText
	if err := c.db.QueryRowx(query, pq.Array(listIDs)).Scan(&out); err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError,
			c.i18n.Ts("globals.messages.errorFetching", "name", "workspace dashboard stats", "error", pqErrMsg(err)))
	}
	return out, nil
}
