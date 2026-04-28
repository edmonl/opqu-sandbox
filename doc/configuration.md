# Configuration

## Root Directory

All sandbox paths for `sbxctl` are resolved relative to a sandbox [root directory](root-directory-structure.md).

The root directory is determined in the following order of precedence:

1. **CLI override**: `sbxctl --root /path/to/sandboxes ...`
2. **Environment override**: `OPQU_SBX_ROOT=/path/to/sandboxes`
3. **Default**: The current working directory.

## Configuration Files

All configuration files are optional and located in the `conf/` directory relative to the root directory. If a file does not exist, the system falls back to default values.

### `conf/default`

This file in dotenv format defines default settings for all sandboxes with the following default values:

```bash
DISTRO=trixie
MIRROR=http://deb.debian.org/debian
VARIANT=standard            # standard = full usable base; required = minimal
SANDBOX_USER=             # defaults to $(whoami) at runtime if left empty
RESOLV_CONF=auto            # for `--resolv-conf` of `systemd-nspawn`
```

An empty value means using the default.

### `conf/{name}.conf`

Per-sandbox runtime configuration in dotenv format. This file can also override default configuration defined in `conf/default` (e.g., `DISTRO`, `MIRROR`, `VARIANT`, `SANDBOX_USER`, `RESOLV_CONF`).

```bash
# Per-sandbox overrides (optional)
# DISTRO=trixie
# VARIANT=standard
# RESOLV_CONF=replace-uplink

# Sandbox-specific settings
PORTS="tcp:8080:8080 tcp:5432:5432"   # space-separated port mapping, each becomes a --port= flag of `systemd-nspawn`
AUDIO=no                              # yes to bind PipeWire socket
```

Default values: no port mapping, no audio.

Without port mapping, multiple sandboxes can run simultaneously without port collisions since each has its own IP on its own bridge.
However, if two sandboxes map the same host-side port, one will fail at start time when the other has been running.

### `conf/{name}.packages`

Per-sandbox extra packages to install at creation time, not used after the sandbox is created.

```
# Extra packages for this sandbox only, installed on top of $VARIANT at create time.
nodejs
postgresql
```

### `conf/{name}.mounts`

Per-sandbox runtime bind mounts, one entry per line.
This file is loaded at start time and builds flags dynamically.
To change mounts, you may edit this file, stop and restart the sandbox.

```
# host_path:sandbox_path[:ro]
/tmp/sandbox:/tmp
/home/user/data:/readonly-data:ro
~/folder-in-home-of-sandbox-user::ro # sandbox path is the same as the host path
/tmp
```

Each mount must have either a non-empty host path or a non-empty sandbox path.

About the host path:
- Relative path is resolved with the sandbox root directory.
- Empty path creates a scratch directory in `/var/tmp` on the host which is removed when the sandbox is stopped. This may be useful to inspect sandbox files when it is running.
- `~` at the beginning resolves to the home directory of the sandbox user.

About the sandbox path:
- Must be absolute.
- Treated as the same as the host path when omitted.

Lines starting with `#` and blank lines are ignored. File ownership stays the same across the mount boundary (see [System Requirements](system-requirements.md#user-requirement)).
