# opqu-sandbox — Specification

## Stack
- Debian trixie (Debian 13, stable as of mid-2025)
- mmdebstrap + systemd-nspawn + user namespaces (no real root)
- All global names (bridges, systemd units, machine names) prefixed with `opqu-sbx-`

---

## Filesystem Structure

```
sandboxes/
├── sbctl                         # control script
├── pkg-cache/                    # shared .deb cache across all sandboxes
├── rootfs-{name}/                # per-sandbox live rootfs
├── rootfs-{name}.base.tar.zst    # clean-slate snapshot for reset
└── conf/
    ├── global.conf               # distro, mirror, variant, container user
    ├── packages.base             # packages added to every sandbox
    ├── {name}.conf               # per-sandbox: ports, audio flag
    ├── {name}.packages           # per-sandbox extra packages
    └── {name}.mounts             # per-sandbox bind mounts
```

All paths in `sbctl` are resolved relative to the script's own location using:
```bash
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
```
No system-wide installation required. Do not hardcode absolute paths.

---

## R1 — Bootstrapping (`sbctl create {name}`)

- Merge package lists: `conf/packages.base` + `conf/{name}.packages`
  (deduplicated, newline-separated, passed as comma-separated to mmdebstrap)
- Run mmdebstrap:
  ```bash
  mmdebstrap \
    --variant=required \
    --include={merged-package-list} \
    --aptopt="Dir::Cache::archives \"$SCRIPT_DIR/pkg-cache/\";" \
    --customize-hook='systemctl enable systemd-networkd' \
    trixie \
    "$SCRIPT_DIR/rootfs-{name}" \
    "$MIRROR"
  ```
  - `--variant=required` is the baseline (minimal viable system)
  - `--aptopt` with `$SCRIPT_DIR/pkg-cache/` (absolute, derived at runtime)
    causes apt to cache downloaded .deb files there; reused on next create
  - `--customize-hook` runs after all packages are installed inside the
    chroot; used here to enable systemd-networkd so networking is live on
    first boot with no manual setup
- After bootstrap, check `/etc/subuid` and `/etc/subgid` for the current user.
  If entries are missing, print a clear error message and exit 1.
  Do not attempt to auto-modify system files.
  User must add entries manually: `sudo usermod --add-subuids 100000-165535 $USER`
- Apply user namespace ownership:
  ```bash
  systemd-nspawn --private-users=pick --private-users-ownership=auto \
    -D "$SCRIPT_DIR/rootfs-{name}" /bin/true
  ```
  Note: requires `newuidmap` and `newgidmap` to be setuid on the host
  (standard on most distros; if missing, this step fails with a clear error)
- Create base tarball immediately after bootstrap:
  ```bash
  tar --zstd -cf "$SCRIPT_DIR/rootfs-{name}.base.tar.zst" \
    -C "$SCRIPT_DIR" "rootfs-{name}/"
  ```
- pkg-cache accumulates over time; prune manually with:
  `rm sandboxes/pkg-cache/*.deb`

---

## R2 — No Real Root

- `--private-users=pick`: kernel assigns an unused subUID range automatically
- `--private-users-ownership=auto`: re-chowns rootfs files to match that range
- Container processes are unprivileged on the host despite seeing UID 0 inside
- The user that container processes run as inside the container is configurable
  via `CONTAINER_USER` in `conf/global.conf`; defaults to the user who invokes
  `sbctl` (i.e. `$(whoami)` evaluated at runtime, not hardcoded at install time)
- Requires `/etc/subuid` and `/etc/subgid` entries (checked at create time)
- This system does not create these entries; they are provisioned by the OS at
  user account creation time and shared with other tools (Docker, Podman, etc.)

---

## R3 — Networking

- Each sandbox uses `--network-zone=opqu-sbx-{name}`
  - Host auto-creates bridge `vz-opqu-sbx-{name}` and runs a DHCP server on it
  - Container runs `systemd-networkd` (enabled at bootstrap) as DHCP client
  - Outbound traffic is NAT'd by the host automatically — no config needed
- DNS: pass `--resolv-conf=replace-stub` to nspawn
  - nspawn reads the host's real upstream DNS servers (bypassing
    systemd-resolved's stub at 127.0.0.53 which is unreachable from the
    container's network namespace) and writes them into the container's
    `/etc/resolv.conf` — DNS works automatically with no daemon inside
- Port forwarding declared in `conf/{name}.conf`:
  ```bash
  PORTS="tcp:8080:8080 tcp:5432:5432"
  ```
  Each entry becomes a `--port=` flag in the nspawn invocation.
  Multiple sandboxes can run simultaneously without port collisions since
  each has its own IP on its own bridge.

---

## R4 — Bind Mounts

Declared in `conf/{name}.mounts`, one entry per line:
```
# host_path:container_path[:ro]
/home/user/projects:/projects
/home/user/data:/data:ro
```
- Lines starting with `#` and blank lines are ignored
- `:ro` suffix → `--bind-ro=`, otherwise `--bind=`
- The script reads this file at start time and builds flags dynamically
- To change mounts: edit the file, stop and restart the container

---

## R5 — Lifecycle

### Daemonizing
`systemd-nspawn --boot` does not self-daemonize. Use `systemd-run` to
launch it as a transient systemd unit on the host:

