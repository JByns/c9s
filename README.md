# c9s

**c9s** is a terminal user interface (TUI) client for
[ClusterControl](https://severalnines.com/clustercontrol/). It runs on the
ClusterControl controller and gives operators a full-screen, keyboard-driven
console for the database clusters that controller manages: health, alarms,
jobs, backups, node operations, maintenance, failover, top queries and logs.

```
Controller: ct-jessie-cc1 [RO]     <0> all                    <enter> Node logs   <m>   Maintenance
Cmon:       https://localhost:9501 <1> Redis Cluster          <t>     Top         <p>   Promote
User:       dba                    <2> MySQL Galera Cluster   <a>     Alarms      <:>   Command
Clusters:   7 (1 failed)           <3> PostgreSQL Streaming   <s>     Start       </>   Filter
Alarms:     0                      <4> MySQL Replication      <S>     Stop        <d>   Describe
c9s:        0.1.1                  <5> MongoDB Replica        <ctrl-r> Restart    <esc> Back
╔════════════════════════════ nodes(MSSQL_AO_Async)[2] ═════════════════════════════╗
║ CID  CLUSTER         HOST             IP          PORT  TYPE   ROLE       STATUS    ║
║  35  MSSQL_AO_Async  ct-jessie-node4  10.10.16.8  1433  mssql  PRIMARY    Online    ║
║  35  MSSQL_AO_Async  ct-jessie-node5  10.10.16.9  1433  mssql  SECONDARY  Online    ║
╚═══════════════════════════════════════════════════════════════════════════════════╝
 ct-jessie-cc1:/MSSQL_AO_Async/nodes                                 ⟳ 5s (0s ago)  RO
```

---

## Contents

**Getting started (follow in order)**

1. [Before you start](#1-before-you-start)
2. [Get c9s](#2-get-c9s)
3. [Install c9s on the controller](#3-install-c9s-on-the-controller)
4. [Start c9s and log in](#4-start-c9s-and-log-in)
5. [Read the screen](#5-read-the-screen)
6. [Your first five minutes: a guided tour](#6-your-first-five-minutes-a-guided-tour)
7. [Turn on write mode to make changes](#7-turn-on-write-mode-to-make-changes)

**Task guides**

- [Task 1: Check cluster and node health](#task-1-check-cluster-and-node-health)
- [Task 2: Review and acknowledge alarms](#task-2-review-and-acknowledge-alarms)
- [Task 3: Follow or abort a job](#task-3-follow-or-abort-a-job)
- [Task 4: Create a backup](#task-4-create-a-backup)
- [Task 5: Restore a backup](#task-5-restore-a-backup)
- [Task 6: Start, stop or restart a node](#task-6-start-stop-or-restart-a-node)
- [Task 7: Put a cluster or node in maintenance](#task-7-put-a-cluster-or-node-in-maintenance)
- [Task 8: Promote a replica (failover)](#task-8-promote-a-replica-failover)
- [Task 9: Rolling restart of a cluster](#task-9-rolling-restart-of-a-cluster)
- [Task 10: Find and kill a heavy query](#task-10-find-and-kill-a-heavy-query)
- [Task 11: Read database log files](#task-11-read-database-log-files)

**Reference**

- [How c9s works](#how-c9s-works)
- [How c9s logs in](#how-c9s-logs-in)
- [Views](#views)
- [Keyboard reference](#keyboard-reference)
- [Command-line reference](#command-line-reference)
- [Upgrade and uninstall](#upgrade-and-uninstall)
- [Troubleshooting](#troubleshooting)
- [FAQ](#faq)
- [Development](#development)
- [Limitations and roadmap](#limitations-and-roadmap)

---

# Getting started

## 1. Before you start

c9s runs **on the ClusterControl controller**, the machine where the `cmon`
service runs. You SSH to that machine and start c9s there. It does not run on
a laptop and does not connect to a controller over the network.

Check these on the controller before you install:

| Check | How | You need |
|---|---|---|
| You can SSH to the controller | `ssh root@<controller>` | A shell on the controller |
| cmon is running | `systemctl status cmon` | `active (running)` |
| CPU architecture | `uname -m` | `x86_64` or `aarch64` (the installer picks the right file) |
| git is installed | `git --version` | Any version. Install with `apt install git` or `dnf install git`. |
| A ClusterControl user | Your ClusterControl login name | Its RSA key on the controller, or its password (see [step 4](#4-start-c9s-and-log-in)) |
| Terminal size | Make your terminal window at least 120 × 30 | Smaller windows cut off columns |

## 2. Get c9s

On the controller, clone the repository:

```bash
git clone https://github.com/JByns/c9s.git
cd c9s
```

No GitHub account or login is needed: the repository is public.

To install a specific version instead of the newest code, check out its tag:

```bash
git checkout v0.1.1
```

## 3. Install c9s on the controller

In the `c9s` folder, run the installer:

```bash
sudo ./install.sh
```

The installer picks the right way by itself:

| If the controller has... | The installer... |
|---|---|
| Go (the version in `go.mod` or newer) | Builds c9s from the code you cloned |
| No Go (the usual case) | Downloads the ready-made c9s for this version (the newest release at or before your checkout) and CPU type (`amd64`/`arm64`) from the GitHub release, checks its checksum, and installs it. It uses `gh` if you are logged in, else `curl`. |

You should see something like:

```
==> downloading c9s_linux_amd64.tar.gz for v0.1.1 from github.com/JByns/c9s
==> checksum OK
==> installed /usr/local/bin/c9s (c9s 0.1.1)
==> ready: run  c9s  (add --write to make changes)
```

The program is installed as `/usr/local/bin/c9s`. To install somewhere else,
set `PREFIX`, for example without root into your home folder:

```bash
PREFIX=$HOME/.local ./install.sh       # installs ~/.local/bin/c9s
```

`make install` does the same as `./install.sh`.

**Check the install:**

```bash
c9s version
```

### Alternative: install a release package

Instead of cloning, you can install a package from the releases page,
**https://github.com/JByns/c9s/releases**:

| File | Use it on |
|---|---|
| `c9s_<version>_linux_amd64.deb` / `c9s_<version>_linux_arm64.deb` | Debian / Ubuntu |
| `c9s_<version>_linux_amd64.rpm` / `c9s_<version>_linux_arm64.rpm` | RHEL / Rocky / Alma / Oracle |
| `c9s_linux_amd64.tar.gz` / `c9s_linux_arm64.tar.gz` | Any Linux (plain binary) |

Download it on the controller and install it, for example:

```bash
curl -fLO https://github.com/JByns/c9s/releases/download/v0.1.1/c9s_0.1.1_linux_amd64.rpm
```

```bash
sudo apt install ./c9s_0.1.1_linux_amd64.deb     # Debian / Ubuntu
sudo dnf install ./c9s_0.1.1_linux_amd64.rpm     # RHEL / Rocky / Alma / Oracle
```

Packages install the program as `/usr/bin/c9s`.

## 4. Start c9s and log in

On the controller, type:

```bash
c9s
```

c9s connects to the cmon on this machine and logs in as a ClusterControl
user. What happens next depends on how the controller is set up for your
Linux user:

**Case A: no questions asked (key login).**
If the controller has a ClusterControl client config for your Linux user
(`~/.s9s/s9s.conf` or `/etc/s9s.conf`) with a user name, and that user's RSA
key at `~/.s9s/<user>.key`, c9s logs in with the key and opens immediately.
This is the usual case for `root` on a controller installed with the standard
installer.

**Case B: c9s asks for your user name and/or password.**
If there is no client config or key, c9s asks:

```
ClusterControl user: admin
Password for admin:
```

Type your ClusterControl login (the same user name and password you use to
sign in to ClusterControl). The password is not shown while you type.

**To log in as a different ClusterControl user**, name it:

```bash
c9s --user alice
```

c9s uses `~/.s9s/alice.key` if it exists, otherwise it asks for alice's
password.

When the login works, the **clusters** view opens. c9s always starts in
**read-only** mode (`[RO]` in the header): you can look at everything, but
nothing can be changed by accident. See [step 7](#7-turn-on-write-mode-to-make-changes)
for making changes.

If you see an error instead, go to [Troubleshooting](#troubleshooting).

## 5. Read the screen

```
┌─ header ────────────────────────────────────────────────────────────────────────────┐
│ ① controller info            ② cluster scopes    ③ keys for this view     logo      │
├─ main table ────────────────────────────────────────────────────────────────────────┤
│ ④ view(scope)[rows]                                                                  │
│ ⑤ rows, coloured by status                                                           │
├─ footer ────────────────────────────────────────────────────────────────────────────┤
│ ⑥ path bar: controller:/Cluster/node/view/file              ⑦ ⟳ refresh  RO/RW     │
└─────────────────────────────────────────────────────────────────────────────────────┘
```

| # | Part | What it tells you |
|---|---|---|
| ① | Controller info | Controller host name and mode (`[RO]` read-only / `[RW]` read-write), cmon address, your user, number of clusters (and how many failed), number of open alarms (red when above 0), c9s version |
| ② | Cluster scopes | `<0> all` and `<1>`–`<9>` for the first nine clusters by ID. Press the number to show only that cluster in every view. |
| ③ | Keys | The keys that work in the view you are looking at. Changes are listed even in read-only mode, so you can see what exists. |
| ④ | Table title | View name, scope in brackets, row count, and the active filter, e.g. `nodes(MSSQL_AO_Async)[2]` or `jobs(all)[2] </backup>` |
| ⑤ | Rows | **Green**: OK. **Orange/yellow**: warning or busy. **Red**: failed, offline or critical. |
| ⑥ | Path bar | Where you are. `ct-jessie-cc1:/MSSQL_AO_Async/ct-jessie-node4/logs` means: controller `ct-jessie-cc1`, cluster `MSSQL_AO_Async`, node `ct-jessie-node4`, view `logs`. Spaces in cluster names show as `_`. |
| ⑦ | Status | The refresh interval and how long ago the data was loaded, and `RO`/`RW`. Messages such as `job 1234 created` appear here for a few seconds. |

The table refreshes on its own every 5 seconds.

## 6. Your first five minutes: a guided tour

Follow these steps once to learn everything you need. Nothing here changes
your clusters. Watch the path bar at the bottom after each step.

| Step | Press | What happens | Path bar shows |
|---|---|---|---|
| 1 | (start) `c9s` | The clusters view lists every cluster | `ct-jessie-cc1:/clusters` |
| 2 | `↓` / `↑` | Move the highlight between rows | |
| 3 | `enter` on a cluster | Opens the nodes of that cluster | `ct-jessie-cc1:/MSSQL_AO_Async/nodes` |
| 4 | `enter` on a node | Opens the log files collected from that node | `ct-jessie-cc1:/MSSQL_AO_Async/ct-jessie-node4/logs` |
| 5 | `esc` | Goes back one step (to nodes) | `ct-jessie-cc1:/MSSQL_AO_Async/nodes` |
| 6 | `d` on a node | Shows everything cmon knows about that node (raw data). `esc` closes it. | |
| 7 | `0` | Scope back to all clusters | |
| 8 | `:` then type `jobs` and `enter` | Opens the jobs view | `ct-jessie-cc1:/jobs` |
| 9 | `/` then type `backup` and `enter` | Shows only rows that contain "backup". The table title shows the filter, e.g. `jobs(all)[2] </backup>`. | |
| 10 | `esc` | Clears the filter (press `esc` again to go back) | |
| 11 | `2` (any number from the header) | Every view now shows only that cluster | `ct-jessie-cc1:/MySQL_Galera_Cluster/jobs` |
| 12 | `:alarms`, `:backups`, `:top` | Jump to other views the same way | |
| 13 | `?` | Help page with all keys. `esc` closes it. | |
| 14 | `ctrl-c` (or `:q`) | Quit | |

That is the whole navigation model:

- **`enter` goes deeper, `esc` goes back.**
- **`:` jumps to any view**, **`/` filters**, **`0`–`9` picks a cluster**.
- **`d` shows the details** of any row.

## 7. Turn on write mode to make changes

c9s starts read-only. If you press a key that changes something (for example
`ctrl-r` on a node), the status line says:

```
read-only: quit and start c9s --write to restart
```

To make changes, quit (`ctrl-c`) and start c9s again with `--write`:

```bash
c9s --write
```

The header now shows `[RW]` in red. Even in write mode:

1. **Every change asks first.** A red **confirm** box shows what will happen.
   **Cancel** is highlighted first. Press `→` to highlight **OK** and `enter`
   to go ahead, or `enter` on **Cancel** (or `esc`) to stop.
2. **Risky changes need the name typed.** Stop node, promote, rolling restart
   and restore ask you to type the node or cluster name exactly. If it does not
   match, nothing happens and the status says `not confirmed`.
3. **Every change runs as a ClusterControl job.** It is recorded in the
   ClusterControl job list and audit log, under your user.
4. **c9s opens the job's live log** right after it starts, so you can watch it.
   Press `esc` to leave the log; the job keeps running.
5. **Your ClusterControl permissions apply.** If your user is not allowed to do
   something, cmon refuses and c9s shows the error.

Tip: use read-only `c9s` for day-to-day watching, and `c9s --write` only when
you plan to change something.

---

# Task guides

Each guide lists the exact keys. Guides marked **(write)** need
`c9s --write` (see [step 7](#7-turn-on-write-mode-to-make-changes)).

## Task 1: Check cluster and node health

1. Start `c9s`. The **clusters** view opens.
2. Look at each row:
   - `STATE`: `STARTED` is healthy; `DEGRADED` or `FAILURE` need attention (shown in red/orange).
   - `NODES`: how many nodes are up out of the total.
   - `ALARMS`: critical/warning alarm counts.
   - `MAINT`: whether maintenance is active.
   - `RECOVERY`: whether automatic recovery is on.
3. Press `enter` on a cluster to see its **nodes**:
   - `ROLE`: primary/master, replica/slave, and so on.
   - `STATUS`: `Online`, `ShutDown`, `Failed`, `OffLine`.
   - `MESSAGE`: cmon's latest message for the node.
4. Press `d` on any cluster or node for the full details.

## Task 2: Review and acknowledge alarms

1. Type `:alarms` and `enter`. The header's **Alarms** count shows the total.
2. Read `SEVERITY`, `HOST`, `TITLE` and `MESSAGE`. Press `d` for full details.
3. **(write)** To acknowledge (ignore) an alarm: select it and press `a`, then
   choose **OK**. The alarm leaves the list on the next refresh.

Shortcut: on a cluster or node row, `a` opens the alarms for just that cluster
or node.

## Task 3: Follow or abort a job

1. Type `:jobs` and `enter`. Newest jobs are at the top, with `STATUS` and `%`
   progress.
2. Press `enter` on a job to open its **live log**. It keeps updating while the
   job runs and scrolls to the newest line. Press `esc` to go back.
3. **(write)** To abort a running job: select it and press `ctrl-k`, then
   **OK**. Jobs that already finished cannot be aborted.

Shortcut: on a cluster row, `j` opens the jobs of that cluster.

## Task 4: Create a backup

**(write)**

1. Open `:clusters`, select the cluster, and press `b`.
   (Or in `:backups`, select any backup of that cluster and press `c`.)
2. A **create backup** form opens:
   - **Method:** filled in for the cluster type
     (`xtrabackupfull` for MySQL/Percona, `mariabackupfull` for MariaDB,
     `pg_basebackup` for PostgreSQL, `mongodump` for MongoDB, `mssqlfull` for
     SQL Server). Change it if you need a different method.
   - **Backup host:** `auto` lets ClusterControl choose, or pick a node.
   - **Directory:** where the backup is stored on the host
     (default `/root/backups`).
3. Use `↓`/`↑` (or `tab`) to move between fields, then highlight **Create**
   and press `enter`.
4. The status shows `job <id> created` and the backup job's live log opens.
5. When it finishes, the backup appears in `:backups` with status `Completed`.

## Task 5: Restore a backup

**(write)** Restore overwrites the data on the cluster. Be sure you have the
right backup.

1. Type `:backups` and `enter`. Optionally press a number to show one cluster.
2. Select a backup whose `STATUS` is `Completed`. Check `AGE`, `METHOD` and
   `SIZE`, and press `d` for details.
3. Press `r`.
4. The confirm box asks you to **type the cluster name**. Type it exactly and
   choose **OK**.
5. The restore job's live log opens. Follow it until it finishes.

## Task 6: Start, stop or restart a node

**(write)**

1. Open the nodes: `enter` on a cluster, or `:nodes`.
2. Select the node.
3. Press:
   - `s` to **start** it, then **OK**.
   - `S` (shift-s) to **stop** it. Type the node's name exactly as shown in the `HOST` column, then **OK**.
   - `ctrl-r` to **restart** it, then **OK**.
4. The job's live log opens. Back in `:nodes`, the `STATUS` column updates
   within a few seconds.

## Task 7: Put a cluster or node in maintenance

**(write)** Maintenance holds alarms and automatic recovery for a time, so
planned work does not raise alerts or trigger failover.

To start maintenance:

1. Select a cluster in `:clusters` or a node in `:nodes`.
2. Press `m`. A **maintenance** form opens:
   - **Duration (minutes):** default `60`.
   - **Reason:** default `maintenance from c9s`; write what you are doing.
3. Choose **Start**. The `MAINT` column shows `yes`.

To end maintenance early:

- Select the same cluster or node and press `m` again, then **OK**; or
- Open `:maintenance`, select the period and press `ctrl-d`, then **OK**.

`:maintenance` lists all periods with `TARGET`, `ACTIVE`, `FROM`, `UNTIL`,
`USER` and `REASON`.

## Task 8: Promote a replica (failover)

**(write)** The selected replica becomes the primary; the current primary is
demoted.

1. Open the nodes of the cluster (`enter` on the cluster).
2. Select a **replica** node (not the current primary; c9s refuses that).
3. Press `p`.
4. Type the node's name exactly as shown in the `HOST` column, and choose **OK**.
5. Follow the job's live log. Back in `:nodes`, the `ROLE` column shows the
   new primary.

## Task 9: Rolling restart of a cluster

**(write)** ClusterControl restarts the nodes one at a time, so the cluster
stays available.

1. In `:clusters`, select the cluster.
2. Press `ctrl-r`.
3. Type the cluster name to confirm, and choose **OK**.
4. Follow the job's live log until every node is back.

## Task 10: Find and kill a heavy query

1. Type `:top` and `enter` (or `t` on a cluster or node). Running queries are
   listed with the longest-running active ones first. `TIME` is seconds
   running, `QUERY` is the statement.
2. Type `:queries` to see the top query digests over time: `COUNT`, average
   and maximum wait, and the statement pattern. This finds the statements that
   cost the most overall.
3. **(write)** To kill a running query: in `:top`, select it and press
   `ctrl-k`. The confirm box shows the PID, node and query. Choose **OK**.

## Task 11: Read database log files

1. Type `:logs` and `enter` for all log files cmon has collected from the
   nodes (for example `mysqld.log`, `postgresql-*.log`), with `HOST`, `FILE`,
   `SIZE` and when it was collected.
   Shortcut: `enter` on a node, or `l` on a cluster, opens just its logs.
2. Press `enter` on a file to read it. c9s shows the last 2000 lines, colours
   errors red and warnings yellow, and keeps following the file while it is
   open. The path bar shows the file name.
3. Use `↑`/`↓`/`PgUp`/`PgDn` to scroll and `esc` to go back.
4. `:events` shows the cmon cluster log (controller events).

If a cluster has no files in `:logs`, cmon has not collected log files for it
(log collection is off, or not supported for that database type).

---

# Reference

## How c9s works

```
 you ──ssh──▶ ┌──────────── ClusterControl controller ────────────┐
              │  ┌───────┐  RPC v2 (127.0.0.1:9501)  ┌──────────┐  │   SSH / DB    ┌──────────┐
              │  │  c9s  │ ────────────────────────▶ │   cmon   │ ─┼─────────────▶ │ DB nodes │
              │  └───────┘ ◀──────────────────────── └──────────┘  │               └──────────┘
              └────────────────────────────────────────────────────┘
```

- c9s is a single static Go binary. It has no agent, no database and no
  server side of its own, and it writes no files.
- It talks only to the **local cmon** through the ClusterControl RPC v2 API.
  It refuses to start if the configured cmon address is not this machine. It
  never logs in to database nodes itself.
- It logs in as a normal **ClusterControl user** and sees exactly what that
  user is allowed to see (its teams and cluster permissions).
- Each view is a table filled by RPC calls (for example `getAllClusterInfo`,
  `getJobInstances`, `getAlarms`, `getBackups`) and refreshed every 5 seconds.
- Every change (restart, backup, promote...) is sent to cmon as a regular
  **job**, recorded in ClusterControl's job list and audit log.

## How c9s logs in

c9s always connects to the cmon on the machine it runs on and works out the
rest like this:

| Setting | Taken from (first that is set) |
|---|---|
| cmon address | `controller=` in the client config, else `https://127.0.0.1:9501`. Must be this machine. |
| User | `--user`, else `cmon_user=` in the client config, else asked at start |
| Key | `--key`, else `~/.s9s/<user>.key` if it exists |
| Password (only without a key) | `$CC_PASSWORD`, else asked at a hidden prompt |

The client config is the ClusterControl client configuration on the
controller: `~/.s9s/s9s.conf` of the Linux user running c9s, then
`/etc/s9s.conf`. c9s only reads it. Example:

```ini
[global]
cmon_user=admin
controller=https://localhost:9501
```

| Method | How it works |
|---|---|
| RSA key | cmon sends a challenge and c9s signs it with the user's private key. No password is sent or asked for. |
| Password | c9s sends the user name and password once to log in. |

The session lives in memory only and ends when you quit. If cmon expires it,
c9s logs in again on its own.

Key login needs the user's RSA key pair registered in ClusterControl, with the
private key at `~/.s9s/<user>.key` on the controller. See the ClusterControl
user management documentation.

## Views

Open a view with `:` and its name or alias. Add a cluster ID to scope it, for
example `:jobs 35`.

| View | Aliases | Shows | Columns |
|---|---|---|---|
| `clusters` | `cl` | All clusters | ID, NAME, TYPE, VENDOR, STATE, NODES, ALARMS (critical/warning), MAINT, RECOVERY, STATUS |
| `nodes` | `no` | Database nodes (controller and monitoring hosts left out) | CID, CLUSTER, HOST, IP, PORT, TYPE, ROLE, STATUS, VERSION, MAINT, MESSAGE |
| `jobs` | `jo` | Recent cmon jobs | ID, CID, STATUS, %, TITLE, USER, AGE, MESSAGE |
| `alarms` | `al` | Open alarms | ID, CID, SEVERITY, COMPONENT, HOST, TITLE, AGE, MESSAGE |
| `backups` | `bk` | Backup records | ID, CID, STATUS, METHOD, HOST, STORAGE, SIZE, ENC, AGE, TITLE |
| `logs` | `lo` | Database log files collected from the nodes | CID, HOST, PORT, FILE, SIZE, COLLECTED |
| `events` | `ev`, `clusterlog` | The cmon cluster log | AGE, CID, SEVERITY, COMPONENT, MESSAGE |
| `top` | `processes`, `pr`, `ps` | Running database processes, active and longest first | CID, NODE, PID, USER, DB, COMMAND, TIME, STATE, QUERY |
| `queries` | `qu` | Top query digests | CID, NODE, DB, COUNT, wait times, QUERY |
| `maintenance` | `maint`, `ma` | Maintenance periods | CID, TARGET, ACTIVE, FROM, UNTIL, USER, REASON |

After `enter` on a node, the `logs`, `top` and `alarms` views show only that
node.

## Keyboard reference

### Everywhere

| Key | Action |
|---|---|
| `↑` `↓` `PgUp` `PgDn` `Home` `End` | Move in tables and text |
| `enter` | Drill down, or the view's main action |
| `esc` | Clear the filter; press again to go back; closes boxes and pages |
| `:` | Command prompt (see below) |
| `/` | Filter the rows of the current view by text |
| `0`–`9` | Scope: all clusters, or cluster N from the header |
| `d` | Details of the selected row |
| `R` | Refresh now |
| `?` | Help |
| `ctrl-c` | Quit |

In confirm boxes and forms: `←`/`→` move between the buttons, `↑`/`↓` (or
`tab`/`shift-tab`) move between fields and buttons, `enter` presses the
highlighted button, `esc` cancels. **Cancel** is highlighted first, so pressing
`enter` straight away never changes anything.

### Commands (`:`)

| Command | Action |
|---|---|
| `:<view>` | Open a view: `cl`, `no`, `jo`, `al`, `bk`, `lo`, `ev`, `top`, `qu`, `maint` |
| `:<view> <cluster-id>` | Open a view for one cluster, for example `:nodes 35` |
| `:help` | Help |
| `:q` | Quit |

### Per view

| View | Key | Action | Needs `--write` |
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
| `--refresh` | `5s` | How often views refresh, e.g. `--refresh 10s` |
| `--write` | off | Allow changes |

| Environment variable | Meaning |
|---|---|
| `CC_PASSWORD` | Password for password login (skips the prompt) |
| `C9S_ASCII` | Set to `1` to keep your terminal locale as it is (see Troubleshooting) |
| `C9S_CONTROLLER` | Development only: another cmon URL on this machine, such as the mock |

`c9s rpc` prints what cmon returns, which helps when reporting a problem:

```bash
c9s rpc /v2/clusters/ getAllClusterInfo '{"with_hosts":true}'
c9s rpc /v2/alarm/ getAlarms '{"cluster_id":35}'
```

## Upgrade and uninstall

**Upgrade** (installed with `install.sh`): in your `c9s` folder, get the new
code and run the installer again:

```bash
cd c9s
git pull
sudo ./install.sh
c9s version
```

Pulling alone does not change the installed program; `./install.sh` does.

**Upgrade** (installed from a package): install the new release package the
same way; the package manager replaces the old version.

**Uninstall:**

```bash
sudo apt remove c9s        # Debian / Ubuntu
sudo dnf remove c9s        # RHEL / Rocky / Alma / Oracle
sudo rm /usr/local/bin/c9s # installed with install.sh or the plain binary
```

c9s keeps no files of its own, so nothing else needs cleaning up.

## Troubleshooting

| Message or symptom | Cause and fix |
|---|---|
| `install.sh`: `could not download ...` | The controller cannot reach github.com (no internet access or a proxy is needed). Copy a release file in by hand ([alternative](#alternative-install-a-release-package)), or set `https_proxy` and run `sudo ./install.sh` again. |
| `install.sh`: `cannot write to /usr/local/bin` | Run it with `sudo`, or use `PREFIX=$HOME/.local ./install.sh`. |
| `install.sh`: `this checkout has no release tag` | You cloned without tags or are on a branch with no release yet. Run `git fetch --tags`, or `git checkout v0.1.1`. |
| `c9s: command not found` | Not installed, or its folder is not in your `PATH`. Repeat [step 3](#3-install-c9s-on-the-controller). |
| `controller ... is not this machine` | The client config points at another host. c9s only runs on the controller: SSH to it and run c9s there. |
| `cannot reach cmon at ...` | You are not on the controller, or cmon is stopped. Check `systemctl status cmon`; start it with `systemctl start cmon`. |
| `login as ... failed` | Wrong user, key or password. Try `c9s --user <name>` and type the password. If key login fails, check that `~/.s9s/<user>.key` is the key registered for that user in ClusterControl. |
| Boxes and lines draw as `?` | The terminal locale is not UTF-8 (common over SSH). c9s switches to `C.UTF-8` on its own; if your terminal really cannot show UTF-8, set `C9S_ASCII=1`. |
| Columns are cut off | Make the terminal window at least 120 × 30. |
| `read-only: quit and start c9s --write ...` | You pressed a key that changes something. Quit and start `c9s --write`. |
| `not confirmed` | The name you typed in the confirm box did not match. Nothing was changed. |
| `only completed backups can be restored` | Pick a backup whose status is `Completed`. |
| `:logs` is empty for a cluster | cmon has not collected log files for it (log collection is off, or not supported for that database type). |
| A view is empty but you expect data | Press `R` to refresh. If still empty, run the matching `c9s rpc ...` call and include its output when reporting the problem. |
| The screen looks broken after resizing | Press `R`, or quit and start again. |

## FAQ

**Can I run c9s from my laptop?**
No. c9s runs on the ClusterControl controller. SSH to the controller and run
it there.

**Is it safe to run on production?**
Yes, in the default read-only mode it only reads. Changes need `--write`,
always ask for confirmation, and run as normal ClusterControl jobs.

**Does it need its own user or password?**
No. It uses your ClusterControl user and its permissions.

**Can several people use it at the same time?**
Yes. Each person runs their own c9s session on the controller; each logs in
separately.

**Does it store anything?**
No. It writes no files, and the login session ends when you quit.

**Why do some keys show in read-only mode?**
So you can see what actions exist. Pressing them in read-only mode only shows
a message.

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

**Build from source** (needs Go, the version in `go.mod`, and `make`):

```bash
git clone https://github.com/JByns/c9s.git
cd c9s
make build           # ./bin/c9s for this machine
make linux           # bin/c9s-linux-amd64 and bin/c9s-linux-arm64
make test            # go vet + unit tests
```

`make install` runs `install.sh`, which builds with your Go. To try a build on
a controller without Go, copy it there:

```bash
scp bin/c9s-linux-amd64 root@<controller>:/usr/local/bin/c9s
```

**Try every action safely with the mock controller.** It is a fake cmon on
`127.0.0.1:9501` with four clusters (Galera, Replication, ClickHouse, SQL
Server) that keeps state for jobs, maintenance and alarms. It works on Linux
or macOS:

```bash
make mock            # terminal 1: fake cmon on http://127.0.0.1:9501
make dev             # terminal 2: c9s against the mock, in write mode
```

**Adding a view:** add a `Resource` in `internal/ui/resources.go` with its
name, aliases, columns and a `Fetch` function that makes the RPC call. Add keys
for it in `actionsFor` in `internal/ui/actions.go`. Use `c9s rpc` on a real
controller first to learn the field names.

**Releasing:** push a tag such as `v0.2.0`. GitHub Actions runs the tests,
then GoReleaser builds the `.tar.gz`, `.deb` and `.rpm` files and attaches
them to the GitHub release.

## Limitations and roadmap

- Changes have been tested against the mock controller. Test write actions on
  a non-production controller before relying on them.
- Views poll every few seconds; cmon's event stream (push updates) is not used
  yet.
- Not yet covered: deploying or importing clusters, adding or removing nodes,
  user and team management, configuration editing, load balancer views.
