# opqu-sandbox — Specification

## Stack
- Debian trixie (Debian 13, stable as of mid-2025)
- mmdebstrap + systemd-nspawn (no user namespaces; real host UIDs inside container)
- Machine names and systemd units are prefixed with `opqu-sbx-`
- Network bridges are prefixed with `vz-` and use a shortened zone name (see R3)

## R1 — Bootstrapping (`sbxctl create {name}`)

- If `rootfs-{name}/` already exists, print a clear error and exit 1:
  `"sandbox '{name}' already exists; run 'sbxctl reset {name}' to wipe it"`
  Note: if a previous `sbxctl create` was run and the rootfs was later manually
  deleted but the tarball kept, running `sbxctl create` again will overwrite the
  tarball. This is expected and acceptable.
- Source `conf/global.conf` if it exists; all values have defaults so the file
  is optional. Defaults: `DISTRO=trixie`, `MIRROR=http://deb.debian.org/debian`,
  `VARIANT=standard`, `SANDBOX_USER=$(whoami)`.
- Resolve `SANDBOX_USER` (defaulting to `$(whoami)` if unset),
  then verify the user exists on the host with `id "$SANDBOX_USER"`.
  If the user does not exist, print a clear error and exit 1 before doing
  anything else:
  `"SANDBOX_USER '{name}' does not exist on the host; create it first"`
  This check must happen before mmdebstrap runs, since the user's UID is
  embedded in the bootstrap hook and cannot be corrected afterwards without
  a full re-bootstrap.
- Create `pkg-cache/` if it does not already exist:
  ```bash
  sudo mkdir -p "$ROOT_DIR/pkg-cache"
  ```
- Read `conf/{name}.conf` if it exists and source `AUDIO` from it (default:
  `AUDIO=no`). If `AUDIO=yes`, `pipewire-pulse` will be merged into the
  package list (see next step).
- Read `conf/{name}.packages` if it exists. Strip comment lines (starting with
  `#`) and blank lines. If `AUDIO=yes`, append `pipewire-pulse` to the
  resulting list (deduplicate if it was already listed explicitly). If the
  final list is non-empty, pass as a comma-separated list via `--include`.
  If the file does not exist or is empty after stripping (and `AUDIO=no`),
  omit `--include` entirely.
- Run `mmdebstrap` if available. If `mmdebstrap` is missing from the host, fall
  back to `debootstrap`.
- **If using `mmdebstrap`**: Run with both customize hooks in a single invocation:
  ```bash
  sudo mmdebstrap \
    --variant=$VARIANT \
    [--include={package-list}] \
    --skip=essential/unlink \
    --setup-hook='mkdir -p "$1/var/cache/apt/archives/"' \
    --setup-hook='sync-in "$ROOT_DIR/pkg-cache" /var/cache/apt/archives/' \
    --customize-hook='sync-out /var/cache/apt/archives "$ROOT_DIR/pkg-cache"' \
    --customize-hook='chroot "$1" systemctl enable systemd-networkd' \
    --customize-hook="chroot \"$1\" /bin/sh -c 'useradd -m -u $(id -u "$SANDBOX_USER") -s /bin/bash $SANDBOX_USER \
      && passwd -l root \
      && passwd -l $SANDBOX_USER'" \
    $DISTRO \
    "$ROOT_DIR/rootfs-{name}" \
    "$MIRROR"
  ```
