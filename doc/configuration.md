# Configuration

Each sandbox is identified by its name (`{name}`).

## Sandbox Directory

`sbx` manages files in a sandbox directory that can be configured in the following order of precedence:

1. **CLI override**: `sbx --sbx-dir /path/to/sandboxes ...`
2. **Environment override**: `OPQU_SBX_DIRECTORY=/path/to/sandboxes`
3. **Default**: The current working directory.

The directory layout is as follows:

```
{sandbox directory}/
├── pkg-cache/                                   # shared .deb cache (mmdebstrap only)
├── rootfs/                                      # rootfs parent, created by sbx
│   ├── {name}/                                  # per-sandbox root filesystem (owned by root)
│   └── {name}.nspawn                            # systemd-nspawn config (owned by root)
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

If the sandbox directory does not exist, `sbx` creates that directory only. Its parent directory must already exist.

The `conf/` directory is created and managed by the user when custom settings are needed to override defaults.
Other files and directories are managed by `sbx`, but can be manually modified or pruned (e.g., `rm pkg-cache/*.deb`) without breaking `sbx`.

### Constraints

- **Package Cache**: `sbx` prefers `mmdebstrap`. If `mmdebstrap` is unavailable, it falls back to `debootstrap`. The shared `pkg-cache/` is only supported when using `mmdebstrap`. During fallback to `debootstrap`, the cache is ignored, and packages are downloaded directly into the sandbox.
- **Whitespace Constraint**: Because `sbx create` uses `mmdebstrap` `sync-in`/`sync-out` special hooks for `pkg-cache/`, the directory path *must not contain whitespace*. This constraint is enforced even if `debootstrap` is available to ensure portability and consistent behavior.

## Configuration Files

All configuration files are optional and located in the `conf/` directory.

### `conf/default`

This dotenv-formatted file defines default settings for sandbox creation:

```bash
# Global only; not supported in per-sandbox configuration:
IMAGES_PATH=/var/lib/machines         # search path for machine images
NSPAWN_FILES_PATH=/etc/systemd/nspawn # search path for nspawn files

# The following may be overridden by each sandbox's configuration:
DISTRO=stable
MIRROR=http://deb.debian.org/debian
VARIANT=standard             # standard = full usable base; required = minimal
RESOLV_CONF=auto             # for `--resolv-conf` of `systemd-nspawn`
ROOT_USER_PASSWORD=          # if empty, root password is disabled (locked)
NETWORK_ZONE=opqu-sbx        # logical network group; max 12 characters
```

An empty value indicates that the default should be used. See [User Model](user-model.md) for more details about sandbox users and `ROOT_USER_PASSWORD`.

`IMAGES_PATH` is the directory where `machinectl` searches for images (refer to the `machinectl` man page on Debian). `sbx` creates a symlink in `IMAGES_PATH` pointing to `{sandbox directory}/rootfs/{name}` for each sandbox `{name}`.

`NSPAWN_FILES_PATH` is the directory containing runtime configurations for local containers, used by `systemd-nspawn` (refer to the `systemd.nspawn` man page on Debian). `sbx` creates a symlink in `NSPAWN_FILES_PATH` pointing to `{sandbox directory}/rootfs/{name}.nspawn` for each sandbox `{name}`.

#### Network Zone Names

Each sandbox belongs to a *Network Zone*, which determines how it is grouped and connected on the host. 
The zone name is defined by `NETWORK_ZONE`. All sandboxes sharing the same zone name connect to the same virtual bridge and can communicate with each other.
To comply with the 15-character Linux network interface name limit, the zone name is restricted to *12 characters*. `systemd-nspawn` automatically creates a host-side bridge prefixed with `vz-` for each zone. See [System Requirements](system-requirements.md#network-zones-and-bridges) for more details.

### `conf/{name}.conf`

Sandbox names are validated as follows:
- Must not be empty.
- Allowed characters: lowercase alphanumeric and hyphens.

Each `{name}.conf` file provides optional runtime configuration in dotenv format for a specific sandbox, overriding settings in `conf/default`.

```bash
# Per-sandbox overrides (optional)
# DISTRO=trixie
# VARIANT=standard
# RESOLV_CONF=replace-uplink

# Sandbox-specific settings
PORTS="tcp:8080:8080 udp:463"   # space-separated port mapping, no mapping by default
```

Each port mapping is passed as a `--port=` flag to `systemd-nspawn`. If omitted, the protocol defaults to `tcp` and the sandbox port defaults to the host port.
Multiple sandboxes can run simultaneously without port collisions, as each has its own IP address on the bridge.
However, mapping the same host port to multiple running sandboxes will result in a startup failure for subsequent sandboxes.

### `conf/{name}.packages`

Lists extra packages to install during sandbox creation. This file is ignored after creation.

```
# Extra packages for this sandbox only, installed on top of $VARIANT at create time.
nodejs
postgresql
```

### `conf/{name}.mounts`

Defines runtime bind mounts for the sandbox, with one entry per line.
This file is evaluated at startup to dynamically generate mount flags.
To apply changes, edit this file and restart the sandbox.

```
# host_path:sandbox_path[:ro]
/tmp/sandbox:/tmp
/home/user/data:/readonly-data:ro
~/folder-in-home-of-sandbox-user::ro # sandbox path is the same as the host path
/tmp
```

Each mount must have either a non-empty host path or a non-empty sandbox path.

Host path rules:
- Relative paths are resolved in the sandbox directory.
- `systemd-nspawn` treats an empty host path as a scratch folder created in `/var/tmp` on the host and removed when the sandbox stops. This is useful for inspecting running sandbox files.
- `~` at the beginning resolves to the sandbox user's home directory.

Sandbox path rules:
- Must be absolute.
- Treated as the same as the host path when omitted.

Lines starting with `#` and blank lines are ignored. File ownership stays consistent across the mount boundary (see [System Requirements](system-requirements.md#user-requirement)).
