package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/JByns/c9s/internal/rpc"
)

// Action is a key bound to the selected row of a view.
type Action struct {
	Key   string // "enter", "s", "S", "ctrl-r", ...
	Label string
	Write bool // changes something on the cluster: needs --write
	Run   func(row rpc.Map)
}

// actionsFor returns the keys of one view. These are the operational tasks
// listed in the help page.
func (a *App) actionsFor(view string) []Action {
	switch view {
	case "clusters":
		return []Action{
			{"enter", "Nodes", false, func(r rpc.Map) { a.drill("nodes", Scope{ClusterID: cid(r)}) }},
			{"l", "Logs", false, func(r rpc.Map) { a.drill("logs", Scope{ClusterID: cid(r)}) }},
			{"j", "Jobs", false, func(r rpc.Map) { a.drill("jobs", Scope{ClusterID: cid(r)}) }},
			{"a", "Alarms", false, func(r rpc.Map) { a.drill("alarms", Scope{ClusterID: cid(r)}) }},
			{"t", "Top", false, func(r rpc.Map) { a.drill("top", Scope{ClusterID: cid(r)}) }},
			{"b", "Backup now", true, a.createBackup},
			{"m", "Maintenance", true, a.toggleMaintenance},
			{"ctrl-r", "Rolling restart", true, a.rollingRestart},
		}
	case "nodes":
		return []Action{
			{"enter", "Node logs", false, func(r rpc.Map) { a.drill("logs", nodeScope(r)) }},
			{"t", "Top", false, func(r rpc.Map) { a.drill("top", nodeScope(r)) }},
			{"a", "Alarms", false, func(r rpc.Map) { a.drill("alarms", nodeScope(r)) }},
			{"s", "Start", true, func(r rpc.Map) { a.nodeJob(r, "start", "Starting Node", false) }},
			{"S", "Stop", true, func(r rpc.Map) { a.nodeJob(r, "stop", "Stopping Node", true) }},
			{"ctrl-r", "Restart", true, func(r rpc.Map) { a.nodeJob(r, "restart", "Restarting Node", false) }},
			{"m", "Maintenance", true, a.toggleMaintenance},
			{"p", "Promote", true, a.promote},
		}
	case "jobs":
		return []Action{
			{"enter", "Follow log", false, a.jobLog},
			{"ctrl-k", "Abort job", true, a.abortJob},
		}
	case "alarms":
		return []Action{
			{"a", "Acknowledge", true, a.ackAlarm},
		}
	case "backups":
		return []Action{
			{"c", "Create backup", true, a.createBackup},
			{"r", "Restore", true, a.restoreBackup},
		}
	case "logs":
		return []Action{
			{"enter", "View file", false, a.viewLogFile},
		}
	case "top":
		return []Action{
			{"ctrl-k", "Kill query", true, a.killProcess},
		}
	case "maintenance":
		return []Action{
			{"ctrl-d", "End maintenance", true, a.endMaintenance},
		}
	}
	return nil
}

func cid(r rpc.Map) int { return int(num(r["cluster_id"])) }

func nodeScope(r rpc.Map) Scope {
	return Scope{ClusterID: cid(r), NodeHost: rpc.Str(r, "hostname"), NodeIP: rpc.Str(r, "ip")}
}

func (a *App) drill(view string, sc Scope) {
	a.switchTo(FindResource(view), sc, "", true)
}

// ---- dialogs -------------------------------------------------------------

func (a *App) showModal(name string, p tview.Primitive, w, h int) {
	grid := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(p, h, 0, true).
			AddItem(nil, 0, 1, false), w, 0, true).
		AddItem(nil, 0, 1, false)
	a.pages.AddPage(name, grid, true, true)
	a.tv.SetFocus(p)
}

func (a *App) closeModal(name string) {
	a.pages.RemovePage(name)
	if front, _ := a.pages.GetFrontPage(); front == "detail" {
		a.tv.SetFocus(a.detail)
	} else {
		a.tv.SetFocus(a.table)
	}
}