```bash
systemd-run \
  --unit="opqu-sbx-{name}" \
  --description="opqu-sandbox {name}" \
  --collect \
  systemd-nspawn \
    --boot \
    --machine=opqu-sbx-{name} \
    --directory="$SCRIPT_DIR/rootfs-{name}" \
    --network-zone=opqu-sbx-{name} \
    --resolv-conf=replace-stub \
    --private-users=pick \
    {bind-mount flags} \
    {port flags}
```

- `--machine=opqu-sbx-{name}` registers the container with machinectl under
  that name; all machinectl commands use this name
- Once registered, `machinectl shell opqu-sbx-{name}` and
  `machinectl poweroff opqu-sbx-{name}` work normally regardless of where
  the rootfs lives
- Logs go to journald; read with `journalctl -M opqu-sbx-{name}`
- `--collect` removes the transient unit automatically after container stops

### Commands
| Command | Implementation |
|---|---|
| `sbctl create {name}` | mmdebstrap + subuid check + tarball (see R1) |
| `sbctl start {name}` | `systemd-run` invocation above, assembled from conf + mounts |
| `sbctl shell {name}` | `machinectl shell opqu-sbx-{name}` |
| `sbctl stop {name}` | `machinectl poweroff opqu-sbx-{name}` |
| `sbctl reset {name}` | stop (if running) + `rm -rf rootfs-{name}/` + re-extract tarball + re-apply subuid ownership |
| `sbctl status` | `machinectl list` filtered to machines prefixed `opqu-sbx-`; also scan conf files to show configured-but-stopped sandboxes |

---

## R6 — Reset

```bash
sbctl reset {name}
```
1. If running: `machinectl poweroff opqu-sbx-{name}`, wait for stop
2. `rm -rf "$SCRIPT_DIR/rootfs-{name}"`
3. `tar --zstd -xf "$SCRIPT_DIR/rootfs-{name}.base.tar.zst" -C "$SCRIPT_DIR"`
4. Re-apply subuid ownership (same nspawn one-shot as in create)

To fully re-bootstrap from scratch: run `sbctl create {name}` again,
which overwrites both the rootfs and the base tarball.

---

## R7 — Audio (Optional, implement last)

- Opt-in in `conf/{name}.conf`: `AUDIO=yes`
- When enabled, add to nspawn invocation:
  ```
  --bind=/run/user/$(id -u)/pipewire-0:/run/user/host/pipewire-0
  --setenv=PIPEWIRE_REMOTE=/run/user/host/pipewire-0
  ```
- PulseAudio compat works via `pipewire-pulse` installed inside container
- Known caveat: UID mismatch across the user namespace boundary may cause
  permission errors on the socket; may need `chmod` or `pactl` ACL adjustment
- Implement and test only after all other features are stable

---

## R8 — Complete Removal

To fully remove everything related to this system:

1. **Stop all running sandboxes**
   ```bash
   for name in $(machinectl list --no-legend | awk '{print $1}' | grep '^opqu-sbx-'); do
     machinectl poweroff "$name"
   done
   ```

2. **Remove the sandboxes directory** (rootfs, tarballs, cache, configs, script)
   ```bash
   rm -rf /path/to/sandboxes/
   ```

3. **Remove host network bridges if any linger after stop**
   nspawn normally tears down `vz-opqu-sbx-{name}` bridges when a sandbox
   stops. If any remain:
   ```bash
   ip link delete vz-opqu-sbx-{name}
   ```
   All opqu-sandbox bridges are identifiable by the `vz-opqu-sbx-` prefix.

4. **Remove stuck transient systemd units if any remain**
   All opqu-sandbox units are identifiable by the `opqu-sbx-` prefix:
   ```bash
   systemctl reset-failed 'opqu-sbx-*'
   ```

5. **subuid/subgid entries — no action needed**
   This system does not create entries in `/etc/subuid` or `/etc/subgid`.
   It only reads entries that already exist (created by the OS at user account
   setup time). Do not remove them — they are shared system entries also used
   by Docker, Podman, and other tools.

After step 2, nothing from this system remains on the host except transient
network and unit state that clears automatically on next reboot.

---

## Config File Formats

### `conf/global.conf`
```bash
DISTRO=trixie
MIRROR=http://deb.debian.org/debian
VARIANT=required
CONTAINER_USER=         # defaults to $(whoami) at runtime if left empty
```

### `conf/packages.base`
```
# One package per line. Applied to every sandbox on top of --variant=required.
# mmdebstrap --variant=required provides: base-files, bash, coreutils,
# dpkg, apt, and minimal dependencies.
# List everything else needed in all sandboxes here:
systemd
dbus
iproute2
iputils-ping
curl
ca-certificates
locales
```

### `conf/{name}.conf`
```bash
PORTS="tcp:8080:8080"   # space-separated, each becomes a --port= flag
AUDIO=no                # yes to bind PipeWire socket
```

### `conf/{name}.packages`
```
# Extra packages for this sandbox only, merged with packages.base at create time
nodejs
postgresql
```

### `conf/{name}.mounts`
```
# host_path:container_path[:ro]
/home/user/projects:/projects
/home/user/data:/data:ro
```

---

## Implementation Order

1. `sbctl create` — subuid check, mmdebstrap with cache + hook, tarball
2. `sbctl start` — conf parsing, mount/port flag assembly, systemd-run launch
3. `sbctl stop` / `sbctl shell` — machinectl wrappers
4. `sbctl reset` — stop + wipe + re-extract + re-chown
5. `sbctl status` — machinectl list filtered to `opqu-sbx-*` + conf file scan
6. Audio support in `sbctl start` — last, after core is verified working