- **If using `debootstrap` (fallback)**: Run in sequence since it lacks hooks:
  1. Bootstrap the base:
     ```bash
     sudo debootstrap \
       --variant=$VARIANT \
       [--include={package-list}] \
       $DISTRO \
       "$ROOT_DIR/rootfs-{name}" \
       "$MIRROR"
     ```
  2. Enable networking:
     ```bash
     sudo chroot "$ROOT_DIR/rootfs-{name}" systemctl enable systemd-networkd
     ```
  3. Create the user and lock root:
     ```bash
     sudo chroot "$ROOT_DIR/rootfs-{name}" /bin/sh -c "useradd -m -u $(id -u "$SANDBOX_USER") -s /bin/bash $SANDBOX_USER \
       && passwd -l root \
       && passwd -l $SANDBOX_USER"
     ```
  Note: `debootstrap` does not support the shared `pkg-cache/` logic; caches
  are ignored during fallback.
  - `--variant=$VARIANT` controls the baseline package set, read from
    `conf/global.conf`. Default is `standard`, which provides a complete,
    friction-free base system (systemd, networking tools, curl, sudo, etc.)
    with no additional configuration needed. Users who want a leaner image
    can set `VARIANT=required` and manage extra packages via `{name}.packages`.
  - `$DISTRO` and `$MIRROR` are also read from `conf/global.conf`.
  - The shared `pkg-cache/` is synchronized into
    `/var/cache/apt/archives/` at setup time and synchronized back out at
    the end via mmdebstrap's `sync-in`/`sync-out` special hooks.
  - `--skip=essential/unlink` is required so downloaded `.deb` files remain
    in `/var/cache/apt/archives/` long enough for the final `sync-out`.
  - Both `--customize-hook` flags go on the same mmdebstrap call; hooks run
    in order after all packages are installed inside the chroot.
  - `sudo` is required here because this project does not use user
    namespaces. `mmdebstrap` must run in its real-root mode so the resulting
    directory rootfs has normal ownership and permissions for later
    `systemd-nspawn` use.
  - The first hook enables systemd-networkd so networking is live on first
    boot with no manual setup. Because mmdebstrap hooks run on the host side,
    the command explicitly uses `chroot "$1"` to target the new rootfs.
  - The second hook creates the container user. `SANDBOX_USER` and its UID
    are resolved on the host before mmdebstrap runs. The hook explicitly
    enters the target rootfs via `chroot "$1"` before running `useradd` and
    `passwd`, so those commands affect the container image rather than the
    host. UID inside the container matches the host UID exactly, so
    bind-mounted files are owned correctly on both sides without any
    remapping.
  - `root` is locked (password disabled) — the only way to get a root shell
    inside is `sudo machinectl shell root@opqu-sbx-{name}` from the host,
    which already requires host sudo; no escalation path from inside.
  - `SANDBOX_USER` is also locked; entry is always via `machinectl shell`
    which does not require a password.
- Create base tarball immediately after bootstrap:
  ```bash
  sudo tar --zstd -cf "$ROOT_DIR/rootfs-{name}.base.tar.zst" \
    -C "$ROOT_DIR" "rootfs-{name}/"
  ```
  The tarball is root-managed for the same reason as the live rootfs.
- Any unexpected error during bootstrapping must log a clear message and exit 1.
- pkg-cache accumulates over time; prune manually with:
  `sudo rm {root}/pkg-cache/*.deb`

### Cache note for mmdebstrap hooks
`sync-in`/`sync-out` are mmdebstrap special hooks, while `"$1"` is the rootfs
path exposed to shell hooks. Construct the cache-related hooks so the
outside path expands in `sbxctl` but the inside path remains literal:
```bash
--setup-hook='mkdir -p "$1/var/cache/apt/archives/"'
--setup-hook='sync-in "'"$ROOT_DIR"'/pkg-cache" /var/cache/apt/archives/'
--customize-hook='sync-out /var/cache/apt/archives "'"$ROOT_DIR"'/pkg-cache"'
```

---

## R5 — Lifecycle

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
| `sbxctl create {name}` | `mmdebstrap` (with cache + hooks) or `debootstrap` (fallback) + tarball (see R1) |
| `sbxctl start {name}` | `sudo systemd-run` invocation above, assembled from conf + mounts |
| `sbxctl shell {name} [command...]` | `sudo machinectl shell SANDBOX_USER@opqu-sbx-{name} [command...]` |
| `sbxctl stop {name}` | if running: `sudo machinectl poweroff opqu-sbx-{name}`; if already stopped: exit 0 |
| `sbxctl reset {name}` | refuse if running + wipe rootfs + re-extract tarball (see R6) |
| `sbxctl snapshot {name} [output_path]` | refuse if running + write user-owned snapshot tarball of current rootfs (see R8) |
| `sbxctl restore {name} {snapshot_path}` | refuse if running + wipe rootfs + extract user snapshot (see R9) |
| `sbxctl delete {name}` | refuse if running + remove rootfs, tarball, conf files, bridge, unit (see R10) |
| `sbxctl status` | thin wrapper over `machinectl list` filtered to `opqu-sbx-*` |

