// Package ui is the c9s terminal interface for ClusterControl.
package ui

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/JByns/c9s/internal/rpc"
)

const logo = `[orange::b] ____ ___
/ ___/ _ \ ___
| |  \_, /(_-<
| |___/_/ /__/
\____|        `

// Options configure the application.
type Options struct {
	Refresh    time.Duration
	AllowWrite bool
	Version    string
	// Host is the controller's short host name, shown in the header and
	// as the first part of the path bar.
	Host string
}

// App holds the UI state.
type App struct {
	tv   *tview.Application
	opts Options

	header, scopes, hints *tview.TextView
	table                 *tview.Table
	detail                *tview.TextView
	prompt                *tview.InputField
	path, status          *tview.TextView
	body                  *tview.Flex
	pages                 *tview.Pages

	mu         sync.Mutex
	rpc        *rpc.Client
	host       string
	res        *Resource
	scope      Scope
	filter     string
	rows       []rpc.Map
	shown      []rpc.Map
	history    []view
	clusters   []rpc.Map
	promptMode rune
	detailName string // e.g. "job 4520" or "mysqld.log", shown in the path
	detailFn   func() (string, bool)
	flashUntil time.Time
	lastLoad   time.Time
	alarmCount int
}

type view struct {
	res    *Resource
	scope  Scope
	filter string
}

// New builds the application.
func New(client *rpc.Client, opts Options) *App {
	a := &App{tv: tview.NewApplication(), rpc: client, opts: opts, host: opts.Host}
	a.build()
	a.switchTo(FindResource("clusters"), Scope{}, "", false)
	return a
}

// Run starts the event loop.
func (a *App) Run() error {
	go a.ticker()
	return a.tv.Run()
}

func (a *App) client() *rpc.Client {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.rpc
}

func (a *App) build() {
	a.header = tview.NewTextView().SetDynamicColors(true)
	a.scopes = tview.NewTextView().SetDynamicColors(true)
	a.hints = tview.NewTextView().SetDynamicColors(true)
	logoView := tview.NewTextView().SetDynamicColors(true).SetText(logo)

	top := tview.NewFlex().
		AddItem(a.header, 0, 3, false).
		AddItem(a.scopes, 0, 2, false).
		AddItem(a.hints, 0, 6, false).
		AddItem(logoView, 15, 0, false)

	a.table = tview.NewTable().SetSelectable(true, false).SetFixed(1, 0)
	a.table.SetBorder(true).SetBorderColor(tcell.ColorAqua)
	a.table.SetSelectedStyle(tcell.StyleDefault.Background(tcell.ColorAqua).Foreground(tcell.ColorBlack))
	a.table.SetInputCapture(a.tableKeys)

	a.detail = tview.NewTextView().SetDynamicColors(true).SetScrollable(true).SetWrap(true)
	a.detail.SetBorder(true).SetBorderColor(tcell.ColorAqua)
	a.detail.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		if ev.Key() == tcell.KeyEsc || ev.Rune() == 'q' {
			a.closeDetail()
			return nil
		}
		return ev
	})

	a.prompt = tview.NewInputField().SetFieldBackgroundColor(tcell.ColorDefault)
	a.prompt.SetBorder(true).SetBorderColor(tcell.ColorAqua)
	a.prompt.SetDoneFunc(a.promptDone)

	a.pages = tview.NewPages().
		AddPage("table", a.table, true, true).
		AddPage("detail", a.detail, true, false)

	a.path = tview.NewTextView().SetDynamicColors(true)
	a.status = tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignRight)
	footer := tview.NewFlex().AddItem(a.path, 0, 1, false).AddItem(a.status, 0, 1, false)

	a.body = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(top, 7, 0, false).
		AddItem(a.prompt, 0, 0, false).
		AddItem(a.pages, 0, 1, true).
		AddItem(footer, 1, 0, false)

	a.tv.SetRoot(a.body, true).SetFocus(a.table)
	a.tv.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		if ev.Key() == tcell.KeyCtrlC {
			a.tv.Stop()
			return nil
		}
		return ev
	})
}

// ---- data flow ----------------------------------------------------------

