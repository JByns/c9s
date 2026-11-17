// mockcmon is a small fake of the cmon RPC v2 API for developing and demoing
// c9s without a real controller. It listens on plain HTTP on localhost and
// uses the field names a real 2.5 controller returns.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type M = map[string]any

var (
	mu     sync.Mutex
	start  = time.Now()
	nextID = 4600
	extra  []M
	maint  []M
	acked  = map[float64]bool{}
)

func ts(ago time.Duration) string {
	return time.Now().Add(-ago).UTC().Format("2006-01-02T15:04:05.000Z")
}

func host(name, ip, typ, role, status string, port int) M {
	return M{"hostname": name, "ip": ip, "port": port, "nodetype": typ, "role": role,
		"hoststatus": status, "version": "", "message": "Up and running.", "maintenance_mode_active": false}
}

func clusters() []any {
	cl := []any{
		M{"cluster_id": 13, "cluster_name": "MySQL Galera Cluster", "cluster_type": "GALERA", "vendor": "percona", "version": "8.0",
			"state": "STARTED", "status_text": "All nodes are operational.", "cluster_auto_recovery": true,
			"alarm_statistics": M{"critical": 0, "warning": 1},
			"hosts": []any{host("10.10.16.8", "10.10.16.8", "galera", "master", "CmonHostOnline", 3306),
				host("10.10.16.9", "10.10.16.9", "galera", "master", "CmonHostOnline", 3306),
				host("10.10.16.10", "10.10.16.10", "galera", "master", "CmonHostOnline", 3306),
				host("10.10.16.3", "10.10.16.3", "controller", "controller", "CmonHostOnline", 9500)}},
		M{"cluster_id": 16, "cluster_name": "MySQL Replication", "cluster_type": "REPLICATION", "vendor": "oracle", "version": "8.0",
			"state": "DEGRADED", "status_text": "1 node is shut down.", "cluster_auto_recovery": true,
			"alarm_statistics": M{"critical": 1, "warning": 0},
			"hosts": []any{host("10.10.16.5", "10.10.16.5", "mysql", "master", "CmonHostOnline", 3306),
				host("10.10.16.4", "10.10.16.4", "mysql", "slave", "CmonHostShutDown", 3306),
				host("10.10.16.6", "10.10.16.6", "mysql", "slave", "CmonHostOnline", 3306)}},
		M{"cluster_id": 19, "cluster_name": "ClickHouse", "cluster_type": "CLICKHOUSE", "vendor": "clickhouse", "version": "24.8",
			"state": "FAILURE", "status_text": "Node 10.10.16.4 is not reachable.", "cluster_auto_recovery": true,
			"alarm_statistics": M{"critical": 2, "warning": 0},
			"hosts": []any{host("10.10.16.4", "10.10.16.4", "clickhouse", "replica", "CmonHostFailed", 9000),
				host("10.10.16.5", "10.10.16.5", "clickhouse_keeper", "keeper", "CmonHostOffLine", 9181)}},
		M{"cluster_id": 35, "cluster_name": "MSSQL_AO_Async", "cluster_type": "MSSQL_AO_ASYNC", "vendor": "microsoft", "version": "2022",
			"state": "STARTED", "status_text": "All nodes are operational.", "cluster_auto_recovery": true,
			"hosts": []any{host("ct-jessie-node4", "10.10.16.8", "mssql", "PRIMARY", "CmonHostOnline", 1433),
				host("ct-jessie-node5", "10.10.16.9", "mssql", "SECONDARY", "CmonHostOnline", 1433)}},
	}
	mu.Lock()
	defer mu.Unlock()
	for _, c := range cl {
		cm := c.(M)
		for _, p := range maint {
			if p["is_active"] == true && p["hostname"] == "" && p["cluster_id"] == cm["cluster_id"] {
				cm["maintenance_mode_active"] = true
			}
			for _, h := range cm["hosts"].([]any) {
				if p["is_active"] == true && p["hostname"] == h.(M)["hostname"] {
					h.(M)["maintenance_mode_active"] = true
				}
			}
		}
	}
	return cl
}

