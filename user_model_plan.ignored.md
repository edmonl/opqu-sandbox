# User Model Plan

## Identities

Use these terms consistently:

1. `invoking user`: the non-root user who launched `sbx`. After privilege
   escalation, derive this from `SUDO_USER`. Before escalation, derive it from
   the current process user.

2. `sandbox user`: the configured `SANDBOX_USER`, represented by
   the host user mirrored inside the sandbox. For normal use, derive this from
   the invoking user. Use `SANDBOX_USER` only as the fallback identity when the
   command is already running as root and no invoking user can be derived.

3. `root`: the privileged executor used only for host operations that require
   elevated privileges.

## Enforcement

Make the invoking user the sandbox user for normal workflows. This removes the
ambiguous case where one host user creates files, snapshots, or cache entries
for a sandbox configured for another host user.

Keep `SANDBOX_USER`, but narrow its role. It is not an independent sandbox owner
for normal workflows. It exists to make direct-root workflows explicit, such as
`su -` or `root# sbx ...`, where `SUDO_USER` is unavailable.

Derive the effective user identity in this order:

1. If the process is non-root, use the current process user.

2. If the process is root and `SUDO_USER` is set, use `SUDO_USER`.

3. If the process is root, `SUDO_USER` is empty, and `SANDBOX_USER` is
   configured, use `SANDBOX_USER`.

4. If the process is root, `SUDO_USER` is empty, and `SANDBOX_USER` is not
   configured, fail with a clear error.

## Shared Helpers

Add shared helpers so commands do not open-code identity and ownership logic:

1. `InvokingUser() (*user.User, error)`

   Returns the non-root invoking user when one exists. Use `SUDO_USER` after
   sudo escalation and the current process user before escalation.

2. `EffectiveUser(conf *config.Config) (*user.User, error)`

   Returns the user identity that owns user-managed paths and is mirrored inside
   the sandbox. Use the derivation order above.

3. `ChownToEffectiveUser(path string, conf *config.Config) error`

   Changes ownership of generated user-owned outputs back to the effective user
   after root creates them.

## Ownership Rules

Use one explicit owner for each path category:

Before a command uses an existing user-managed path, verify that the path is
owned by the effective user. If ownership does not match, fail with a clear
error. Do not silently chown existing user-managed paths, because that can hide
cross-user mistakes and unexpectedly transfer ownership.

1. `conf/`: invoking-user owned.

   Configuration is user-managed input. For direct-root workflows, it belongs to
   the effective user from `SANDBOX_USER`.

2. `pkg-cache/`: invoking-user owned.

   Package cache reuse is a user-level convenience and should not become
   root-owned after provisioning.

3. `snapshots/`: invoking-user owned.

   Snapshot directories are user-owned output locations. A different user should
   fail instead of root creating or taking ownership of that path. For
   direct-root workflows, the effective user from `SANDBOX_USER` owns these
   paths.

4. `rootfs/{name}`: root-owned.

   The root filesystem is a system image managed through privileged operations.

5. `rootfs/{name}.nspawn`: root-owned.

   This is generated runtime configuration for the system container.

6. `IMAGES_PATH/{name}` and `NSPAWN_FILES_PATH/{name}.nspawn`: root-owned or
   system-managed symlinks.

   These live in system paths used by `machinectl` and `systemd-nspawn`.

## Command Behavior

1. `create`

   Resolve the effective user before provisioning and use it as the sandbox user.
   Create user-owned directories before sudo. Create the root-owned rootfs after
   sudo.

2. `snapshot`

   Create or verify the snapshot directory as the effective user before sudo.
   Create the archive as root, then chown the archive back to the effective
   user.

3. `restore`

   Require the snapshot archive to be readable by the effective user before sudo.
   Use root only to replace the rootfs.

4. `up`

   Resolve the effective user as the sandbox user before starting the sandbox.
   Use root for systemd and nspawn setup.

5. `down`

   Resolve the effective user before stopping the sandbox. Keep cleanup
   best-effort.

6. `delete`

   Resolve the effective user before deleting the sandbox. Use root for machine
   and rootfs cleanup, while preserving user-managed configuration.