// arrowNav lets the arrow keys move around a form, not only tab:
// left/right between buttons, up/down between fields and buttons. Text
// editing in input fields and an open dropdown list keep their arrows.
func arrowNav(form *tview.Form) *tview.Form {
	tab := func(back bool) *tcell.EventKey {
		if back {
			return tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone)
		}
		return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
	}
	form.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		item, button := form.GetFocusedItemIndex()
		if item >= 0 {
			if dd, ok := form.GetFormItem(item).(*tview.DropDown); ok && dd.IsOpen() {
				return ev
			}
		}
		switch ev.Key() {
		case tcell.KeyUp:
			return tab(true)
		case tcell.KeyDown:
			return tab(false)
		case tcell.KeyLeft, tcell.KeyRight:
			if button >= 0 {
				return tab(ev.Key() == tcell.KeyLeft)
			}
		}
		return ev
	})
	return form
}

// confirm asks yes/no. With word set, the user has to type it (for actions
// that are hard to undo, like stop, promote, restore).
func (a *App) confirm(question, word string, onYes func()) {
	form := arrowNav(tview.NewForm())
	form.SetBorder(true).SetTitle(" [red::b]confirm ").SetBorderColor(tcell.ColorRed)
	text := tview.NewTextView().SetDynamicColors(true).SetText(question)
	var typed *tview.InputField
	if word != "" {
		form.AddInputField(fmt.Sprintf("Type %q:", word), "", 30, nil, nil)
		typed = form.GetFormItem(0).(*tview.InputField)
	}
	form.AddButton("Cancel", func() { a.closeModal("confirm") })
	form.AddButton("OK", func() {
		if typed != nil && strings.TrimSpace(typed.GetText()) != word {
			a.flash("[yellow]not confirmed")
			return
		}
		a.closeModal("confirm")
		onYes()
	})
	form.SetCancelFunc(func() { a.closeModal("confirm") })
	box := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(text, 3, 0, false).AddItem(form, 0, 1, true)
	box.SetBorder(true).SetBorderColor(tcell.ColorRed).SetTitle(" [red::b]confirm ")
	form.SetBorder(false)
	h := 9
	if word != "" {
		h = 11
	}
	a.showModal("confirm", box, 72, h)
}

// ---- jobs ---------------------------------------------------------------

// createJob registers a cmon job and opens
// its live log.
func (a *App) createJob(clusterID int, title, command string, data rpc.Map) {
	c := a.client()
	data["clusterid"] = clusterID
	go func() {
		r, err := c.Call("/v2/jobs/", "createJobInstance", rpc.Map{
			"cluster_id": clusterID,
			"job": rpc.Map{
				// cmon rejects a job without its class name.
				"class_name": "CmonJobInstance",
				"title":      title,
				"job_spec":   rpc.Map{"command": command, "job_data": data},
			},
		})
		a.tv.QueueUpdateDraw(func() {
			if err != nil {
				a.flash("[red]" + tview.Escape(err.Error()))
				return
			}
			job, _ := r["job"].(map[string]any)
			a.flash(fmt.Sprintf("[green]job %s created: %s", rpc.Str(job, "job_id"), title))
			if job != nil {
				a.jobLog(job)
			}
		})
	}()
}

// jobLog shows a job's messages and keeps following them while it runs.
func (a *App) jobLog(job rpc.Map) {
	id := int(num(job["job_id"]))
	c := a.client()
	a.showDetail(fmt.Sprintf("job-%d", id), fmt.Sprintf(" job %d: %s ", id, tview.Escape(rpc.Str(job, "title"))), func() (string, bool) {
		r, err := c.Call("/v2/jobs/", "getJobLog", rpc.Map{"job_id": id, "ascending": true, "limit": 2000})
		if err != nil {
			return "[red]" + tview.Escape(err.Error()), false
		}
		var b strings.Builder
		for _, m := range rpc.List(r, "messages") {
			color := "white"
			switch rpc.Str(m, "message_status") {
			case "JOB_FAILED", "JOB_ERROR":
				color = "red"
			case "JOB_WARNING":
				color = "yellow"
			}
			fmt.Fprintf(&b, "[gray]%s [%s]%s\n", rpc.Str(m, "created"), color, tview.Escape(rpc.Str(m, "message_text")))
		}
		st, _ := c.Call("/v2/jobs/", "getJobInstance", rpc.Map{"job_id": id})
		status := rpc.Str(st, "job.status")
		fmt.Fprintf(&b, "\n[aqua::b]-- %s %s%% --", status, rpc.Str(st, "job.progress_percent"))
		return b.String(), status == "RUNNING" || status == "DEFINED" || status == "DEQUEUED"
	})
}

