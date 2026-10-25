package api

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// statusFilter narrows the status logs (what Orion itself saw: checks, incidents, alerts, servers).
type statusFilter struct {
	Levels     []string
	Types      []string
	ServerID   string
	MonitorID  string
	IncidentID string
	Text       string
	From, To   time.Time
}

// statusRow is one event before it gets a title and detail.
type statusRow struct {
	ID          string
	AtMs        int64
	Type        string
	Level       string
	Kind        string
	ServerID    string
	ServerName  string
	MonitorID   string
	MonitorName string
	IncidentID  string
	IncNum      int
	A, B, C, D  string
	N1, N2      int
	N3, N4      int
}

const statusColumns = "id, at_ms, type, level, kind, server_id, server_name, monitor_id, monitor_name, incident_id, inc_num, a, b, c, d, n1, n2, n3, n4"

// msExpr is the epoch milliseconds of a stored timestamp column.
func msExpr(col string) string {
	return fmt.Sprintf("(CAST(strftime('%%s', %[1]s) AS INTEGER) * 1000 + CAST(ROUND((CAST(strftime('%%f', %[1]s) AS REAL) - CAST(strftime('%%S', %[1]s) AS INTEGER)) * 1000) AS INTEGER))", col)
}

func (f statusFilter) allowsType(t string) bool {
	if len(f.Types) == 0 {
		return true
	}
	for _, x := range f.Types {
		if x == t {
			return true
		}
	}
	return false
}