func (a *App) ticker() {
	t := time.NewTicker(a.opts.Refresh)
	for range t.C {
		a.mu.Lock()
		fn, front := a.detailFn, a.pages != nil
		a.mu.Unlock()
		if !front {
			continue
		}
		if name, _ := a.pages.GetFrontPage(); name == "detail" {
			if fn != nil {
				a.loadDetail(fn, true)
			}
			continue
		}
		if name, _ := a.pages.GetFrontPage(); name == "table" {
			a.refresh()
		}
		a.tv.QueueUpdateDraw(func() {
			a.mu.Lock()
			a.renderFooter()
			a.mu.Unlock()
		})
	}
}

// refresh fetches the current resource in the background and redraws.
func (a *App) refresh() {
	a.mu.Lock()
	res, sc, c := a.res, a.scope, a.rpc
	a.mu.Unlock()
	go func() {
		rows, err := res.Fetch(c, sc)
		var clusters []rpc.Map
		if res.Name == "clusters" && sc.ClusterID == 0 && err == nil {
			clusters = rows
		} else {
			clusters, _ = clusterInfo(c)
		}
		alarms := -1
		if ar, aerr := c.Call("/v2/alarm/", "getAlarms", rpc.Map{}); aerr == nil {
			alarms = len(rpc.List(ar, "alarms"))
		}
		if err == nil && sc.NodeHost != "" && res.NodeField != "" {
			kept := rows[:0]
			for _, r := range rows {
				if sc.matchNode(rpc.Str(r, res.NodeField)) {
					kept = append(kept, r)
				}
			}
			rows = kept
		}
		a.tv.QueueUpdateDraw(func() {
			a.mu.Lock()
			defer a.mu.Unlock()
			if clusters != nil {
				a.clusters = clusters
			}
			if alarms >= 0 {
				a.alarmCount = alarms
			}
			if res != a.res || sc != a.scope || c != a.rpc {
				return // view changed while loading
			}
			if err != nil {
				a.flash("[red]" + tview.Escape(err.Error()))
			} else {
				a.rows = rows
				a.lastLoad = time.Now()
			}
			a.renderTable()
			a.renderHeader()
			a.renderFooter()
		})
	}()
}

func (a *App) switchTo(res *Resource, sc Scope, filter string, push bool) {
	a.mu.Lock()
	if push && a.res != nil {
		a.history = append(a.history, view{a.res, a.scope, a.filter})
	}
	a.res, a.scope, a.filter = res, sc, filter
	a.rows, a.detailFn, a.detailName = nil, nil, ""
	a.renderTable()
	a.renderHeader()
	a.renderFooter()
	a.mu.Unlock()
	a.pages.SwitchToPage("table")
	a.tv.SetFocus(a.table)
	a.table.Select(1, 0)
	a.refresh()
}

func (a *App) back() {
	a.mu.Lock()
	if a.filter != "" {
		a.filter = ""
		a.renderTable()
		a.renderFooter()
		a.mu.Unlock()
		return
	}
	if len(a.history) == 0 {
		a.mu.Unlock()
		return
	}
	v := a.history[len(a.history)-1]
	a.history = a.history[:len(a.history)-1]
	a.mu.Unlock()
	a.switchTo(v.res, v.scope, v.filter, false)
}

// ---- rendering (callers hold a.mu) ---------------------------------------

func (a *App) renderTable() {
	sel, _ := a.table.GetSelection()
	a.table.Clear()
	for i, c := range a.res.Columns {
		cell := tview.NewTableCell(c.Title + " ").SetSelectable(false).
			SetTextColor(tcell.ColorWhite).SetAttributes(tcell.AttrBold).SetExpansion(1)
		if c.Right {
			cell.SetAlign(tview.AlignRight)
		}
		a.table.SetCell(0, i, cell)
	}
	a.shown = a.shown[:0]
	f := strings.ToLower(a.filter)
	for _, row := range a.rows {
		vals := make([]string, len(a.res.Columns))
		status := ""
		for i, c := range a.res.Columns {
			vals[i] = strings.ReplaceAll(c.Value(row), "\n", " ")
			if c.Status {
				status = vals[i]
			}
		}
		if f != "" && !strings.Contains(strings.ToLower(strings.Join(vals, " ")), f) {
			continue
		}
		a.shown = append(a.shown, row)
		color := statusColor(status)
		r := len(a.shown)
		for i, c := range a.res.Columns {
			cell := tview.NewTableCell(tview.Escape(vals[i]) + " ").SetTextColor(color).SetExpansion(1).SetMaxWidth(70)
			if c.Right {
				cell.SetAlign(tview.AlignRight)
			}
			a.table.SetCell(r, i, cell)
		}
	}
	title := fmt.Sprintf(" [aqua::b]%s([fuchsia]%s[aqua])[white][%d] ", a.res.Name, tview.Escape(a.scopeLabel()), len(a.shown))
	if a.filter != "" {
		title += fmt.Sprintf("[fuchsia::b]</%s> ", tview.Escape(a.filter))
	}
	a.table.SetTitle(title)
	if sel < 1 {
		sel = 1
	}
	if sel > len(a.shown) {
		sel = len(a.shown)
	}
	a.table.Select(sel, 0)
}