func (a *App) abortJob(job rpc.Map) {
	id := int(num(job["job_id"]))
	if st := rpc.Str(job, "status"); st != "RUNNING" && st != "DEFINED" {
		a.flash("[yellow]job " + strconv.Itoa(id) + " is " + st + ", nothing to abort")
		return
	}
	a.confirm(fmt.Sprintf("Abort job %d: %s?", id, rpc.Str(job, "title")), "", func() {
		a.simpleCall("/v2/jobs/", "killJobInstance", rpc.Map{"job_id": id, "signal": 15}, "abort signal sent to job "+strconv.Itoa(id))
	})
}

// simpleCall runs one RPC call and flashes the outcome.
func (a *App) simpleCall(path, op string, params rpc.Map, okMsg string) {
	c := a.client()
	go func() {
		_, err := c.Call(path, op, params)
		a.tv.QueueUpdateDraw(func() {
			if err != nil {
				a.flash("[red]" + tview.Escape(err.Error()))
				return
			}
			a.flash("[green]" + tview.Escape(okMsg))
			a.refresh()
		})
	}()
}

// ---- nodes & clusters -----------------------------------------------------

func nodeRef(r rpc.Map) rpc.Map {
	return rpc.Map{"hostname": rpc.Str(r, "hostname"), "port": int(num(r["port"]))}
}

func (a *App) nodeJob(r rpc.Map, command, title string, typed bool) {
	host := rpc.Str(r, "hostname")
	word := ""
	if typed {
		word = host
	}
	q := fmt.Sprintf("%s [white::b]%s:%s[-:-:-] in cluster %s?", title, host, rpc.Str(r, "port"), rpc.Str(r, "_cluster_name"))
	a.confirm(q, word, func() {
		a.createJob(cid(r), title, command, rpc.Map{
			"node": nodeRef(r), "hostname": host, "port": int(num(r["port"])),
		})
	})
}

func (a *App) promote(r rpc.Map) {
	role := strings.ToLower(rpc.Str(r, "role"))
	if role == "master" || role == "primary" {
		a.flash("[yellow]" + rpc.Str(r, "hostname") + " is already the primary")
		return
	}
	host := rpc.Str(r, "hostname")
	a.confirm(fmt.Sprintf("Promote replica [white::b]%s[-:-:-] to primary in cluster %s?\nThe current primary is demoted.", host, rpc.Str(r, "_cluster_name")), host, func() {
		a.createJob(cid(r), "Promoting Replica", "promote_replication_slave", rpc.Map{"node": nodeRef(r)})
	})
}

func (a *App) rollingRestart(r rpc.Map) {
	name := rpc.Str(r, "cluster_name")
	a.confirm(fmt.Sprintf("Rolling restart of all nodes in [white::b]%s[-:-:-]?\nNodes restart one at a time.", name), name, func() {
		a.createJob(cid(r), "Rolling Restart", "rolling_restart", rpc.Map{})
	})
}

// ---- maintenance -----------------------------------------------------------

// toggleMaintenance turns maintenance on with a form, or ends the active
// period when the cluster/node is already in maintenance.
func (a *App) toggleMaintenance(r rpc.Map) {
	isNode := a.res.Name == "nodes"
	target := rpc.Str(r, "cluster_name")
	if isNode {
		target = rpc.Str(r, "hostname")
	}
	if rpc.Str(r, "maintenance_mode_active") == "true" {
		a.endMaintenanceFor(cid(r), isNode, r)
		return
	}
	form := arrowNav(tview.NewForm())
	form.AddInputField("Duration (minutes):", "60", 10, tview.InputFieldInteger, nil)
	form.AddInputField("Reason:", "maintenance from c9s", 40, nil, nil)
	form.AddButton("Cancel", func() { a.closeModal("maint") })
	form.AddButton("Start", func() {
		mins, _ := strconv.Atoi(form.GetFormItem(0).(*tview.InputField).GetText())
		if mins <= 0 {
			mins = 60
		}
		reason := form.GetFormItem(1).(*tview.InputField).GetText()
		a.closeModal("maint")
		now := time.Now().UTC()
		p := rpc.Map{
			"initiate": now.Format("2006-01-02T15:04:05.000Z"),
			"deadline": now.Add(time.Duration(mins) * time.Minute).Format("2006-01-02T15:04:05.000Z"),
			"reason":   reason,
		}
		if isNode {
			p["hostname"] = target
		} else {
			p["cluster_id"] = cid(r)
		}
		a.simpleCall("/v2/maintenance/", "addMaintenance", p, fmt.Sprintf("maintenance on for %s (%d min)", target, mins))
	})
	form.SetCancelFunc(func() { a.closeModal("maint") })
	form.SetBorder(true).SetTitle(" [yellow::b]maintenance: " + tview.Escape(target) + " ")
	a.showModal("maint", form, 70, 9)
}