If an unrecognised subcommand is given, print a usage summary to stderr and
exit 1.

### Running check
Use `machinectl list --no-legend` and grep for `opqu-sbx-{name}` to
determine if a sandbox is running. Not found = stopped = not an error.
Any unexpected error from machinectl = log + exit 1.

### Output from subcommands
Commands pass subcommand output (stdout and stderr) through to the terminal
unmodified. Do not capture or summarize output from `machinectl`, `systemd-run`,
`mmdebstrap`, or `tar`. The script only emits its own messages for errors it
detects itself (wrong state, missing files, failed precondition checks).

### `sbxctl shell`
Shells in or runs a command as `SANDBOX_USER` via `sudo machinectl shell SANDBOX_USER@opqu-sbx-{name} [command...]`.
`SANDBOX_USER` is read from `conf/global.conf`; if the file does not exist or
the value is empty, it defaults to `$(whoami)` at runtime — the same defaulting
logic as all other commands.
If `sudo machinectl shell` fails for any reason (sandbox not running, user not found,
permission denied, etc.), its error output passes through directly and the
script exits with machinectl's exit code. No pre-check, no fallback to root.
To get a root shell: `sudo machinectl shell root@opqu-sbx-{name}`.
If additional arguments are provided after `{name}`, they are passed directly
to `machinectl shell` as the program and arguments to execute inside the sandbox.

### `sbxctl stop`
Stop is defined by the final state, not the transition. If the sandbox is
already stopped, `sbxctl stop` exits 0 silently — the desired state is already
reached. If the sandbox is running, `sudo machinectl poweroff` is invoked and its
output passes through unmodified.

### `sbxctl start`
A thin wrapper: conf and mounts are parsed, flags assembled, and
`sudo systemd-run` is invoked. No precondition checks (missing rootfs,
already running) are performed — errors from nspawn or systemd-run pass
through directly.

`sbxctl start` is intentionally not idempotent. If the sandbox is already
running, `systemd-run` will fail with its own error (unit name already
exists) and that error passes through unmodified. This is by design.

### `sbxctl status [name]`
If no `{name}` is provided, it performs a global health check and lists
running sandboxes:
1.  **Check Host Commands**: Verify that all required commands (see
    Prerequisites) are available in the PATH. Print each command followed by
    `OK` or `MISSING`. For `mmdebstrap`/`debootstrap`, it's acceptable if
    only one is present.
2.  **Check Networking**:
    - **systemd-networkd**: Check if the service is active via `systemctl`.
    - **IP Forwarding**: Check if `/proc/sys/net/ipv4/ip_forward` is `1`.
3.  **Check SANDBOX_USER**: Verify that the `SANDBOX_USER` (from
    `global.conf` or default) exists on the host. Print the result.
4.  **List Existing Rootfs**: List all directories in the `ROOT_DIR` that match
    the pattern `rootfs-*`.
5.  **List Running Sandboxes**: Runs `machinectl list` (with header) and
    filters output to lines matching `opqu-sbx-`, plus the header line. The
    footer is recomputed and printed as `"{N} machines listed."`.

If a `{name}` is provided, it shows the status of that specific sandbox:
1.  **Rootfs Existence**: Check if `rootfs-{name}/` exists.
2.  **Base Image**: Check if `rootfs-{name}.base.tar.zst` exists.
3.  **Running State**: Check if the sandbox is currently running via `machinectl`.
4.  **Port Mapping**: List the ports configured for forwarding in
    `conf/{name}.conf`.
5.  **Configuration**: List which configuration files (`.conf`, `.packages`,
    `.mounts`) exist for this sandbox.

---

## R6 — Reset

```bash
sbxctl reset {name}
```
1. Check if running; if yes, print:
   `"sandbox '{name}' is running; stop it first with 'sbxctl stop {name}'"` and exit 1
2. `sudo rm -rf "$ROOT_DIR/rootfs-{name}"`
3. `sudo tar --zstd -xf "$ROOT_DIR/rootfs-{name}.base.tar.zst" -C "$ROOT_DIR"`
4. Remove host network bridge if it lingers (best-effort, same as `sbxctl delete`):
   ```bash
   sudo ip link delete "vz-{zone_name}" 2>/dev/null || true
   ```