func jobs() []any {
	pct := int(time.Since(start).Seconds()) * 4
	status := "RUNNING"
	if pct >= 100 {
		pct, status = 100, "FINISHED"
	}
	out := []any{
		M{"job_id": 4599, "cluster_id": 13, "status": status, "progress_percent": pct, "title": "Create Backup", "user_name": "admin", "created": ts(30 * time.Second), "status_text": "xtrabackup streaming"},
		M{"job_id": 4598, "cluster_id": 19, "status": "FAILED", "title": "Start Node", "user_name": "system", "created": ts(21 * time.Minute), "status_text": "Host is not reachable"},
		M{"job_id": 4597, "cluster_id": 35, "status": "FINISHED", "title": "Create Backup", "user_name": "admin", "created": ts(3 * time.Hour), "status_text": "Job finished."},
		M{"job_id": 4596, "cluster_id": 16, "status": "FINISHED", "title": "Rolling Restart", "user_name": "admin", "created": ts(30 * time.Hour), "status_text": "Job finished."},
	}
	mu.Lock()
	for _, e := range extra {
		out = append([]any{e}, out...)
	}
	mu.Unlock()
	return out
}

func handle(w http.ResponseWriter, r *http.Request) {
	var req M
	_ = json.NewDecoder(r.Body).Decode(&req)
	op, _ := req["operation"].(string)
	reply := M{"request_status": "Ok"}
	switch op {
	case "authenticateWithPassword", "authenticateResponse":
		http.SetCookie(w, &http.Cookie{Name: "cmon-sid", Value: "mock"})
	case "authenticate":
		reply["challenge"] = "mock-challenge"
	case "getAllClusterInfo":
		reply["clusters"] = clusters()
	case "getJobInstances":
		reply["jobs"] = scoped(jobs(), req, "cluster_id")
	case "getJobInstance":
		for _, j := range jobs() {
			if fmt.Sprint(j.(M)["job_id"]) == fmt.Sprint(req["job_id"]) {
				reply["job"] = j
			}
		}
	case "getJobLog":
		n := int(time.Since(start).Seconds())
		var msgs []any
		for i := 0; i <= n && i < 25; i++ {
			msgs = append(msgs, M{"created": ts(time.Duration(25-i) * time.Second), "message_status": "JOB_SUCCESS",
				"message_text": fmt.Sprintf("step %d: working", i)})
		}
		reply["messages"] = msgs
	case "killJobInstance", "removeMaintenance", "killProcess":
		if op == "removeMaintenance" {
			mu.Lock()
			for _, p := range maint {
				if p["UUID"] == req["UUID"] {
					p["is_active"] = false
				}
			}
			mu.Unlock()
		}
	case "getAlarms":
		all := []any{
			M{"alarm_id": 41.0, "cluster_id": 19, "severity_name": "ALARM_CRITICAL", "component_name": "Node", "hostname": "10.10.16.4", "title": "Server node unreachable", "message": "Server 10.10.16.4 is not reachable.", "created": ts(22 * time.Minute)},
			M{"alarm_id": 40.0, "cluster_id": 16, "severity_name": "ALARM_CRITICAL", "component_name": "Node", "hostname": "10.10.16.4", "title": "MySQL server is shut down", "message": "10.10.16.4:3306 is down.", "created": ts(2 * time.Hour)},
			M{"alarm_id": 37.0, "cluster_id": 13, "severity_name": "ALARM_WARNING", "component_name": "Backup", "hostname": "", "title": "Backup overdue", "message": "No backup in 24h.", "created": ts(26 * time.Hour)},
		}
		var out []any
		mu.Lock()
		for _, a := range all {
			if !acked[a.(M)["alarm_id"].(float64)] {
				out = append(out, a)
			}
		}
		mu.Unlock()
		reply["alarms"] = scoped(out, req, "cluster_id")
	case "ignoreAlarm":
		mu.Lock()
		acked[req["alarm_id"].(float64)] = true
		mu.Unlock()
	case "getBackups":
		reply["backup_records"] = scoped([]any{
			M{"id": 77, "cid": 13, "status": "Completed", "method": "xtrabackupfull", "backup_host": "10.10.16.8", "storage_host": "10.10.16.3", "total_datadir_size": 734003200, "created": ts(25 * time.Hour), "title": "BACKUP-77"},
			M{"id": 6, "cid": 35, "status": "Completed", "method": "mssqlcert", "backup_host": "ct-jessie-node4", "storage_host": "ct-jessie-node4", "encrypted": true, "created": ts(53 * 24 * time.Hour), "title": "BACKUP-6"},
			M{"id": 75, "cid": 16, "status": "Failed", "method": "mysqldump", "backup_host": "10.10.16.6", "storage_host": "10.10.16.3", "created": ts(73 * time.Hour), "title": "BACKUP-75"},
		}, req, "cid")
	case "listCollectedLogs":
		var files []any
		for _, c := range clusters() {
			cm := c.(M)
			if fmt.Sprint(cm["cluster_id"]) != fmt.Sprint(req["cluster_id"]) {
				continue
			}
			for _, h := range cm["hosts"].([]any) {
				hm := h.(M)
				if hm["nodetype"] == "controller" {
					continue
				}
				name := map[string]string{"galera": "mysqld.log", "mysql": "mysqld.log", "mssql": "errorlog", "clickhouse": "clickhouse-server.log"}[hm["nodetype"].(string)]
				if name == "" {
					name = "server.log"
				}
				files = append(files, M{"filename": name, "hostname": hm["hostname"], "port": fmt.Sprint(hm["port"]), "length": "24572", "created": fmt.Sprint(time.Now().Add(-90 * time.Second).Unix())})
			}
		}
		reply["files"] = files
	case "getCollectedLog":
		var b strings.Builder
		for i := 0; i < 40; i++ {
			lvl := "Note"
			if i%13 == 7 {
				lvl = "Warning"
			}
			if i == 33 {
				lvl = "ERROR"
			}
			fmt.Fprintf(&b, "%s 0 [%s] [MY-0%d] [Server] message %d from %v\r\n", ts(time.Duration(40-i)*time.Minute), lvl, 10000+i, i, req["hostname"])
		}
		reply["data"] = M{"content": b.String(), "filename": req["filename"], "hostname": req["hostname"]}
	case "getLogEntries":
		reply["log_entries"] = []any{}
	case "getSqlProcesses":
		if fmt.Sprint(req["cluster_id"]) == "16" || fmt.Sprint(req["cluster_id"]) == "13" {
			reply["processes"] = []any{
				M{"node": "10.10.16.5:3306", "pid": 812, "user": "app", "db": "shop", "command": "Query", "time": 47, "state": "Sending data", "query": "SELECT * FROM orders WHERE created > NOW() - INTERVAL 30 DAY"},
				M{"node": "10.10.16.5:3306", "pid": 5, "user": "event_scheduler", "db": "", "command": "Daemon", "time": 703295, "state": "Waiting on empty queue", "query": ""},
				M{"node": "10.10.16.6:3306", "pid": 77, "user": "system user", "db": "", "command": "Connect", "time": 12, "state": "Replica has read all relay log", "query": ""},
			}
		}
	case "getTopQueries":
		if fmt.Sprint(req["cluster_id"]) == "16" {
			reply["digests"] = []any{
				M{"instance": "10.10.16.5:3306", "databaseName": "shop", "count": 18211, "waitMillisAvg": 41, "waitMillisMax": 2210, "waitMillisSum": 746651, "statementPattern": "SELECT * FROM `orders` WHERE `created` > NOW ( ) - INTERVAL ? DAY"},
				M{"instance": "10.10.16.5:3306", "databaseName": "shop", "count": 9120, "waitMillisAvg": 3, "waitMillisMax": 90, "waitMillisSum": 27360, "statementPattern": "UPDATE `cart` SET `qty` = ? WHERE `id` = ?"},
			}
		}
	case "getMaintenance":
		mu.Lock()
		var recs []any
		for _, p := range maint {
			recs = append(recs, M{"hostname": p["hostname"], "cluster_id": p["cluster_id"], "maintenance_periods": []any{p}})
		}
		mu.Unlock()
		reply["maintenance_records"] = recs
	case "addMaintenance":
		mu.Lock()
		h, _ := req["hostname"].(string)
		maint = append(maint, M{"UUID": fmt.Sprintf("uuid-%d", len(maint)+1), "hostname": h, "cluster_id": req["cluster_id"],
			"initiate": req["initiate"], "deadline": req["deadline"], "reason": req["reason"], "username": "admin", "is_active": true})
		mu.Unlock()
	case "createJobInstance":
		mu.Lock()
		nextID++
		job, _ := req["job"].(M)
		e := M{"job_id": nextID, "cluster_id": req["cluster_id"], "status": "DEFINED", "title": job["title"], "user_name": "admin", "created": ts(0)}
		extra = append(extra, e)
		mu.Unlock()
		reply["job"] = e
	default:
		reply = M{"request_status": "InvalidRequest", "error_string": "mock does not implement " + op}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(reply)
}

// scoped keeps only items of the requested cluster, like cmon does.
func scoped(items []any, req M, key string) []any {
	want, ok := req["cluster_id"]
	if !ok {
		return items
	}
	var out []any
	for _, it := range items {
		if fmt.Sprint(it.(M)[key]) == fmt.Sprint(want) {
			out = append(out, it)
		}
	}
	return out
}

func main() {
	addr := flag.String("listen", "127.0.0.1:9501", "listen address")
	flag.Parse()
	http.HandleFunc("/", handle)
	log.Printf("mock cmon RPC v2 on http://%s", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