func (a *App) scopeLabel() string {
	if a.scope.ClusterID == 0 {
		return "all"
	}
	s := a.clusterName(a.scope.ClusterID)
	if a.scope.NodeHost != "" {
		s += "/" + a.scope.NodeHost
	}
	return s
}

// pathText is the location bar: context:/Cluster_Name/node/view[/item].
func (a *App) pathText() string {
	parts := []string{}
	if a.scope.ClusterID > 0 {
		parts = append(parts, strings.ReplaceAll(a.clusterName(a.scope.ClusterID), " ", "_"))
	}
	if a.scope.NodeHost != "" {
		parts = append(parts, a.scope.NodeHost)
	}
	parts = append(parts, a.res.Name)
	if name, _ := a.pages.GetFrontPage(); name == "detail" && a.detailName != "" {
		parts = append(parts, a.detailName)
	}
	return a.host + ":/" + strings.Join(parts, "/")
}

func (a *App) renderFooter() {
	a.path.SetText(" [black:orange:b] " + tview.Escape(a.pathText()) + " [-:-:-]")
	if time.Now().Before(a.flashUntil) {
		return
	}
	var b strings.Builder
	if a.filter != "" {
		fmt.Fprintf(&b, "[fuchsia]/%s  ", tview.Escape(a.filter))
	}
	fmt.Fprintf(&b, "[gray]⟳ %s", a.opts.Refresh)
	if !a.lastLoad.IsZero() {
		fmt.Fprintf(&b, " (%s ago)", shortDur(time.Since(a.lastLoad)))
	}
	if a.opts.AllowWrite {
		b.WriteString("  [red::b]RW ")
	} else {
		b.WriteString("  [green::b]RO ")
	}
	a.status.SetText(b.String())
}

// flash shows a message in the status line for a few seconds.
func (a *App) flash(msg string) {
	a.flashUntil = time.Now().Add(6 * time.Second)
	a.status.SetText(msg + " ")
}

func (a *App) renderHeader() {
	alarms, failed := a.alarmCount, 0
	for _, c := range a.clusters {
		if s := strings.ToUpper(rpc.Str(c, "state")); strings.Contains(s, "FAIL") {
			failed++
		}
	}
	mode := "[green]RO"
	if a.opts.AllowWrite {
		mode = "[red]RW"
	}
	fmt.Fprintf(a.header.Clear(),
		"[orange]Controller: [white]%s [%s[white]]\n[orange]Cmon:       [white]%s\n[orange]User:       [white]%s\n[orange]Clusters:   [white]%d ([red]%d failed[white])\n[orange]Alarms:     [%s]%d[white]\n[orange]c9s:        [white]%s",
		tview.Escape(a.host), mode, tview.Escape(a.rpc.BaseURL), tview.Escape(a.rpc.User),
		len(a.clusters), failed, map[bool]string{true: "red", false: "white"}[alarms > 0], alarms, a.opts.Version)

	var b strings.Builder
	fmt.Fprintf(&b, "[fuchsia]<0>[white] all\n")
	for i, c := range a.scopeClusters() {
		fmt.Fprintf(&b, "[fuchsia]<%d>[white] %s\n", i+1, tview.Escape(rpc.Str(c, "cluster_name")))
	}
	a.scopes.SetText(b.String())

	// Actions of this view first (the operational tasks), then navigation.
	type hint struct{ key, label string }
	var hs []hint
	for _, ac := range a.actionsFor(a.res.Name) {
		label := ac.Label
		if ac.Write && !a.opts.AllowWrite {
			label = "[gray]" + label
		}
		hs = append(hs, hint{ac.Key, label})
	}
	for _, h := range [][2]string{{":", "Command"}, {"/", "Filter"}, {"d", "Describe"}, {"esc", "Back"}, {"?", "Help"}} {
		hs = append(hs, hint{h[0], h[1]})
	}
	const rowsN = 6
	cols := (len(hs) + rowsN - 1) / rowsN
	b.Reset()
	for r := 0; r < rowsN; r++ {
		for c := 0; c < cols; c++ {
			i := c*rowsN + r
			if i >= len(hs) {
				continue
			}
			key := fmt.Sprintf("<%s>", hs[i].key)
			fmt.Fprintf(&b, "[blue]%-9s[white]%s%s", key, hs[i].label, strings.Repeat(" ", max(1, 17-tview.TaggedStringWidth(hs[i].label))))
		}
		b.WriteString("\n")
	}
	a.hints.SetText(b.String())
}