5. Remove a stuck transient systemd unit if it remains (best-effort, same as `sbxctl delete`):
   ```bash
   sudo systemctl reset-failed "opqu-sbx-{name}" 2>/dev/null || true
   ```

Steps 4 and 5 are best-effort: failures are silently ignored since the
network interface and unit may already be gone. They mirror the same cleanup
steps in `sbxctl delete` for consistency.

If the base tarball does not exist (e.g. manually deleted), step 3 will fail
with an error from `tar`. This is expected and acceptable; the user must
re-bootstrap from scratch in that case.

To fully re-bootstrap from scratch: run `sbxctl create {name}` again.
This requires the rootfs to not exist; remove it manually first if needed:
`sudo rm -rf sandboxes/rootfs-{name}/ sandboxes/rootfs-{name}.base.tar.zst`

---

## R7 — Audio (Optional, implement last)

- Opt-in in `conf/{name}.conf`: `AUDIO=yes`
- When `AUDIO=yes`, `sbxctl create` automatically adds `pipewire-pulse` to the
  package list passed to `--include`, merged with any packages from
  `{name}.packages`. The user does not need to list it manually.
- When enabled, add to nspawn invocation as `{audio flags}` (a separate placeholder
  from bind-mount and port flags — see the `systemd-run` template in R5):
  ```
  --bind=/run/user/$(id -u)/pipewire-0:/run/user/host/pipewire-0
  --setenv=PIPEWIRE_REMOTE=/run/user/host/pipewire-0
  ```
  `$(id -u)` is used here — not `$(whoami)` — because `/run/user/` paths are
  keyed by numeric UID, not username. `$(whoami)` would produce a name like
  `alice` which is not a valid XDG runtime dir path. This is a hard requirement
  of the XDG spec, not a style choice. `$(id -u)` is expanded by the shell
  running `sbxctl` (the operator) before `sudo systemd-run` is called, so the
  socket bound belongs to whoever runs `sbxctl start`. In the common case the
  operator and `SANDBOX_USER` are the same person, so audio works as expected.
  If they differ (e.g. `SANDBOX_USER=alice` but operator is `bob`), the
  socket bound will be Bob's, and audio will likely not work inside the
  container. There is no automatic fix for this; it is an accepted limitation.
- PulseAudio compat works via `pipewire-pulse` installed inside container
- Note: since UIDs match between host and container, the PipeWire socket
  permission issue from user namespace UID mismatch does not apply here
- Implement and test only after all other features are stable

---

## R8 — Snapshot (`sbxctl snapshot {name} [output_path]`)

Persists the current state of a sandbox rootfs as a user-managed backup
archive. This does not replace the built-in base tarball used by `reset`.

1. Check if running; if yes, print:
   `"sandbox '{name}' is running; stop it first with 'sbxctl stop {name}'"` and exit 1
2. Resolve the output path:
   - If `output_path` is given, use it exactly
   - Otherwise write to the caller's current working directory as:
     `./opqu-sbx-{name}.snapshot.tar.zst`
3. If the output path already exists, print a clear error telling the user to
   move it away first, then exit 1
4. Create a compressed tarball of `rootfs-{name}/` using Zstandard with a
   high-compression setting
5. If writing the snapshot succeeds but moving it into the final output path
   fails, keep the temporary snapshot file and print its path so the user can
   move or clean it up manually

Notes:
- The archive is user-managed, unlike `rootfs-{name}.base.tar.zst` which is
  root-managed
- Do not change ownership of files inside the archived rootfs; only the output
  archive file itself is expected to be owned by the invoking user
- The archive contains the `rootfs-{name}/` directory itself, not only its
  contents, so it can be extracted directly into `ROOT_DIR` during restore
- If the rootfs is missing or unreadable, let `tar` fail normally and pass
  its output through unmodified

---

## R9 — Restore (`sbxctl restore {name} {snapshot_path}`)

Restores a user-created snapshot over the live rootfs. This is distinct from
`reset`, which always restores the clean base image created by `sbxctl create`.

1. Check if running; if yes, print:
   `"sandbox '{name}' is running; stop it first with 'sbxctl stop {name}'"` and exit 1
2. Before deleting anything, verify that `snapshot_path` exists, is a regular
   file, and is readable. If not, print a clear error and exit 1
3. Before deleting anything, verify that the archive contains
   `rootfs-{name}/` as a top-level entry. If not, print a clear error and exit 1
