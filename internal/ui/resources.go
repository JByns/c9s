package ui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/JByns/c9s/internal/rpc"
)

// Scope narrows a view to one cluster and optionally one node.
type Scope struct {
	ClusterID int
	NodeHost  string // cmon host name, e.g. ct-jessie-node4 or 10.10.16.8
	NodeIP    string
}

// matchNode reports whether a host value (name, IP or host:port) is the
// scoped node.
func (s Scope) matchNode(v string) bool {
	if s.NodeHost == "" {
		return true
	}
	for _, n := range []string{s.NodeHost, s.NodeIP} {
		if n != "" && (v == n || strings.HasPrefix(v, n+":")) {
			return true
		}
	}
	return false
}

// Column is one table column of a resource view.
type Column struct {
	Title string
	Value func(rpc.Map) string
	// Status marks the column whose value colours the whole row.
	Status bool
	Right  bool
}

// Resource is one browsable object type (clusters, nodes, jobs...).
type Resource struct {
	Name    string
	Aliases []string
	Columns []Column
	Fetch   func(c *rpc.Client, s Scope) ([]rpc.Map, error)
	// NodeField is the row field compared with a node scope.
	NodeField string
}

func field(path string) func(rpc.Map) string {
	return func(m rpc.Map) string { return rpc.Str(m, path) }
}

func firstOf(paths ...string) func(rpc.Map) string {
	return func(m rpc.Map) string {
		for _, p := range paths {
			if v := rpc.Str(m, p); v != "" {
				return v
			}
		}
		return ""
	}
}

func age(path string) func(rpc.Map) string {
	return func(m rpc.Map) string { return humanAge(rpc.Str(m, path)) }
}

func yesNo(path string) func(rpc.Map) string {
	return func(m rpc.Map) string {
		if rpc.Str(m, path) == "true" {
			return "yes"
		}
		return ""
	}
}