// scopeClusters are the clusters bound to the number keys 1-9.
func (a *App) scopeClusters() []rpc.Map {
	if len(a.clusters) > 9 {
		return a.clusters[:9]
	}
	return a.clusters
}

func (a *App) clusterName(id int) string {
	if c := a.clusterByID(id); c != nil {
		return rpc.Str(c, "cluster_name")
	}
	return strconv.Itoa(id)
}

func (a *App) clusterByID(id int) rpc.Map {
	for _, c := range a.clusters {
		if int(num(c["cluster_id"])) == id {
			return c
		}
	}
	return nil
}

// ---- input -------------------------------------------------------------

func (a *App) selected() rpc.Map {
	r, _ := a.table.GetSelection()
	a.mu.Lock()
	defer a.mu.Unlock()
	if r < 1 || r > len(a.shown) {
		return nil
	}
	return a.shown[r-1]
}

func keyName(ev *tcell.EventKey) string {
	switch ev.Key() {
	case tcell.KeyRune:
		return string(ev.Rune())
	case tcell.KeyEnter:
		return "enter"
	}
	if ev.Key() >= tcell.KeyCtrlA && ev.Key() <= tcell.KeyCtrlZ {
		return "ctrl-" + string(rune('a'+ev.Key()-tcell.KeyCtrlA))
	}
	return ""
}

func (a *App) tableKeys(ev *tcell.EventKey) *tcell.EventKey {
	if ev.Key() == tcell.KeyEsc {
		a.back()
		return nil
	}
	k := keyName(ev)
	for _, ac := range a.actionsFor(a.res.Name) {
		if ac.Key == k {
			row := a.selected()
			if row == nil {
				return nil
			}
			if ac.Write && !a.opts.AllowWrite {
				a.flash("[yellow]read-only: quit and start c9s --write to " + strings.ToLower(ac.Label))
				return nil
			}
			ac.Run(row)
			return nil
		}
	}
	switch r := ev.Rune(); {
	case ev.Key() == tcell.KeyEnter:
		a.describe()
	case r == ':' || r == '/':
		a.openPrompt(r)
	case r >= '0' && r <= '9':
		a.setScope(int(r - '0'))
	case r == 'd':
		a.describe()
	case r == 'R':
		a.refresh()
	case r == '?':
		a.help()
	default:
		return ev
	}
	return nil
}

func (a *App) setScope(n int) {
	sc := Scope{}
	if n > 0 {
		a.mu.Lock()
		list := a.scopeClusters()
		a.mu.Unlock()
		if n > len(list) {
			return
		}
		sc.ClusterID = int(num(list[n-1]["cluster_id"]))
	}
	a.switchTo(a.res, sc, "", false)
}

func (a *App) openPrompt(mode rune) {
	a.promptMode = mode
	label := "[aqua::b]:[-:-:-]"
	if mode == '/' {
		label = "[fuchsia::b]/[-:-:-]"
	}
	a.prompt.SetLabel(label).SetText("")
	a.body.ResizeItem(a.prompt, 3, 0)
	a.tv.SetFocus(a.prompt)
}

func (a *App) promptDone(key tcell.Key) {
	text := strings.TrimSpace(a.prompt.GetText())
	a.body.ResizeItem(a.prompt, 0, 0)
	a.tv.SetFocus(a.table)
	if key != tcell.KeyEnter {
		return
	}
	if a.promptMode == '/' {
		a.mu.Lock()
		a.filter = text
		a.renderTable()
		a.renderFooter()
		a.mu.Unlock()
		return
	}
	a.command(text)
}