func (a *App) endMaintenanceFor(clusterID int, isNode bool, r rpc.Map) {
	c := a.client()
	go func() {
		rows, err := FindResource("maintenance").Fetch(c, Scope{ClusterID: clusterID})
		a.tv.QueueUpdateDraw(func() {
			if err != nil {
				a.flash("[red]" + tview.Escape(err.Error()))
				return
			}
			for _, p := range rows {
				if rpc.Str(p, "is_active") != "true" {
					continue
				}
				if isNode && !nodeScope(r).matchNode(rpc.Str(p, "hostname")) {
					continue
				}
				if !isNode && rpc.Str(p, "hostname") != "" {
					continue
				}
				a.endMaintenance(p)
				return
			}
			a.flash("[yellow]no active maintenance period found")
		})
	}()
}

func (a *App) endMaintenance(p rpc.Map) {
	if rpc.Str(p, "is_active") != "true" {
		a.flash("[yellow]this maintenance period is not active")
		return
	}
	target := firstOf("hostname", "cluster_name")(p)
	a.confirm(fmt.Sprintf("End maintenance for %s now?", target), "", func() {
		a.simpleCall("/v2/maintenance/", "removeMaintenance", rpc.Map{"UUID": rpc.Str(p, "UUID")}, "maintenance ended for "+target)
	})
}

// ---- alarms, processes --------------------------------------------------------

func (a *App) ackAlarm(r rpc.Map) {
	a.confirm(fmt.Sprintf("Acknowledge (ignore) alarm %s: %s?", rpc.Str(r, "alarm_id"), rpc.Str(r, "title")), "", func() {
		a.simpleCall("/v2/alarm/", "ignoreAlarm", rpc.Map{
			"alarm_id": int(num(r["alarm_id"])), "cluster_id": cid(r), "ignore": true,
		}, "alarm "+rpc.Str(r, "alarm_id")+" acknowledged")
	})
}

func (a *App) killProcess(r rpc.Map) {
	node := firstOf("node", "instance")(r)
	q := firstOf("query", "info")(r)
	if len(q) > 120 {
		q = q[:120] + "..."
	}
	a.confirm(fmt.Sprintf("Kill process %s on %s?\n%s", rpc.Str(r, "pid"), node, tview.Escape(q)), "", func() {
		a.simpleCall("/v2/stat", "killProcess", rpc.Map{
			"cluster_id": cid(r), "host_port": node, "pid": rpc.Str(r, "pid"),
		}, "process "+rpc.Str(r, "pid")+" killed on "+node)
	})
}

// ---- backups ---------------------------------------------------------------

// defaultBackupMethod suggests a cmon backup method for the cluster type.
func defaultBackupMethod(clusterType, vendor string) string {
	t := strings.ToUpper(clusterType)
	switch {
	case strings.Contains(t, "GALERA"), strings.Contains(t, "REPLICATION"), strings.Contains(t, "GROUP_REPL"):
		if strings.Contains(strings.ToLower(vendor), "mariadb") {
			return "mariabackupfull"
		}
		return "xtrabackupfull"
	case strings.Contains(t, "POSTGRES"):
		return "pg_basebackup"
	case strings.Contains(t, "MONGO"):
		return "mongodump"
	case strings.Contains(t, "MSSQL"):
		return "mssqlfull"
	}
	return ""
}

