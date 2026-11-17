# c9s

**c9s** is a terminal user interface (TUI) client for
[ClusterControl](https://severalnines.com/clustercontrol/). It runs on the
ClusterControl controller and gives operators a full-screen, keyboard-driven
console for the database clusters that controller manages.

SSH to the ClusterControl controller, type `c9s`, and you get a live,
keyboard-driven view of every cluster that controller manages: health, alarms, jobs, backups, node
operations, maintenance, failover, top queries and database logs, all in one
SSH session and without a browser.

```
Controller: ct-jessie-cc1 [RO]     <0> all                    <enter> Node logs   <m>   Maintenance
Cmon:       https://localhost:9501 <1> Redis Cluster          <t>     Top         <p>   Promote
User:       dba                    <2> MySQL Galera Cluster   <a>     Alarms      <:>   Command
Clusters:   7 (1 failed)           <3> PostgreSQL Streaming   <s>     Start       </>   Filter
Alarms:     0                      <4> MySQL Replication      <S>     Stop        <d>   Describe
c9s:        0.2.0                  <5> MongoDB Replica        <ctrl-r> Restart    <esc> Back
╔════════════════════════════ nodes(MSSQL_AO_Async)[2] ═════════════════════════════╗
║ CID  CLUSTER         HOST             IP          PORT  TYPE   ROLE       STATUS    ║
║  35  MSSQL_AO_Async  ct-jessie-node4  10.10.16.8  1433  mssql  PRIMARY    Online    ║
║  35  MSSQL_AO_Async  ct-jessie-node5  10.10.16.9  1433  mssql  SECONDARY  Online    ║
╚═══════════════════════════════════════════════════════════════════════════════════╝
 ct-jessie-cc1:/MSSQL_AO_Async/nodes                                 ⟳ 5s (0s ago)  RO
```

---

## Contents

- [How it works](#how-it-works)
- [Requirements](#requirements)
- [Install](#install)
- [Quick start](#quick-start)
- [How c9s logs in](#how-c9s-logs-in)
- [The screen](#the-screen)
- [Views](#views)
- [The top 10 operational tasks](#the-top-10-operational-tasks)
- [Read-only and write mode](#read-only-and-write-mode)
- [Keyboard reference](#keyboard-reference)
- [Command-line reference](#command-line-reference)
- [Troubleshooting](#troubleshooting)
- [Development](#development)
- [Limitations and roadmap](#limitations-and-roadmap)

---

## How it works

c9s runs **on the ClusterControl controller**. You SSH to the controller and
start it there; it does not run on a
laptop or connect to a controller over the network.

```
 you ──ssh──▶ ┌──────────── ClusterControl controller ────────────┐
              │  ┌───────┐  RPC v2 (127.0.0.1:9501)  ┌──────────┐  │   SSH / DB    ┌──────────┐
              │  │  c9s  │ ────────────────────────▶ │   cmon   │ ─┼─────────────▶ │ DB nodes │
              │  └───────┘ ◀──────────────────────── └──────────┘  │               └──────────┘
              └────────────────────────────────────────────────────┘
```

- c9s is a single static Go binary. It has no agent, no database and no
  server side of its own.
- It talks only to the **local cmon** through the ClusterControl RPC v2 API.
  It refuses to start if the configured
  controller address is not this machine. It never logs in to database nodes
  itself.
- It logs in as a normal **ClusterControl user** and sees exactly what that
  user is allowed to see (its teams and cluster permissions).
- Each view is a table filled by one or more RPC calls (for example
  `getAllClusterInfo`, `getJobInstances`, `getAlarms`, `getBackups`). The
  table refreshes every 5 seconds by default.
- Every change (restart, backup, promote...) is sent to cmon as a regular
  **job**. The job is recorded in ClusterControl's job list and audit log, and
  c9s opens its live log so you can watch it run.

## Requirements

| What | Detail |
|---|---|
| Where | The ClusterControl controller host (where `cmon` runs) |
| ClusterControl | 2.x with the RPC v2 API on port 9501 (the default) |
| OS | Linux, amd64 or arm64 |
| Account | A ClusterControl user. With the user's RSA key on the controller (`~/.s9s/<user>.key`) there is no password prompt; otherwise any ClusterControl user and its password. |
| Terminal | At least 120 × 30, UTF-8 recommended. Works over SSH and in tmux/screen. |

To build from source you also need Go (the version in `go.mod`) and `make`.

## Install

Install c9s on the controller.

### From a release

Every `v*` tag builds release assets with GoReleaser:

| Asset | Use |
|---|---|
| `c9s_<ver>_linux_amd64.deb` / `.rpm` | Debian/Ubuntu or RHEL/Rocky packages; installs `/usr/bin/c9s` |
| `c9s_linux_<arch>.tar.gz` | Plain binary for any Linux |

```bash
# Debian / Ubuntu
sudo apt install ./c9s_*_linux_amd64.deb

# RHEL / Rocky / Alma
sudo dnf install ./c9s_*_linux_amd64.rpm

# Any Linux
tar xzf c9s_linux_amd64.tar.gz && sudo install c9s /usr/local/bin/
```

### From source

Build on the controller:

```bash
git clone https://github.com/JByns/c9s.git
cd c9s
make build && sudo make install     # /usr/local/bin/c9s
```

Or build on another machine and copy the binary over:

```bash
make linux            # bin/c9s-linux-amd64 and bin/c9s-linux-arm64
scp bin/c9s-linux-amd64 root@controller:/usr/local/bin/c9s
```

## Quick start

SSH to the controller and run it:

```bash
ssh root@controller
c9s
```

That is all. c9s connects to the local cmon, takes the user from the
controller's client config, logs in with that user's key and opens the
**clusters** view in read-only mode. If no client config or key is set up for
your Linux user, c9s asks for a ClusterControl user name and password instead.

A typical first session:

1. The **clusters** view lists every cluster with its state, node count,
   alarms and maintenance flag. Red rows need attention.
2. Press `1`–`9` to focus on one cluster (the numbers are shown in the
   header), or `0` for all clusters.
3. Press `enter` on a cluster to see its **nodes**. Press `enter` on a node to
   see that node's **log files**, and `enter` again to read one.
4. Press `:` and type a view name (`jobs`, `alarms`, `backups`, `top`...) to
   jump anywhere. Press `esc` to go back.
5. Watch the bottom bar: it always shows where you are, for example
   `ct-jessie-cc1:/MSSQL_AO_Async/ct-jessie-node4/logs`.
6. Press `?` for help and `ctrl-c` to quit.

When you need to change something (restart a node, take a backup), quit and
start again with `c9s --write`.

## How c9s logs in

c9s always connects to the cmon on the machine it runs on. It works out the
rest like this:

| Setting | Taken from (first that is set) |
|---|---|
| cmon address | `controller=` in the client config, else `https://127.0.0.1:9501`. Must be this machine. |
| User | `--user`, else `cmon_user=` in the client config, else asked at start |
| Key | `--key`, else `~/.s9s/<user>.key` if it exists |
| Password (only without a key) | `$CC_PASSWORD`, else asked at a hidden prompt |

The client config is the ClusterControl client configuration on the
controller: `~/.s9s/s9s.conf` of the Linux user running c9s, then
`/etc/s9s.conf`. c9s only reads it.

| Method | How it works |
|---|---|
| RSA key | cmon sends a challenge and c9s signs it with the user's private key. No password is sent or asked for. |
| Password | c9s sends the user name and password once to log in. |

The session lives in memory only and ends when you quit. If cmon expires it,
c9s logs in again on its own. c9s writes no files.

Examples:

```bash
c9s                       # user and key from the client config: no prompt
c9s --user admin          # another user; key ~/.s9s/admin.key or a password prompt
CC_PASSWORD=... c9s --user admin   # password without the prompt
```

Key login needs the user's RSA key pair registered in ClusterControl, with
the private key at `~/.s9s/<user>.key` on the controller. See the
ClusterControl user management documentation.

## The screen

```
┌─ header ────────────────────────────────────────────────────────────────────────────┐
│ info (controller, cmon,      cluster scopes      keys for this view    c9s logo     │
│ user, clusters, alarms)      <0> all <1> ...     <s> Start ...                      │
├─ main table ────────────────────────────────────────────────────────────────────────┤
│ view(scope)[rows]                                                                    │
│ rows, coloured by status: green ok, orange warning/busy, red failed/critical        │
├─ footer ────────────────────────────────────────────────────────────────────────────┤
│ path bar: controller:/Cluster/node/view/file         filter  ⟳ refresh (age)  RO/RW  │
└─────────────────────────────────────────────────────────────────────────────────────┘
```

- **Header, left:** the controller's host name (with `[RO]` or `[RW]`), the
  cmon URL, the user, how many clusters there are and how many have
  failed, and the open alarm count (red when above zero).
- **Header, middle:** the cluster scopes. `<0>` is all clusters and `<1>`–`<9>`
  are the first nine clusters by ID. Pressing the number limits every view to
  that cluster.
- **Header, right:** the keys that work in the current view. Write actions are
  listed even in read-only mode, so you can see what exists.
- **Table title:** the view name, the scope in brackets and the row count, for
  example `nodes(MSSQL_AO_Async)[2]`.
- **Path bar (bottom left):** where you are, as
  `controller:/Cluster_Name/node/view/file`, where `controller` is the host
  name of the controller. Spaces in cluster names show as `_`.
  Examples:
  - `ct-jessie-cc1:/clusters`
  - `ct-jessie-cc1:/MSSQL_AO_Async/nodes`
  - `ct-jessie-cc1:/MSSQL_AO_Async/ct-jessie-node4/logs`
  - `ct-jessie-cc1:/MySQL_Replication/logs/mysqld.log`
- **Status (bottom right):** the active filter, the refresh interval and how
  long ago the data was loaded, and `RO`/`RW`. Messages such as
  "job 1234 created" appear here for a few seconds.

## Views

Open a view with `:` and its name or alias. Add a cluster ID to scope it, for
example `:jobs 35`.

| View | Aliases | Shows | Columns |
|---|---|---|---|
| `clusters` | `cl` | All clusters | ID, NAME, TYPE, VENDOR, STATE, NODES, ALARMS (critical/warning), MAINT, RECOVERY, STATUS |
| `nodes` | `no` | Database nodes of the scoped cluster(s); the controller and Prometheus hosts are left out | CID, CLUSTER, HOST, IP, PORT, TYPE, ROLE, STATUS, VERSION, MAINT, MESSAGE |
| `jobs` | `jo` | Recent cmon jobs | ID, CID, STATUS, %, TITLE, USER, AGE, MESSAGE |
| `alarms` | `al` | Open alarms | ID, CID, SEVERITY, COMPONENT, HOST, TITLE, AGE, MESSAGE |
| `backups` | `bk` | Backup records | ID, CID, STATUS, METHOD, HOST, STORAGE, SIZE, ENC, AGE, TITLE |
| `logs` | `lo` | Database log files that cmon has collected from the nodes (for example `mysqld.log`, `postgresql-*.log`) | CID, HOST, PORT, FILE, SIZE, COLLECTED |
| `events` | `ev`, `clusterlog` | The cmon cluster log | AGE, CID, SEVERITY, COMPONENT, MESSAGE |
| `top` | `processes`, `pr`, `ps` | Running database processes; active queries first, longest running first | CID, NODE, PID, USER, DB, COMMAND, TIME, STATE, QUERY |
| `queries` | `qu` | Top query digests | CID, NODE, DB, COUNT, wait times, QUERY |
| `maintenance` | `maint`, `ma` | Scheduled and active maintenance periods | CID, TARGET (host or cluster), ACTIVE, FROM, UNTIL, USER, REASON |

When a node is in scope (after `enter` on a node), the `logs`, `top` and
`alarms` views show only that node.

Press `d` on any row to see the full record from cmon as JSON. This is the
quickest way to see a field that has no column.

## The top 10 operational tasks

These are the tasks c9s is built around. Steps 2–10 that change something need
write mode (see [below](#read-only-and-write-mode)).

### 1. Check cluster and node health

`:clusters` (the start view) shows every cluster's state and how many of its
nodes are up. Press `enter` on a cluster for its nodes, with each node's role
(primary, replica...) and status. Red means failed or offline.

### 2. Review and acknowledge alarms

`:alarms` lists open alarms, critical first. The header shows the total. Press
`a` on an alarm to acknowledge (ignore) it.

### 3. Follow and abort jobs

`:jobs` lists recent jobs with their progress. Press `enter` to open a job's
live log; it keeps updating until the job ends. Press `ctrl-k` to abort a
running job.

### 4. Create and restore backups

- **Create:** on a cluster press `b`, or in `:backups` press `c`. A form asks
  for the backup method (filled in for the cluster type, for example
  `xtrabackupfull`, `mariabackupfull`, `pg_basebackup`, `mongodump`,
  `mssqlfull`), the host (`auto` lets cmon choose) and the directory. c9s then
  opens the backup job's log.
- **Restore:** in `:backups`, select a `Completed` backup and press `r`. You
  must type the cluster name to confirm, because restore overwrites data.

### 5. Start, stop and restart nodes

In `:nodes`: `s` starts, `S` stops (type the node name to confirm) and `ctrl-r`
restarts the selected node.

### 6. Maintenance mode

On a cluster or a node press `m`. If no maintenance is active, a form asks for
the duration and the reason; alarms and auto-recovery are then held for that
time. If maintenance is active, `m` ends it. `:maintenance` lists all periods,
and `ctrl-d` ends the selected one.

### 7. Failover: promote a replica

In `:nodes`, select a replica and press `p`. Type the node name to confirm.
c9s refuses if the node is already the primary.

### 8. Rolling restart

On a cluster press `ctrl-r` and type the cluster name. cmon restarts the nodes
one at a time, keeping the cluster available.

### 9. Top processes and queries

`:top` shows what the databases are running now, with active queries at the
top. Press `ctrl-k` to kill the selected query. `:queries` shows the top query
digests (count and wait times) to find the heavy statements.

### 10. Logs

`:logs` lists the database log files cmon has collected. Press `enter` to read
a file: c9s shows the last 2000 lines, colours errors and warnings, and keeps
following the file while it is open. `:events` shows the cmon cluster log.

### Summary

| # | Task | View | Keys |
|---|---|---|---|
| 1 | Cluster & node health | `:clusters` `:nodes` | `enter` drill down |
| 2 | Alarms | `:alarms` | `a` acknowledge |
| 3 | Jobs | `:jobs` | `enter` live log, `ctrl-k` abort |
| 4 | Backups | `:backups`, `:clusters` | `c` / `b` create, `r` restore |
| 5 | Node start/stop/restart | `:nodes` | `s` / `S` / `ctrl-r` |
| 6 | Maintenance | `:clusters` `:nodes` `:maintenance` | `m` on/off, `ctrl-d` end |
| 7 | Failover | `:nodes` | `p` promote |
| 8 | Rolling restart | `:clusters` | `ctrl-r` |
| 9 | Top processes / queries | `:top` `:queries` | `ctrl-k` kill query |
| 10 | Logs | `:logs` `:events` | `enter` view file |

## Read-only and write mode

c9s starts **read-only**. Looking is always safe; changing needs a deliberate
step.

| Start with | Mode |
|---|---|
| `c9s` | Read-only (`RO`). Write keys show a message instead of acting. |
| `c9s --write` | Read-write (`RW`) |

In write mode every change still asks first:

- Simple actions (start, restart, acknowledge, abort, kill query, maintenance)
  ask yes/no.
- Risky actions (**stop**, **promote**, **rolling restart**, **restore**) make
  you type the node or cluster name.

The ClusterControl user's own permissions always apply on top: if the user may
not restart a node, cmon refuses the request.

## Keyboard reference

### Everywhere

| Key | Action |
|---|---|
| `:` | Command prompt (see below) |
| `/` | Filter the rows of the current view by text |
| `0`–`9` | Scope: all clusters, or cluster N from the header |
| `enter` | Drill down, or the view's main action |
| `d` | Describe the selected row (raw JSON) |
| `R` | Refresh now |
| `esc` | Clear the filter; press again to go back |
| `?` | Help |
| `ctrl-c` | Quit |
| arrows, `j`/`k`, `PgUp`/`PgDn`, `g`/`G` | Move in tables and text |

### Commands (`:`)

| Command | Action |
|---|---|
| `:<view>` | Open a view: `cl`, `no`, `jo`, `al`, `bk`, `lo`, `ev`, `top`, `qu`, `maint` |
| `:<view> <cluster-id>` | Open a view scoped to one cluster, for example `:nodes 35` |
| `:help` | Help |
| `:q` | Quit |

### Per view

| View | Key | Action | Write |
|---|---|---|---|
| clusters | `enter` | Nodes of the cluster | |
| | `l` / `j` / `a` / `t` | Logs / Jobs / Alarms / Top for the cluster | |
| | `b` | Backup now | yes |
| | `m` | Maintenance on/off | yes |
| | `ctrl-r` | Rolling restart | yes |
| nodes | `enter` | Log files of the node | |
| | `t` / `a` | Top / Alarms for the node | |
| | `s` / `S` / `ctrl-r` | Start / Stop / Restart | yes |
| | `m` | Maintenance on/off | yes |
| | `p` | Promote replica | yes |
| jobs | `enter` | Follow the live job log | |
| | `ctrl-k` | Abort job | yes |
| alarms | `a` | Acknowledge | yes |
| backups | `c` | Create backup | yes |
| | `r` | Restore backup | yes |
| logs | `enter` | View the file (follows live) | |
| top | `ctrl-k` | Kill query | yes |
| maintenance | `ctrl-d` | End maintenance | yes |

## Command-line reference

```
c9s [flags]                 open the UI
c9s rpc <path> <op> [json]  send one raw RPC v2 call to the local cmon and print the reply
c9s version                 print the version
```

| Flag | Default | Meaning |
|---|---|---|
| `--user` | `cmon_user` from the client config | ClusterControl user |
| `--key` | `~/.s9s/<user>.key` | RSA key for key login |
| `--refresh` | `5s` | How often views refresh |
| `--write` | off | Allow changes |

| Environment variable | Meaning |
|---|---|
| `CC_PASSWORD` | Password for password login |
| `C9S_ASCII` | Set to keep your locale as it is (see Troubleshooting) |
| `C9S_CONTROLLER` | Development only: another cmon URL on this machine, such as the mock |

`c9s rpc` is handy for checking what cmon returns:

```bash
c9s rpc /v2/clusters/ getAllClusterInfo '{"with_hosts":true}'
c9s rpc /v2/alarm/ getAlarms '{"cluster_id":35}'
```

## Troubleshooting

| Symptom | Cause and fix |
|---|---|
| `controller ... is not this machine` | The client config points at another host. c9s only runs on the controller: SSH to it and run c9s there. |
| `cannot reach cmon` | You are not on the controller, or cmon is stopped. Check `systemctl status cmon`. |
| `login as ... failed` | Wrong user, key or password. Check the key in `~/.s9s/<user>.key` is registered for that user in ClusterControl, or try `c9s --user <name>` with the password. |
| Boxes and lines draw as `?` | The terminal locale is not UTF-8 (common over SSH). c9s switches to `C.UTF-8` on its own; if your terminal really cannot show UTF-8, set `C9S_ASCII=1`. |
| Layout is cut off | Make the terminal at least 120 × 30. |
| A write key says "read-only" | Quit and start again with `c9s --write`. |
| `:logs` is empty for a cluster | cmon has not collected log files for it (log collection is off, or not supported for that database type). |
| A view is empty but you expect data | Press `d` on a nearby row, or run `c9s rpc ...` to see what cmon returns, and open an issue with the output. |

## Development

```
.
├── main.go                  flags, subcommands, login
├── internal/rpc             cmon RPC v2 client (password and key login, re-login)
├── internal/config          finds the local cmon, user and key (client config)
├── internal/ui
│   ├── app.go               layout, header, path bar, keys, refresh loop
│   ├── resources.go         views: RPC calls and columns
│   └── actions.go           per-view actions, forms, confirmations, job logs
└── cmd/mockcmon             fake controller for local testing
```

You can develop on any machine (Linux or macOS) with the mock controller,
which listens on `127.0.0.1:9501`, so c9s accepts it as local:

```bash
make mock            # terminal 1: fake cmon on http://127.0.0.1:9501
make dev             # terminal 2: c9s against the mock, in write mode
```

The mock has four clusters (Galera, Replication, ClickHouse, MSSQL) and keeps
state for jobs, maintenance and alarms, so every action can be tried safely.

```bash
make test            # go vet + unit tests
```

### Adding a view

Add a `Resource` in `internal/ui/resources.go` with its name, aliases,
columns and a `Fetch` function that makes the RPC call. Add keys for it in
`actionsFor` in `internal/ui/actions.go`. Use `c9s rpc` against a real
controller first to learn the field names.

### Releasing

Push a tag such as `v0.2.0`. GitHub Actions runs the tests, then GoReleaser
builds the archives and the `.deb`/`.rpm` packages and attaches them to the
GitHub release.

## Limitations and roadmap

- Changes have been tested against the mock controller. Test write actions on
  a non-production controller before relying on them.
- Views poll every few seconds; cmon's event stream (push updates) is not used
  yet.
- Not yet covered: deploying or importing clusters, adding or removing nodes,
  user and team management, configuration editing, ProxySQL/HAProxy views.