4. `sudo rm -rf "$ROOT_DIR/rootfs-{name}"`
5. `sudo tar --zstd -xf "{snapshot_path}" -C "$ROOT_DIR"`

Notes:
- `snapshot_path` is required and positional
- Only basic path and top-level naming checks are performed before restore;
  deeper archive correctness remains the user's responsibility
- No extra automation is performed; if extraction fails, `tar`'s own error
  output passes through and the command exits non-zero
- Restoring a snapshot does not update or replace `rootfs-{name}.base.tar.zst`

---

## R10 — Delete (`sbxctl delete {name}`)

Permanently removes binary artifacts for a single sandbox while preserving
its configuration files.

1. Check if running; if yes, print:
   `"sandbox '{name}' is running; stop it first with 'sbxctl stop {name}'"` and exit 1
2. Remove the rootfs and base tarball:
   ```bash
   sudo rm -rf "$ROOT_DIR/rootfs-{name}"
   sudo rm -f  "$ROOT_DIR/rootfs-{name}.base.tar.zst"
   ```
3. List any found per-sandbox conf files in `conf/` and inform the user that
   they are being kept (not deleted).
4. Remove the host network bridge if it lingers:
   nspawn normally tears it down on stop, but if it remains:
   ```bash
   sudo ip link delete vz-{zone_name} 2>/dev/null || true
   ```
5. Remove a stuck transient systemd unit if it remains:
   ```bash
   sudo systemctl reset-failed "opqu-sbx-{name}" 2>/dev/null || true
   ```

- Steps 4 and 5 are best-effort: failures are silently ignored since the
  network interface and unit may already be gone.
- `pkg-cache/` and `conf/global.conf` are shared across sandboxes and are
  never removed by `sbxctl delete`.

---

## R11 — Complete Removal

To fully remove everything related to this system:

1. **Stop all running sandboxes**
   ```bash
   for name in $(machinectl list --no-legend | awk '{print $1}' | grep '^opqu-sbx-'); do
     sudo machinectl poweroff "$name"
   done
   ```

2. **Remove the sandboxes directory** (rootfs, tarballs, cache, configs, script)
   ```bash
   sudo rm -rf /path/to/sandboxes/
   ```

3. **Remove host network bridges if any linger after stop**
   nspawn normally tears down `vz-{zone_name}` bridges when a sandbox
   stops. If any remain:
   ```bash
   sudo ip link delete "vz-{zone_name}"
   ```
   All opqu-sandbox bridges are identifiable by the `vz-` prefix and their
   correspondence to sandbox names (see R3).

4. **Remove stuck transient systemd units if any remain**
   All opqu-sandbox units are identifiable by the `opqu-sbx-` prefix:
   ```bash
   sudo systemctl reset-failed 'opqu-sbx-*'
   ```

After step 2, nothing from this system remains on the host except transient
network and unit state that clears automatically on next reboot.

---

## Error Handling Convention

All commands follow this convention:
- Unexpected errors (failed subcommands, missing files, bad state) log a
  clear human-readable message to stderr and exit 1
- Expected negative states (sandbox not running, sandbox already stopped)
  are handled gracefully and do not error unless the outcome is wrong
- No command silently swallows errors or continues in a degraded state
- Output from subcommands (`machinectl`, `systemd-run`, `mmdebstrap`, `tar`)
  passes through unmodified; the script does not capture or summarize it

---

## Implementation Order

1. `sbxctl create` — existence check, SANDBOX_USER host check, pkg-cache mkdir, `mmdebstrap` (with cache + hooks) or `debootstrap` (fallback), tarball
2. `sbxctl start` — conf parsing, missing-file defaults, mount/port flag assembly, sudo systemd-run launch
3. `sbxctl stop` / `sbxctl shell` — machinectl wrappers; stop is idempotent, shell passes errors through
4. `sbxctl reset` — running check + wipe + re-extract
5. `sbxctl snapshot` / `sbxctl restore` — user-managed backup and recovery path
6. `sbxctl delete` — thin wrapper; removes rootfs, tarball, and conf files
7. `sbxctl status` — machinectl list filtered to `opqu-sbx-*`, footer recomputed
8. Audio support in `sbxctl start` — last, after core is verified working