// command runs a ":" command such as "nodes", "jobs 12" or "q".
func (a *App) command(text string) {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return
	}
	switch parts[0] {
	case "q", "quit", "q!":
		a.tv.Stop()
		return
	case "help":
		a.help()
		return
	}
	res := FindResource(parts[0])
	if res == nil {
		a.flash("[red]unknown command: " + tview.Escape(parts[0]) + " (try :help)")
		return
	}
	a.mu.Lock()
	sc := a.scope
	a.mu.Unlock()
	if len(parts) > 1 {
		cid, _ := strconv.Atoi(parts[1])
		sc = Scope{ClusterID: cid}
	}
	if res.NodeField == "" {
		sc.NodeHost, sc.NodeIP = "", ""
	}
	a.switchTo(res, sc, "", true)
}

// ---- detail views ------------------------------------------------------

func (a *App) describe() {
	row := a.selected()
	if row == nil {
		return
	}
	clean := rpc.Map{}
	for k, v := range row {
		if !strings.HasPrefix(k, "_") {
			clean[k] = v
		}
	}
	b, _ := json.MarshalIndent(clean, "", "  ")
	a.showDetail("describe", " describe ", func() (string, bool) { return tview.Escape(string(b)), false })
}

func (a *App) showDetail(name, title string, fn func() (string, bool)) {
	a.detail.SetTitle("[aqua::b]" + title)
	a.detail.SetText("loading...")
	a.pages.SwitchToPage("detail")
	a.tv.SetFocus(a.detail)
	a.mu.Lock()
	a.detailName = name
	a.renderFooter()
	a.mu.Unlock()
	a.loadDetail(fn, false)
}

// loadDetail fills the detail pane. Content that is still changing (a running
// job) keeps being refreshed by the ticker while it stays open.
func (a *App) loadDetail(fn func() (string, bool), fromTicker bool) {
	go func() {
		text, live := fn()
		a.tv.QueueUpdateDraw(func() {
			if name, _ := a.pages.GetFrontPage(); name != "detail" {
				return
			}
			row, _ := a.detail.GetScrollOffset()
			_, _, _, h := a.detail.GetInnerRect()
			atEnd := !fromTicker || row+h+1 >= strings.Count(a.detail.GetText(false), "\n")
			a.detail.SetText(text)
			if atEnd {
				a.detail.ScrollToEnd()
			} else {
				a.detail.ScrollTo(row, 0)
			}
			a.mu.Lock()
			if live {
				a.detailFn = fn
			} else {
				a.detailFn = nil
			}
			a.mu.Unlock()
		})
	}()
}

func (a *App) closeDetail() {
	a.mu.Lock()
	a.detailFn, a.detailName = nil, ""
	a.renderFooter()
	a.mu.Unlock()
	a.pages.SwitchToPage("table")
	a.tv.SetFocus(a.table)
	a.refresh()
}

func (a *App) help() {
	a.showDetail("help", " help ", func() (string, bool) { return helpText, false })
}

const helpText = `[aqua::b]TOP 10 CLUSTERCONTROL TASKS[-:-:-]
  1  Cluster & node health    :clusters  :nodes          enter = drill down
  2  Alarms                   :alarms                    a = acknowledge (ignore)
  3  Jobs                     :jobs                      enter = follow live log, ctrl-k = abort
  4  Backups                  :backups                   c = create, r = restore   (also b on a cluster)
  5  Node start/stop/restart  :nodes                     s = start, S = stop, ctrl-r = restart
  6  Maintenance mode         :clusters / :nodes         m = on/off   (:maintenance lists periods)
  7  Failover                 :nodes (replica)           p = promote to primary
  8  Rolling restart          :clusters                  ctrl-r
  9  Top processes / queries  :top  :queries             ctrl-k = kill query (in :top)
 10  Logs                     :logs (DB log files)       enter = view file   (:events = cmon cluster log)

[aqua::b]NAVIGATION[-:-:-]
  :<view> [cluster-id]   open a view (cl, no, jo, al, bk, lo, ev, top, qu, maint)
  0-9                    scope: all clusters / cluster N from the header
  enter                  drill down: cluster > nodes > node logs > file
  /text                  filter rows    esc  clear filter, then go back
  d                      describe (raw JSON from cmon)
  R                      refresh now
  :q  ctrl-c             quit

The bottom bar shows where you are: controller:/Cluster/node/view
Actions that change clusters need --write; each asks to confirm and runs as a
normal cmon job, recorded in the ClusterControl job list.

esc / q closes this page.`