// union builds the UNION ALL of every event source, pushing the time range and the id filters into each branch.
func (f statusFilter) union() (string, []any) {
	var branches []string
	var args []any
	add := func(sql string, a ...any) {
		branches = append(branches, sql)
		args = append(args, a...)
	}
	timeRange := func(col string) (string, []any) {
		var clauses []string
		var a []any
		if !f.From.IsZero() {
			clauses = append(clauses, col+" >= ?")
			a = append(a, f.From.UTC())
		}
		if !f.To.IsZero() {
			clauses = append(clauses, col+" <= ?")
			a = append(a, f.To.UTC())
		}
		if len(clauses) == 0 {
			return "1=1", nil
		}
		return strings.Join(clauses, " AND "), a
	}
	monitorScope := func(extra string) (string, []any) {
		clause, a := "1=1", []any(nil)
		if f.ServerID != "" {
			clause += " AND m.node_id = ?"
			a = append(a, f.ServerID)
		}
		if f.MonitorID != "" {
			clause += " AND m.id = ?"
			a = append(a, f.MonitorID)
		}
		return clause + extra, a
	}
	incidentScope := func() (string, []any) {
		clause, a := "1=1", []any(nil)
		if f.ServerID != "" {
			clause += " AND i.node_id = ?"
			a = append(a, f.ServerID)
		}
		if f.MonitorID != "" {
			clause += " AND i.monitor_id = ?"
			a = append(a, f.MonitorID)
		}
		if f.IncidentID != "" {
			clause += " AND i.id = ?"
			a = append(a, f.IncidentID)
		}
		return clause, a
	}
	// A branch that has no meaning under an incident filter is skipped.
	noIncident := f.IncidentID == ""

	if f.allowsType("monitor") && noIncident {
		tr, ta := timeRange("r.created_at")
		sc, sa := monitorScope("")
		add(`SELECT 'r:' || r.id AS id, `+msExpr("r.created_at")+` AS at_ms, 'monitor' AS type, CASE r.health WHEN 'up' THEN 'info' WHEN 'degraded' THEN 'warn' ELSE 'error' END AS level, 'check' AS kind,
			COALESCE(m.node_id,'') AS server_id, COALESCE(a.name,'') AS server_name, m.id AS monitor_id, m.name AS monitor_name, '' AS incident_id, 0 AS inc_num,
			r.health AS a, r.payload AS b, '' AS c, '' AS d, 0 AS n1, 0 AS n2, 0 AS n3, 0 AS n4
			FROM monitor_reports r JOIN monitors m ON m.id = r.monitor_id LEFT JOIN nodes a ON a.id = m.node_id WHERE `+tr+` AND `+sc, append(ta, sa...)...)

		// State changes: a report whose health differs from the previous report of the same monitor.
		lookback := time.Time{}
		if !f.From.IsZero() {
			lookback = f.From.Add(-6 * time.Hour)
		}
		lbClause, lbArgs := "1=1", []any(nil)
		if !lookback.IsZero() {
			lbClause, lbArgs = "r.created_at >= ?", []any{lookback.UTC()}
		}
		toClause, toArgs := "1=1", []any(nil)
		if !f.To.IsZero() {
			toClause, toArgs = "r.created_at <= ?", []any{f.To.UTC()}
		}
		fromClause, fromArgs := "1=1", []any(nil)
		if !f.From.IsZero() {
			fromClause, fromArgs = "t.created_at >= ?", []any{f.From.UTC()}
		}
		scTr, saTr := monitorScope("")
		add(`SELECT 't:' || t.id AS id, `+msExpr("t.created_at")+` AS at_ms, 'monitor' AS type, CASE t.health WHEN 'up' THEN 'info' WHEN 'degraded' THEN 'warn' ELSE 'error' END AS level, 'transition' AS kind,
			COALESCE(m.node_id,'') AS server_id, COALESCE(a.name,'') AS server_name, m.id AS monitor_id, m.name AS monitor_name, '' AS incident_id, 0 AS inc_num,
			t.health AS a, t.payload AS b, t.prev AS c, '' AS d, 0 AS n1, 0 AS n2, 0 AS n3, 0 AS n4
			FROM (SELECT r.id, r.monitor_id, r.created_at, r.health, r.payload, LAG(r.health) OVER (PARTITION BY r.monitor_id ORDER BY r.created_at, r.id) AS prev
				FROM monitor_reports r WHERE `+lbClause+` AND `+toClause+`) t
			JOIN monitors m ON m.id = t.monitor_id LEFT JOIN nodes a ON a.id = m.node_id
			WHERE t.prev IS NOT NULL AND t.prev <> t.health AND `+fromClause+` AND `+scTr, concatArgs(lbArgs, toArgs, fromArgs, saTr)...)
	}
	if f.allowsType("server") && noIncident && f.MonitorID == "" {
		tr, ta := timeRange("ar.created_at")
		scope, sa := "1=1", []any(nil)
		if f.ServerID != "" {
			scope, sa = "ar.node_id = ?", []any{f.ServerID}
		}
		add(`SELECT 'a:' || ar.id AS id, `+msExpr("ar.created_at")+` AS at_ms, 'server' AS type, 'info' AS level, 'report' AS kind,
			ar.node_id AS server_id, COALESCE(a.name,'') AS server_name, '' AS monitor_id, '' AS monitor_name, '' AS incident_id, 0 AS inc_num,
			COALESCE(CAST(json_extract(ar.cpu, '$.usage_percent') AS TEXT),'') AS a, COALESCE(CAST(json_extract(ar.memory, '$.used_percent') AS TEXT),'') AS b, COALESCE(CAST(json_extract(ar.disk, '$.used_percent') AS TEXT),'') AS c, ar.node_version AS d, 0 AS n1, 0 AS n2, 0 AS n3, 0 AS n4
			FROM node_reports ar LEFT JOIN nodes a ON a.id = ar.node_id WHERE `+tr+` AND `+scope, append(ta, sa...)...)
		tr, ta = timeRange("g.created_at")
		scope, sa = "g.system = 0", nil
		if f.ServerID != "" {
			scope, sa = "g.system = 0 AND g.id = ?", []any{f.ServerID}
		}
		add(`SELECT 'g:' || g.id AS id, `+msExpr("g.created_at")+` AS at_ms, 'server' AS type, 'info' AS level, 'registered' AS kind,
			g.id AS server_id, g.name AS server_name, '' AS monitor_id, '' AS monitor_name, '' AS incident_id, 0 AS inc_num, g.name AS a, '' AS b, '' AS c, '' AS d, 0 AS n1, 0 AS n2, 0 AS n3, 0 AS n4
			FROM nodes g WHERE `+tr+` AND `+scope, append(ta, sa...)...)
	}
	if f.allowsType("monitor") && noIncident {
		tr, ta := timeRange("m.created_at")
		sc, sa := monitorScope(" AND m.system = 0")
		add(`SELECT 'm:' || m.id AS id, `+msExpr("m.created_at")+` AS at_ms, 'monitor' AS type, 'info' AS level, 'created' AS kind,
			COALESCE(m.node_id,'') AS server_id, COALESCE(a.name,'') AS server_name, m.id AS monitor_id, m.name AS monitor_name, '' AS incident_id, 0 AS inc_num, m.name AS a, '' AS b, '' AS c, '' AS d, 0 AS n1, 0 AS n2, 0 AS n3, 0 AS n4
			FROM monitors m LEFT JOIN nodes a ON a.id = m.node_id WHERE `+tr+` AND `+sc, append(ta, sa...)...)
	}
	if f.allowsType("incident") {
		tr, ta := timeRange("e.created_at")
		sc, sa := incidentScope()
		add(`SELECT 'i:' || e.id AS id, `+msExpr("e.created_at")+` AS at_ms, 'incident' AS type,
			CASE e.type WHEN 'incident_opened' THEN 'error' WHEN 'monitor_failed' THEN 'error' WHEN 'incident_reopened' THEN 'error' WHEN 'incident_declared' THEN 'error' WHEN 'incident_covered' THEN 'warn' ELSE 'info' END AS level, e.type AS kind,
			i.node_id AS server_id, COALESCE(a.name,'') AS server_name, i.monitor_id AS monitor_id, COALESCE(m.name,'') AS monitor_name, i.id AS incident_id, i.number AS inc_num,
			e.message AS a, e.note AS b, e.actor_id AS c, i.title AS d, 0 AS n1, 0 AS n2, 0 AS n3, 0 AS n4
			FROM incident_events e JOIN incidents i ON i.id = e.incident_id LEFT JOIN monitors m ON m.id = i.monitor_id LEFT JOIN nodes a ON a.id = i.node_id WHERE `+tr+` AND `+sc, append(ta, sa...)...)
	}
	if f.allowsType("alert") {
		tr, ta := timeRange("d.created_at")
		sc, sa := incidentScope()
		add(`SELECT 'd:' || d.id AS id, `+msExpr("d.created_at")+` AS at_ms, 'alert' AS type,
			CASE d.status WHEN 'failed' THEN 'error' WHEN 'suppressed' THEN 'warn' WHEN 'cooldown' THEN 'warn' ELSE 'info' END AS level, 'delivery' AS kind,
			COALESCE(i.node_id,'') AS server_id, COALESCE(a.name,'') AS server_name, COALESCE(i.monitor_id,'') AS monitor_id, COALESCE(m.name,'') AS monitor_name, COALESCE(i.id,'') AS incident_id, COALESCE(i.number,0) AS inc_num,
			d.channel AS a, d.type AS b, d.status AS c, d.error AS d, d.attempt_count AS n1, d.max_attempts AS n2,
			COALESCE((SELECT x.http_status FROM alert_delivery_attempts x WHERE x.alert_delivery_id = d.id ORDER BY x.attempt_number DESC LIMIT 1),0) AS n3,
			COALESCE((SELECT CAST((julianday(x.completed_at) - julianday(x.started_at)) * 86400000 AS INTEGER) FROM alert_delivery_attempts x WHERE x.alert_delivery_id = d.id AND x.completed_at IS NOT NULL ORDER BY x.attempt_number DESC LIMIT 1),0) AS n4
			FROM alert_deliveries d LEFT JOIN incidents i ON i.id = d.incident_id LEFT JOIN monitors m ON m.id = i.monitor_id LEFT JOIN nodes a ON a.id = i.node_id WHERE `+tr+` AND `+sc, append(ta, sa...)...)
	}
	if f.allowsType("system") && noIncident && f.MonitorID == "" {
		tr, ta := timeRange("u.created_at")
		scope, sa := "1=1", []any(nil)
		if f.ServerID != "" {
			scope, sa = "u.affected_object_type = 'node' AND u.affected_object_id = ?", []any{f.ServerID}
		}
		add(`SELECT 'u:' || u.id AS id, `+msExpr("u.created_at")+` AS at_ms, 'system' AS type, 'info' AS level, 'audit' AS kind,
			CASE WHEN u.affected_object_type = 'node' THEN u.affected_object_id ELSE '' END AS server_id, COALESCE(a.name,'') AS server_name, '' AS monitor_id, '' AS monitor_name, '' AS incident_id, 0 AS inc_num,
			u.action AS a, u.metadata_json AS b, u.actor_id AS c, u.affected_object_type AS d, 0 AS n1, 0 AS n2, 0 AS n3, 0 AS n4
			FROM audit_events u LEFT JOIN nodes a ON a.id = u.affected_object_id AND u.affected_object_type = 'node' WHERE `+tr+` AND `+scope, append(ta, sa...)...)
	}
	if len(branches) == 0 {
		return `SELECT '' AS id, 0 AS at_ms, '' AS type, '' AS level, '' AS kind, '' AS server_id, '' AS server_name, '' AS monitor_id, '' AS monitor_name, '' AS incident_id, 0 AS inc_num, '' AS a, '' AS b, '' AS c, '' AS d, 0 AS n1, 0 AS n2, 0 AS n3, 0 AS n4 WHERE 1=0`, nil
	}
	return strings.Join(branches, " UNION ALL "), args
}

