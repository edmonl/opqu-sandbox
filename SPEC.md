### `sbxctl shell`
Shells in or runs a command as `SANDBOX_USER` via `sudo machinectl shell SANDBOX_USER@opqu-sbx-{name} [command...]`.
`SANDBOX_USER` is read from `conf/default`; if the file does not exist or
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
1. **Check Host Commands**: Verify that all required commands (see
    Prerequisites) are available in the PATH. Print each command followed by
    `OK` or `MISSING`. For `mmdebstrap`/`debootstrap`, it's acceptable if
    only one is present.
2. **Check Networking**:
    - **systemd-networkd**: Check if the service is active via `systemctl`.
    - **IP Forwarding**: Check if `/proc/sys/net/ipv4/ip_forward` is `1`.
3. **Check SANDBOX_USER**: Verify that the `SANDBOX_USER` (from
    `default` or default) exists on the host. Print the result.
4. **List Existing Rootfs**: List all directories in the `ROOT_DIR/rootfs/` subdirectory.
5. **List Running Sandboxes**: Runs `machinectl list` (with header) and
    filters output to lines matching `opqu-sbx-`, plus the header line. The
    footer is recomputed and printed as `"{N} machines listed."`.

If a `{name}` is provided, it shows the status of that specific sandbox:
1. **Rootfs Existence**: Check if `rootfs/{name}/` exists.
2. **Base Image**: Check if `rootfs/{name}.base.tar.zst` exists.
3. **Running State**: Check if the sandbox is currently running via `machinectl`.
4. **Port Mapping**: List the ports configured for forwarding in
    `conf/{name}.conf`.
5. **Configuration**: List which configuration files (`.conf`, `.packages`,
    `.mounts`) exist for this sandbox.

---

## R6 — Reset

```bash
sbxctl reset {name}
```
1. Check if running; if yes, print:
   `"sandbox '{name}' is running; stop it first with 'sbxctl stop {name}'"` and exit 1
2. `sudo rm -rf "$ROOT_DIR/rootfs/{name}"`
3. `sudo tar --zstd -xf "$ROOT_DIR/rootfs/{name}.base.tar.zst" -C "$ROOT_DIR/rootfs"`
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
steps in `doc/uninstall.md` for consistency.

If the base tarball does not exist (e.g. manually deleted), step 3 will fail with an error. This is expected and acceptable; the user must re-bootstrap from scratch in that case.

To fully re-bootstrap from scratch: run `sbxctl create {name}` again.
This requires the rootfs to not exist; remove it manually first if needed:
`sudo rm -rf sandboxes/rootfs/{name}/ sandboxes/rootfs/{name}.base.tar.zst`

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
4. Create a compressed tarball of `rootfs/{name}/` using Zstandard with a
   high-compression setting
5. If writing the snapshot succeeds but moving it into the final output path
   fails, keep the temporary snapshot file and print its path so the user can
   move or clean it up manually

Notes:
- The archive is user-managed, unlike `rootfs/{name}.base.tar.zst` which is
  root-managed
- Do not change ownership of files inside the archived rootfs; only the output
  archive file itself is owned by the invoking user
- The archive contains only the *contents* of the rootfs, not the `{name}/`
  directory itself, so it can be extracted directly into `ROOT_DIR/rootfs/{name}` during restore
- This ensures snapshots are name-independent; a snapshot of `sbx1` can be restored into `sbx2`.
- If the rootfs is missing or unreadable, let the internal archiver report the error normally.

---

## R9 — Restore (`sbxctl restore {name} {snapshot_path}`)

Restores a user-created snapshot over the live rootfs. This is distinct from
`reset`, which always restores the clean base image created by `sbxctl create`.

1. Check if running; if yes, print:
   `"sandbox '{name}' is running; stop it first with 'sbxctl stop {name}'"` and exit 1
2. Before deleting anything, verify that `snapshot_path` exists, is a regular
   file, and is readable. If not, print a clear error and exit 1
3. Wipe the current `rootfs/{name}/`
4. Extract the snapshot archive contents directly into `rootfs/{name}/`

Notes:
- `snapshot_path` is required and positional
- Only basic path checks are performed before restore;
  deeper archive correctness remains the user's responsibility
- No extra automation is performed; if extraction fails, the internal extractor's error output passes through and the command exits non-zero
- Restoring a snapshot does not update or replace `rootfs/{name}.base.tar.zst`

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
