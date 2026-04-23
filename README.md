# opqu-sandbox

`opqu-sandbox` is a small Bash-based tool for creating and managing disposable Debian sandboxes with `mmdebstrap` and `systemd-nspawn`.

It is designed for local development and testing on a systemd-based Linux host. Each sandbox gets:

- its own Debian root filesystem
- isolated networking via `systemd-nspawn` network zones
- optional bind mounts into the container
- optional audio support via PipeWire (disabled by default)
- an easy reset path back to a clean base image
- snapshot and restore support for preserving sandbox state

The main entrypoint is [`sbxctl`](./sbxctl).

## Overview

`sbxctl` manages sandbox files relative to a root directory. By default that root is the current working directory, so you can keep the script, config, and sandbox data together in one project folder.

Sandbox names:

- must be 1 to 12 characters
- may contain lowercase letters, numbers, and `-`

## Requirements

Host environment:

- Debian trixie or another Linux host with systemd tooling available
- root access via `sudo`
- no whitespace in the sandbox root path when using `create`

Required host commands:

- `mmdebstrap`
- `systemd-nspawn`
- `systemd-run`
- `machinectl`
- `tar` with Zstandard support
- `zstd`
- `ip`
- `systemctl`

## Why It Uses Root

This project intentionally uses real root via `sudo` instead of user namespaces.

- setup stays simple and predictable: no subordinate UID/GID mapping, no extra namespace configuration, and no special host preparation beyond the required tools
- bind mounts are easier to use because the container user keeps the same UID as the host user, so file ownership lines up naturally
- the security model is explicit: this is a convenience sandbox for local work, not a hard multi-tenant isolation boundary

The tradeoff is deliberate: less setup friction and fewer surprises in exchange for requiring a trusted local machine and an operator who already has `sudo` access.

## Sandbox Root Layout

When you run `sbxctl`, it manages sandbox data under the selected root directory:

```text
ROOT/
├── conf/
│   ├── global.conf
│   ├── <name>.conf
│   ├── <name>.packages
│   └── <name>.mounts
├── pkg-cache/
├── rootfs-<name>/
└── rootfs-<name>.base.tar.zst
```

- `conf/` is user-managed
- `pkg-cache/`, `rootfs-*`, and `rootfs-*.base.tar.zst` are created and managed by the tool

## Getting Started

1. Install the required host packages.
2. Create a config directory:

```bash
mkdir -p conf
```

3. Optionally create `conf/global.conf`:

```bash
DISTRO=trixie
MIRROR=http://deb.debian.org/debian
VARIANT=standard
CONTAINER_USER=your-user
```

If `conf/global.conf` is omitted:

- `DISTRO=trixie`
- `MIRROR=http://deb.debian.org/debian`
- `VARIANT=standard`
- `CONTAINER_USER=$(whoami)`

4. See the built-in help for commands and examples:

```bash
./sbxctl --help
```

## Usage

Run the built-in help for the complete command list and argument syntax:

```bash
./sbxctl --help
```

You can also set a persistent sandbox root with:

```bash
export OPQU_SBX_ROOT=/path/to/sandboxes
```

`--root` takes precedence over `OPQU_SBX_ROOT`.

## Configuration

### `conf/<name>.conf`

Per-sandbox runtime settings:

```bash
PORTS='tcp:8080:8080 udp:5353:5353 tcp:5432:5432'
AUDIO=no
```

- `PORTS` forwards host ports into the sandbox; entries use `[protocol:]host_port[:container_port]`, protocol may be `tcp` or `udp`, omitting it defaults to `tcp`, and omitting `container_port` uses the same value as `host_port`
- `AUDIO=yes` enables PipeWire socket binding and adds `pipewire-pulse` at create time; audio is disabled by default (`AUDIO=no`). If you enable audio after the sandbox is created, you must manually install `pipewire-pulse` inside the sandbox using a root shell (e.g., `sudo machinectl shell root@opqu-sbx-<name>`).

### `conf/<name>.packages`

Extra packages to install during `create`, one per line:

```text
git
curl
ripgrep
```

Blank lines and `#` comments are ignored.

### `conf/<name>.mounts`

Bind mounts to apply on `start`:

```text
/path/to/project:/workspace
/path/to/reference-data:/data:ro
```

Format:

```text
host_path:container_path[:ro]
```

- omit `:ro` for read-write mounts
- stop and restart the sandbox after changing mounts

## Snapshots

`reset` restores the clean base image created by `create`. `restore` applies a user-created snapshot instead.

## Notes

- `start`, `shell`, and most lifecycle operations use `sudo` because the project does not use user namespaces
- the container user inside the sandbox is created with the same UID as the host user, which makes bind-mounted file ownership line up naturally
- `status` shows only currently running sandboxes
- logs for a running sandbox are available through `journalctl -M opqu-sbx-<name>`