func concatArgs(parts ...[]any) []any {
	var out []any
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// outer returns the WHERE clause applied over the union (levels, text and the exact time bounds).
func (f statusFilter) outer() (string, []any) {
	var clauses []string
	var args []any
	if len(f.Levels) > 0 {
		clauses = append(clauses, "level IN ?")
		args = append(args, f.Levels)
	}
	if !f.From.IsZero() {
		clauses = append(clauses, "at_ms >= ?")
		args = append(args, f.From.UnixMilli())
	}
	if !f.To.IsZero() {
		clauses = append(clauses, "at_ms <= ?")
		args = append(args, f.To.UnixMilli())
	}
	if f.Text != "" {
		like := "%" + strings.ToLower(escapeStatusLike(f.Text)) + "%"
		clauses = append(clauses, `(lower(server_name) LIKE ? ESCAPE '\' OR lower(monitor_name) LIKE ? ESCAPE '\' OR lower(a) LIKE ? ESCAPE '\' OR lower(b) LIKE ? ESCAPE '\' OR lower(kind) LIKE ? ESCAPE '\' OR lower(d) LIKE ? ESCAPE '\' OR CAST(inc_num AS TEXT) LIKE ? ESCAPE '\')`)
		args = append(args, like, like, like, like, like, like, like)
	}
	if len(clauses) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func escapeStatusLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "%", `\%`)
	return strings.ReplaceAll(s, "_", `\_`)
}

