# Configuration

## Sandbox Directory

`sbx` manages files in a sandbox directory that can be configured in the following order of precedence:

1. **CLI override**: `sbx --sbx-directory /path/to/sandboxes ...`
2. **Environment override**: `OPQU_SBX_DIRECTORY=/path/to/sandboxes`
3. **Default**: The current working directory.

The directory layout is as follows:

```
{sandbox directory}/
├── pkg-cache/                                   # shared .deb cache (mmdebstrap only)
├── snapshots/                                   # sandbox snapshots
│   └── {name}/                                  # per-sandbox
│       ├── base.{timestamp}.tar.zst             # base snapshot
│       └── {snapshot name}.{timestamp}.tar.zst  # other snapshots
└── conf/                                        # user-managed configuration (optional)
    ├── default                                  # global defaults
    ├── {name}.conf                              # per-sandbox: ports
    ├── {name}.packages                          # per-sandbox extra packages
    └── {name}.mounts                            # per-sandbox bind mounts
```

The `conf/` directory is created and managed by the user when custom settings are needed to override the defaults.
Other files and directories are managed by `sbx`, though they may be manually managed or pruned (e.g., `rm pkg-cache/*.deb`) without breaking `sbx`.

### Constraints

- **Package Cache**: `sbx` prefers `mmdebstrap`. If `mmdebstrap` is not available, it falls back to `debootstrap`. The shared `pkg-cache/` is only supported when using `mmdebstrap`. When falling back to `debootstrap`, the cache is ignored and packages are downloaded directly into the sandbox.
- **Whitespace Constraint**: Because `sbx create` uses `mmdebstrap` `sync-in`/`sync-out` special hooks for `pkg-cache/`, the directory path *must not contain whitespace*. This constraint is enforced even if `debootstrap` is available, to ensure portability and consistent behavior.

## Configuration Files

All configuration files are optional and located in the `conf/` directory.

### `conf/default`

This file in dotenv format defines default settings for *creating* sandboxes with the following default values:

```bash
DISTRO=stable
MIRROR=http://deb.debian.org/debian
VARIANT=standard             # standard = full usable base; required = minimal
IMAGE_PATH=/var/lib/machines # search path for machine images, used by `machinectl`
SANDBOX_USER=                # defaults to the current user at runtime if left empty
RESOLV_CONF=auto             # for `--resolv-conf` of `systemd-nspawn`
ROOT_USER_PASSWORD=          # if empty, root password is disabled (locked)
NETWORK_ZONE=opqu-sbx        # logical network group; max 12 characters
MAX_SNAPSHOTS=10             # max numbers of snapshots to keep, excluding the base snapshot
```

An empty value indicates that the default should be used. See [User Model](user-model.md) for more about `ROOT_USER_PASSWORD`.

#### Network Zone Names

Each sandbox belongs to a *Network Zone*, which determines how it is grouped and connected on the host. 
The zone name is defined by `NETWORK_ZONE`. All sandboxes sharing the same zone name are connected to the same virtual bridge and can communicate with each other.
To comply with the 15-character Linux interface name limit, the zone name itself is limited to *12 characters*. `systemd-nspawn` automatically creates a host-side bridge prefixed with `vz-` for each zone. See [System Requirements](system-requirements.md#network-zones-and-bridges) for more details.

### `conf/{name}.conf`

Sandbox names are validated as follows:
- Must not be empty.
- Allowed characters: lowercase alphanumeric and hyphens.

Each `{name}.conf` file provides extra runtime configuration in dotenv format for the named sandbox, as well as optionally overriding the default configuration in `conf/default`.

```bash
# Per-sandbox overrides (optional)
# DISTRO=trixie
# VARIANT=standard
# RESOLV_CONF=replace-uplink

# Sandbox-specific settings
PORTS="tcp:8080:8080 udp:463"   # space-separated port mapping, no mapping by default
```

Each port mapping becomes a `--port=` flag for `systemd-nspawn`. The protocol defaults to `tcp` if omitted, and the sandbox port is assumed to be the same as the host port if omitted.
Without port mapping, multiple sandboxes can run simultaneously without port collisions since each has its own IP on its own bridge.
However, if two sandboxes map the same host-side port, one will fail at start time if the other is already running.

### `conf/{name}.packages`

Per-sandbox extra packages to install at creation time. These are not used after the sandbox is created.

```
# Extra packages for this sandbox only, installed on top of $VARIANT at create time.
nodejs
postgresql
```

### `conf/{name}.mounts`

Per-sandbox runtime bind mounts, one entry per line.
This file is loaded at start time and builds flags dynamically.
To change mounts, edit this file, then stop and restart the sandbox.

```
# host_path:sandbox_path[:ro]
/tmp/sandbox:/tmp
/home/user/data:/readonly-data:ro
~/folder-in-home-of-sandbox-user::ro # sandbox path is the same as the host path
/tmp
```

Each mount must have either a non-empty host path or a non-empty sandbox path.

About the host path:
- Relative paths are resolved in the sandbox directory.
- `systemd-nspawn` treats an empty host path as a scratch folder created in `/var/tmp` on the host and removed when the sandbox is stopped. This may be useful to inspect sandbox files when it is running.
- `~` at the beginning resolves to the home directory of the sandbox user.

About the sandbox path:
- Must be absolute.
- Treated as the same as the host path when omitted.

Lines starting with `#` and blank lines are ignored. File ownership stays the same across the mount boundary (see [System Requirements](system-requirements.md#user-requirement)).