func (a *App) createBackup(r rpc.Map) {
	a.mu.Lock()
	cl := a.clusterByID(cid(r))
	a.mu.Unlock()
	if cl == nil {
		a.flash("[yellow]pick a cluster first")
		return
	}
	hosts := []string{"auto"}
	for _, h := range rpc.List(cl, "hosts") {
		if isDataNode(h) && !strings.Contains(rpc.Str(h, "nodetype"), "proxy") &&
			rpc.Str(h, "nodetype") != "keepalived" && rpc.Str(h, "nodetype") != "haproxy" {
			hosts = append(hosts, rpc.Str(h, "hostname")+":"+rpc.Str(h, "port"))
		}
	}
	name := rpc.Str(cl, "cluster_name")
	form := arrowNav(tview.NewForm())
	form.AddInputField("Method:", defaultBackupMethod(rpc.Str(cl, "cluster_type"), rpc.Str(cl, "vendor")), 24, nil, nil)
	form.AddDropDown("Backup host:", hosts, 0, nil)
	form.AddInputField("Directory:", "/root/backups", 40, nil, nil)
	form.AddButton("Cancel", func() { a.closeModal("backup") })
	form.AddButton("Create", func() {
		method := form.GetFormItem(0).(*tview.InputField).GetText()
		_, host := form.GetFormItem(1).(*tview.DropDown).GetCurrentOption()
		dir := form.GetFormItem(2).(*tview.InputField).GetText()
		a.closeModal("backup")
		data := rpc.Map{"backup_method": method, "backupdir": dir, "description": "Backup created by c9s."}
		if host != "auto" {
			h, p, _ := strings.Cut(host, ":")
			data["hostname"] = h
			if port, err := strconv.Atoi(p); err == nil {
				data["port"] = port
			}
		}
		a.createJob(cid(cl), "Create Backup", "backup", data)
	})
	form.SetCancelFunc(func() { a.closeModal("backup") })
	form.SetBorder(true).SetTitle(" [green::b]create backup: " + tview.Escape(name) + " ")
	a.showModal("backup", form, 72, 11)
}

func (a *App) restoreBackup(r rpc.Map) {
	if rpc.Str(r, "status") != "Completed" {
		a.flash("[yellow]only completed backups can be restored")
		return
	}
	id := rpc.Str(r, "id")
	a.mu.Lock()
	name := a.clusterName(cid(r))
	a.mu.Unlock()
	q := fmt.Sprintf("Restore backup %s (%s) to cluster [white::b]%s[-:-:-]?\nThis overwrites the data on the cluster.", id, rpc.Str(r, "method"), name)
	a.confirm(q, name, func() {
		a.createJob(cid(r), "Restore Backup", "restore_backup", rpc.Map{
			"backupid": int(num(r["id"])), "bootstrap": true,
		})
	})
}

// ---- logs ---------------------------------------------------------------

func (a *App) viewLogFile(r rpc.Map) {
	c := a.client()
	file, host := rpc.Str(r, "filename"), rpc.Str(r, "hostname")
	params := rpc.Map{"cluster_id": cid(r), "filename": file, "hostname": host}
	if p := int(num(r["port"])); p > 0 {
		params["port"] = p
	}
	a.showDetail(file, fmt.Sprintf(" %s: %s ", tview.Escape(host), tview.Escape(file)), func() (string, bool) {
		rep, err := c.Call("/v2/log/", "getCollectedLog", params)
		if err != nil {
			return "[red]" + tview.Escape(err.Error()), false
		}
		lines := strings.Split(strings.ReplaceAll(rpc.Str(rep, "data.content"), "\r\n", "\n"), "\n")
		if len(lines) > 2000 {
			lines = lines[len(lines)-2000:]
		}
		var b strings.Builder
		for _, l := range lines {
			color := "white"
			u := strings.ToUpper(l)
			switch {
			case strings.Contains(u, "ERROR"), strings.Contains(u, "FATAL"), strings.Contains(u, "PANIC"):
				color = "red"
			case strings.Contains(u, "WARN"):
				color = "yellow"
			}
			fmt.Fprintf(&b, "[%s]%s\n", color, tview.Escape(l))
		}
		return b.String(), true // collected logs keep growing; keep following
	})
}