// statusPage returns up to limit rows older than the cursor (or newer when newer is set), newest first.
func statusPage(database *gorm.DB, f statusFilter, limit int, cursorMs int64, cursorID string, hasCursor bool, newer bool) ([]statusRow, error) {
	union, uargs := f.union()
	where, wargs := f.outer()
	args := append(append([]any{}, uargs...), wargs...)
	order := "ORDER BY at_ms DESC, id DESC"
	if hasCursor {
		cond := "(at_ms < ? OR (at_ms = ? AND id < ?))"
		if newer {
			cond, order = "(at_ms > ? OR (at_ms = ? AND id > ?))", "ORDER BY at_ms ASC, id ASC"
		}
		if where == "" {
			where = " WHERE " + cond
		} else {
			where += " AND " + cond
		}
		args = append(args, cursorMs, cursorMs, cursorID)
	}
	var rows []statusRow
	args = append(args, limit)
	if err := database.Raw("SELECT "+statusColumns+" FROM ("+union+")"+where+" "+order+" LIMIT ?", args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	if newer {
		for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
			rows[i], rows[j] = rows[j], rows[i]
		}
	}
	return rows, nil
}

func statusCount(database *gorm.DB, f statusFilter) (int64, error) {
	union, uargs := f.union()
	where, wargs := f.outer()
	var n int64
	err := database.Raw("SELECT COUNT(*) FROM ("+union+")"+where, append(append([]any{}, uargs...), wargs...)...).Scan(&n).Error
	return n, err
}

type statusGroup struct {
	Key   string `gorm:"column:k"`
	Label string `gorm:"column:label"`
	N     int64  `gorm:"column:n"`
}

// statusGroups counts events by one column (and an optional label column).
func statusGroups(database *gorm.DB, f statusFilter, keyCol, labelCol string, limit int) ([]statusGroup, int, error) {
	union, uargs := f.union()
	where, wargs := f.outer()
	if where == "" {
		where = " WHERE " + keyCol + " <> ''"
	} else {
		where += " AND " + keyCol + " <> ''"
	}
	label := "''"
	if labelCol != "" {
		label = "MAX(" + labelCol + ")"
	}
	var rows []statusGroup
	sql := "SELECT " + keyCol + " AS k, " + label + " AS label, COUNT(*) AS n FROM (" + union + ")" + where + " GROUP BY " + keyCol + " ORDER BY n DESC, k ASC"
	if err := database.Raw(sql, append(append([]any{}, uargs...), wargs...)...).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	total := len(rows)
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, total, nil
}

type statusBucket struct {
	Bucket int64
	Level  string
	N      int64
}

func statusHistogram(database *gorm.DB, f statusFilter, widthMs int64) ([]statusBucket, error) {
	union, uargs := f.union()
	where, wargs := f.outer()
	var rows []statusBucket
	sql := fmt.Sprintf("SELECT (at_ms / %d) * %d AS bucket, level, COUNT(*) AS n FROM (%s)%s GROUP BY bucket, level ORDER BY bucket", widthMs, widthMs, union, where)
	err := database.Raw(sql, append(append([]any{}, uargs...), wargs...)...).Scan(&rows).Error
	return rows, err
}
