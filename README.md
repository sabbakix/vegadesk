# vegadesk

A single-host, [btop](https://github.com/aristocratos/btop)-styled terminal UI for managing a Linux
server — the parts of [Cockpit](https://cockpit-project.org/) you'd want over a plain SSH session,
with no web server, no browser, and nothing to expose on the network.

You SSH into a box the normal way, run `vegadesk`, and get a live dashboard plus management screens
for storage, services, containers, VMs, networking, logs, users, and the firewall — the same way you'd
run `btop` or `htop`, not a separate agent or daemon.

## Why

Cockpit is great, but it needs a running web server and a browser. If your only access to a machine is
`ssh`, that's often one more thing to install, expose, and keep patched. vegadesk is a single static
binary: copy it to the box (or build it there), run it, and it's gone when you exit.

## What it does

| Section | What you get |
|---|---|
| **Overview** | Live CPU/memory/network graphs and per-core gauges, disk usage — the "btop" screen |
| **Processes** | Sortable process table; terminate / force-kill with confirmation |
| **Storage** | Disk/partition topology, usage, mount/unmount, fstab-vs-mounted diffing |
| **Services** | systemd units — start/stop/restart/enable/disable, live log follow |
| **Containers** | Docker or Podman (auto-detected) — start/stop/remove, exec shell, follow logs |
| **Virtual Machines** | libvirt/virsh — start/shutdown/destroy, interactive console |
| **Networking** | Interfaces, addresses, live throughput |
| **Logs** | Browse recent journal entries, follow live |
| **Users** | Local accounts — lock/unlock/delete |
| **Firewall** | ufw status and rules, enable/disable |

Every screen a host can't support (no libvirt, no docker/podman, no systemd, ufw not installed, ...)
shows up dimmed in the sidebar with a plain-language reason instead of erroring — vegadesk is meant to
work across distros, not just the one it was built on.

Every action that changes something — stopping a service, deleting a user, unmounting a disk, force-
killing a process — asks for confirmation first, and destructive ones make you type the name back.

**Known limits (v1):** no partition creation / `mkfs` / LVM management (read-only topology + mount/
unmount only); no user-creation wizard (manage existing accounts only); Podman, libvirt, and firewalld
support is implemented and unit-tested against documented output formats but wasn't exercised against
a live daemon during development (none were installed on the build machine) — Docker, systemd, ufw, and
plain `ip`/`lsblk`/`journalctl` were.

## Install

**Requires Go 1.24+** (only to build it; the resulting binary has no runtime dependencies).

```bash
git clone <this-repo> vegadesk
cd vegadesk
go build -o vegadesk ./cmd/vegadesk
```

That produces a single `vegadesk` binary. Copy it wherever you'll run it — including to a different
server, since it has no runtime dependencies beyond the standard Linux tools it shells out to
(`lsblk`, `systemctl`, `docker`/`podman`, `virsh`, `ip`, `journalctl`, `ufw`) — those just need to be on
the target box for the corresponding section to work.

To install it onto your `$PATH` instead:

```bash
go install ./cmd/vegadesk
```
Or you copy paste the following command to install

```bash
curl -fsSL https://raw.githubusercontent.com/sabbakix/vegadesk/main/install.sh | sh
```

## Use

Run it:

```bash
./vegadesk
```

Over SSH, just run it in your session like you would `btop` or `top` — no extra setup, no port to open.

### Navigation

| Key | Action |
|---|---|
| `1`–`9`, `0` | Jump to a section |
| `Tab` / `→` | Next section |
| `Shift+Tab` / `←` | Previous section |
| `↑`/`↓`, `j`/`k`, `PgUp`/`PgDn` | Move within a table |
| `?` | Toggle full help |
| `q` | Quit (or cancel a dialog, if one's open) |
| `Ctrl+C` | Force quit, always — even mid-dialog |

The footer shows your current privilege posture (`root`, `sudo: passwordless`, `sudo: will prompt`, or
`read-only (no sudo)`) so it's always clear whether an action can actually run.

### Per-section keys

- **Processes** — `c`/`m`/`p` sort by CPU/memory/PID, `x` terminate, `X` force-kill
- **Storage** — `u` unmount selected, `m` mount a selected `/etc/fstab` entry, `r` refresh
- **Services** — `s` start, `S` stop, `R` restart, `e` enable, `d` disable, `l` follow logs, `r` refresh
- **Containers** — `s` start, `S` stop, `d` remove, `l` follow logs, `e` exec a shell, `r` refresh
- **Virtual Machines** — `s` start, `S` shutdown, `D` force destroy, `c` console (`Ctrl+]` to exit), `r` refresh
- **Networking** — `r` refresh
- **Logs** — `r` refresh, `f` follow live (`Ctrl+C` to return)
- **Users** — `l` lock, `u` unlock, `d` delete, `r` refresh (system accounts are hidden)
- **Firewall** — `e` enable, `d` disable, `a` authenticate (if it couldn't read status), `r` refresh

Any action needing root runs as root directly, via passwordless `sudo` if that's configured, or by
handing you a real password prompt (via a suspended, full-screen `sudo`) otherwise. If neither root nor
`sudo` is available at all, the affected screens tell you so instead of silently failing.

### Options

```bash
./vegadesk --color=truecolor   # force 24-bit color (auto-detection often under-reports over SSH/tmux)
./vegadesk --color=256         # force 256-color
./vegadesk --color=auto        # default: detect from the terminal
```