// parseTime reads cmon timestamps: RFC3339 strings or UNIX epoch seconds.
func parseTime(ts string) (time.Time, bool) {
	if ts == "" {
		return time.Time{}, false
	}
	if n, err := strconv.ParseInt(ts, 10, 64); err == nil && n > 1e9 {
		return time.Unix(n, 0), true
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.000Z", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, ts); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// humanAge turns a timestamp into a short age such as 6d8h or 21m.
func humanAge(ts string) string {
	t, ok := parseTime(ts)
	if !ok {
		return ts
	}
	d := time.Since(t)
	if d < 0 {
		return "in " + shortDur(-d)
	}
	return shortDur(d)
}

func shortDur(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	default:
		return fmt.Sprintf("%dd%dh", int(d.Hours())/24, int(d.Hours())%24)
	}
}

func humanSize(path string) func(rpc.Map) string {
	return func(m rpc.Map) string {
		n := num(rpc.Get(m, path))
		switch {
		case n == 0:
			return ""
		case n < 1<<10:
			return fmt.Sprintf("%.0fB", n)
		case n < 1<<20:
			return fmt.Sprintf("%.1fK", n/(1<<10))
		case n < 1<<30:
			return fmt.Sprintf("%.1fM", n/(1<<20))
		default:
			return fmt.Sprintf("%.1fG", n/(1<<30))
		}
	}
}

// statusColor maps cmon state words to a colour.
func statusColor(s string) tcell.Color {
	u := strings.ToUpper(s)
	switch {
	case u == "":
		return colInk
	case strings.Contains(u, "FAIL"), strings.Contains(u, "OFFLINE"), strings.Contains(u, "CRITICAL"),
		strings.Contains(u, "ABORT"), strings.Contains(u, "SHUTDOWN"), strings.Contains(u, "ERR"):
		return colBad
	case strings.Contains(u, "DEGRADED"), strings.Contains(u, "WARN"), strings.Contains(u, "RUNNING"),
		strings.Contains(u, "RECOVER"), strings.Contains(u, "DEFINED"), strings.Contains(u, "SCHEDULED"),
		strings.Contains(u, "MAINT"), u == "YES":
		return colWarn
	case strings.Contains(u, "COMPLETED"), strings.Contains(u, "FINISHED"):
		return colDim
	default:
		return colInk
	}
}

func clusterInfo(c *rpc.Client) ([]rpc.Map, error) {
	r, err := c.Call("/v2/clusters/", "getAllClusterInfo", rpc.Map{"with_hosts": true})
	if err != nil {
		return nil, err
	}
	cl := rpc.List(r, "clusters")
	sort.Slice(cl, func(i, j int) bool { return num(cl[i]["cluster_id"]) < num(cl[j]["cluster_id"]) })
	return cl, nil
}

func withCluster(p rpc.Map, s Scope) rpc.Map {
	if s.ClusterID > 0 {
		p["cluster_id"] = s.ClusterID
	}
	return p
}

// perCluster runs a cluster-only call for the scoped cluster, or for every
// cluster when not scoped, and tags each row with its cluster id.
func perCluster(c *rpc.Client, s Scope, fn func(cid int) ([]rpc.Map, error)) ([]rpc.Map, error) {
	ids := []int{s.ClusterID}
	if s.ClusterID == 0 {
		cl, err := clusterInfo(c)
		if err != nil {
			return nil, err
		}
		ids = ids[:0]
		for _, x := range cl {
			ids = append(ids, int(num(x["cluster_id"])))
		}
	}
	var out []rpc.Map
	var lastErr error
	for _, id := range ids {
		rows, err := fn(id)
		if err != nil {
			lastErr = err
			continue
		}
		for _, r := range rows {
			if _, ok := r["cluster_id"]; !ok {
				r["cluster_id"] = id
			}
		}
		out = append(out, rows...)
	}
	if len(out) == 0 && lastErr != nil && len(ids) == 1 {
		return nil, lastErr
	}
	return out, nil
}

// isDataNode is false for the hosts that are part of ClusterControl itself.
func isDataNode(h rpc.Map) bool {
	t := rpc.Str(h, "nodetype")
	return t != "controller" && t != "prometheus"
}

// Resources lists every view the TUI can show.
var Resources = []*Resource{
	{
		Name: "clusters", Aliases: []string{"cl", "cluster"},
		Columns: []Column{
			{Title: "ID", Value: field("cluster_id"), Right: true},
			{Title: "NAME", Value: field("cluster_name")},
			{Title: "TYPE", Value: field("cluster_type")},
			{Title: "VENDOR", Value: func(m rpc.Map) string { return strings.TrimSpace(rpc.Str(m, "vendor") + " " + rpc.Str(m, "version")) }},
			{Title: "STATE", Value: field("state"), Status: true},
			{Title: "NODES", Value: func(m rpc.Map) string {
				n := 0
				for _, h := range rpc.List(m, "hosts") {
					if isDataNode(h) {
						n++
					}
				}
				return fmt.Sprint(n)
			}, Right: true},
			{Title: "ALARMS", Value: func(m rpc.Map) string {
				c, w := num(rpc.Get(m, "alarm_statistics.critical")), num(rpc.Get(m, "alarm_statistics.warning"))
				if c+w == 0 {
					return ""
				}
				return fmt.Sprintf("%.0f/%.0f", c, w)
			}, Right: true},
			{Title: "MAINT", Value: yesNo("maintenance_mode_active")},
			{Title: "RECOVERY", Value: func(m rpc.Map) string {
				if rpc.Str(m, "cluster_auto_recovery") == "true" {
					return "on"
				}
				return "off"
			}},
			{Title: "STATUS", Value: field("status_text")},
		},
		Fetch: func(c *rpc.Client, s Scope) ([]rpc.Map, error) {
			rows, err := clusterInfo(c)
			if err != nil || s.ClusterID == 0 {
				return rows, err
			}
			var out []rpc.Map
			for _, r := range rows {
				if int(num(r["cluster_id"])) == s.ClusterID {
					out = append(out, r)
				}
			}
			return out, nil
		},
	},
	{
		Name: "nodes", Aliases: []string{"no", "node", "hosts"},
		NodeField: "hostname",
		Columns: []Column{
			{Title: "CID", Value: field("cluster_id"), Right: true},
			{Title: "CLUSTER", Value: field("_cluster_name")},
			{Title: "HOST", Value: field("hostname")},
			{Title: "IP", Value: field("ip")},
			{Title: "PORT", Value: field("port"), Right: true},
			{Title: "TYPE", Value: field("nodetype")},
			{Title: "ROLE", Value: field("role")},
			{Title: "STATUS", Value: func(m rpc.Map) string { return strings.TrimPrefix(rpc.Str(m, "hoststatus"), "CmonHost") }, Status: true},
			{Title: "VERSION", Value: field("version")},
			{Title: "MAINT", Value: yesNo("maintenance_mode_active")},
			{Title: "MESSAGE", Value: firstOf("message", "description")},
		},
		Fetch: func(c *rpc.Client, s Scope) ([]rpc.Map, error) {
			clusters, err := clusterInfo(c)
			if err != nil {
				return nil, err
			}
			var out []rpc.Map
			for _, cl := range clusters {
				if s.ClusterID > 0 && int(num(cl["cluster_id"])) != s.ClusterID {
					continue
				}
				for _, h := range rpc.List(cl, "hosts") {
					if !isDataNode(h) {
						continue
					}
					h["_cluster_name"] = rpc.Str(cl, "cluster_name")
					h["_cluster_type"] = rpc.Str(cl, "cluster_type")
					h["cluster_id"] = cl["cluster_id"]
					out = append(out, h)
				}
			}
			return out, nil
		},
	},
	{
		Name: "jobs", Aliases: []string{"jo", "job"},
		Columns: []Column{
			{Title: "ID", Value: field("job_id"), Right: true},
			{Title: "CID", Value: field("cluster_id"), Right: true},
			{Title: "STATUS", Value: field("status"), Status: true},
			{Title: "%", Value: field("progress_percent"), Right: true},
			{Title: "TITLE", Value: field("title")},
			{Title: "USER", Value: field("user_name")},
			{Title: "AGE", Value: age("created")},
			{Title: "MESSAGE", Value: field("status_text")},
		},
		Fetch: func(c *rpc.Client, s Scope) ([]rpc.Map, error) {
			r, err := c.Call("/v2/jobs/", "getJobInstances", withCluster(rpc.Map{"limit": 100, "ascending": false}, s))
			if err != nil {
				return nil, err
			}
			return rpc.List(r, "jobs"), nil
		},
	},
	{
		Name: "alarms", Aliases: []string{"al", "alarm"},
		NodeField: "hostname",
		Columns: []Column{
			{Title: "ID", Value: field("alarm_id"), Right: true},
			{Title: "CID", Value: field("cluster_id"), Right: true},
			{Title: "SEVERITY", Value: func(m rpc.Map) string { return strings.TrimPrefix(rpc.Str(m, "severity_name"), "ALARM_") }, Status: true},
			{Title: "COMPONENT", Value: field("component_name")},
			{Title: "HOST", Value: field("hostname")},
			{Title: "TITLE", Value: field("title")},
			{Title: "AGE", Value: age("created")},
			{Title: "MESSAGE", Value: field("message")},
		},
		Fetch: func(c *rpc.Client, s Scope) ([]rpc.Map, error) {
			r, err := c.Call("/v2/alarm/", "getAlarms", withCluster(rpc.Map{}, s))
			if err != nil {
				return nil, err
			}
			return rpc.List(r, "alarms"), nil
		},
	},
	{
		Name: "backups", Aliases: []string{"bk", "backup"},
		NodeField: "backup_host",
		Columns: []Column{
			{Title: "ID", Value: field("id"), Right: true},
			{Title: "CID", Value: field("cid"), Right: true},
			{Title: "STATUS", Value: field("status"), Status: true},
			{Title: "METHOD", Value: field("method")},
			{Title: "HOST", Value: field("backup_host")},
			{Title: "STORAGE", Value: firstOf("storage_host", "root_dir")},
			{Title: "SIZE", Value: humanSize("total_datadir_size"), Right: true},
			{Title: "ENC", Value: yesNo("encrypted")},
			{Title: "AGE", Value: age("created")},
			{Title: "TITLE", Value: field("title")},
		},
		Fetch: func(c *rpc.Client, s Scope) ([]rpc.Map, error) {
			r, err := c.Call("/v2/backup/", "getBackups", withCluster(rpc.Map{"ascending": false, "limit": 100}, s))
			if err != nil {
				return nil, err
			}
			rows := rpc.List(r, "backup_records")
			for _, b := range rows {
				b["cluster_id"] = b["cid"]
			}
			return rows, nil
		},
	},
	{
		Name: "logs", Aliases: []string{"lo", "log"},
		NodeField: "hostname",
		Columns: []Column{
			{Title: "CID", Value: field("cluster_id"), Right: true},
			{Title: "HOST", Value: field("hostname")},
			{Title: "PORT", Value: field("port"), Right: true},
			{Title: "FILE", Value: field("filename")},
			{Title: "SIZE", Value: humanSize("length"), Right: true},
			{Title: "COLLECTED", Value: age("created")},
		},
		Fetch: func(c *rpc.Client, s Scope) ([]rpc.Map, error) {
			return perCluster(c, s, func(cid int) ([]rpc.Map, error) {
				r, err := c.Call("/v2/log/", "listCollectedLogs", rpc.Map{"cluster_id": cid})
				if err != nil {
					return nil, err
				}
				return rpc.List(r, "files"), nil
			})
		},
	},
	{
		Name: "events", Aliases: []string{"ev", "clusterlog"},
		Columns: []Column{
			{Title: "AGE", Value: age("created")},
			{Title: "CID", Value: field("log_origins.cluster_id"), Right: true},
			{Title: "SEVERITY", Value: func(m rpc.Map) string { return strings.TrimPrefix(rpc.Str(m, "severity"), "LOG_") }, Status: true},
			{Title: "COMPONENT", Value: field("component")},
			{Title: "MESSAGE", Value: field("log_specifics.message_text")},
		},
		Fetch: func(c *rpc.Client, s Scope) ([]rpc.Map, error) {
			r, err := c.Call("/v2/log/", "getLogEntries", withCluster(rpc.Map{"limit": 300, "ascending": false}, s))
			if err != nil {
				return nil, err
			}
			return rpc.List(r, "log_entries"), nil
		},
	},
	{
		Name: "top", Aliases: []string{"processes", "pr", "ps"},
		NodeField: "node",
		Columns: []Column{
			{Title: "CID", Value: field("cluster_id"), Right: true},
			{Title: "NODE", Value: firstOf("node", "instance")},
			{Title: "PID", Value: field("pid"), Right: true},
			{Title: "USER", Value: field("user")},
			{Title: "DB", Value: field("db")},
			{Title: "COMMAND", Value: field("command")},
			{Title: "TIME", Value: field("time"), Right: true},
			{Title: "STATE", Value: field("state")},
			{Title: "QUERY", Value: firstOf("query", "info", "sql")},
		},
		Fetch: func(c *rpc.Client, s Scope) ([]rpc.Map, error) {
			return perCluster(c, s, func(cid int) ([]rpc.Map, error) {
				r, err := c.Call("/v2/clusters/", "getSqlProcesses", rpc.Map{"cluster_id": cid})
				if err != nil {
					return nil, err
				}
				rows := rpc.List(r, "processes")
				// Running queries first (longest first), idle and system threads last.
				active := func(r rpc.Map) bool {
					cmd := rpc.Str(r, "command")
					return firstOf("query", "info")(r) != "" && cmd != "Sleep" && cmd != "Daemon"
				}
				sort.SliceStable(rows, func(i, j int) bool {
					if ai, aj := active(rows[i]), active(rows[j]); ai != aj {
						return ai
					}
					return num(rows[i]["time"]) > num(rows[j]["time"])
				})
				return rows, nil
			})
		},
	},
	{
		Name: "queries", Aliases: []string{"qu", "topqueries"},
		NodeField: "instance",
		Columns: []Column{
			{Title: "CID", Value: field("cluster_id"), Right: true},
			{Title: "NODE", Value: field("instance")},
			{Title: "DB", Value: field("databaseName")},
			{Title: "COUNT", Value: field("count"), Right: true},
			{Title: "AVG ms", Value: field("waitMillisAvg"), Right: true},
			{Title: "MAX ms", Value: field("waitMillisMax"), Right: true},
			{Title: "SUM ms", Value: field("waitMillisSum"), Right: true},
			{Title: "QUERY", Value: field("statementPattern")},
		},
		Fetch: func(c *rpc.Client, s Scope) ([]rpc.Map, error) {
			return perCluster(c, s, func(cid int) ([]rpc.Map, error) {
				r, err := c.Call("/v2/clusters/", "getTopQueries", rpc.Map{"cluster_id": cid, "limit": 50})
				if err != nil {
					return nil, err
				}
				return rpc.List(r, "digests"), nil
			})
		},
	},
	{
		Name: "maintenance", Aliases: []string{"maint", "ma"},
		NodeField: "hostname",
		Columns: []Column{
			{Title: "CID", Value: field("cluster_id"), Right: true},
			{Title: "TARGET", Value: firstOf("hostname", "cluster_name")},
			{Title: "ACTIVE", Value: yesNo("is_active"), Status: true},
			{Title: "FROM", Value: age("initiate")},
			{Title: "UNTIL", Value: age("deadline")},
			{Title: "USER", Value: field("username")},
			{Title: "REASON", Value: field("reason")},
		},
		Fetch: func(c *rpc.Client, s Scope) ([]rpc.Map, error) {
			r, err := c.Call("/v2/maintenance/", "getMaintenance", withCluster(rpc.Map{}, s))
			if err != nil {
				return nil, err
			}
			var out []rpc.Map
			for _, rec := range rpc.List(r, "maintenance_records") {
				for _, p := range rpc.List(rec, "maintenance_periods") {
					p["hostname"] = rpc.Str(rec, "hostname")
					p["cluster_id"] = firstOf("cluster_id", "cluster.cluster_id")(rec)
					p["cluster_name"] = firstOf("cluster_name", "cluster.cluster_name")(rec)
					out = append(out, p)
				}
			}
			return out, nil
		},
	},
}

// FindResource resolves a :command to a resource.
func FindResource(name string) *Resource {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, r := range Resources {
		if r.Name == name {
			return r
		}
		for _, a := range r.Aliases {
			if a == name {
				return r
			}
		}
	}
	return nil
}

func num(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case string:
		f, _ := strconv.ParseFloat(t, 64)
		return f
	}
	return 0
}
