# Root Directory Structure

The root directory stores everything `sbxctl` creates and manages. Deleting it may require `sudo` and gives you a clean slate. You may move it freely with according [configuration](configuration.md#root-directory).

## Layout

```
{root}/
├── pkg-cache/                    # shared .deb cache (auto-created, mmdebstrap only)
├── rootfs-{name}/                # per-sandbox live rootfs (auto-created)
├── rootfs-{name}.base.tar.zst    # clean-slate snapshot (auto-created)
└── conf/                         # user-managed configuration (optional)
    ├── global.conf               # global defaults
    ├── {name}.conf               # per-sandbox: ports, audio flag
    ├── {name}.packages           # per-sandbox extra packages
    └── {name}.mounts             # per-sandbox bind mounts
```

## Lifecycle and Ownership

- **Auto-created artifacts**: `pkg-cache/`, `rootfs-{name}/`, and `rootfs-{name}.base.tar.zst` are created and managed by `sbxctl`. They are root-managed (require `sudo` for modification/deletion).
- **User-managed configuration**: The `conf/` directory and all files within it are **entirely optional** and are **never automatically created** by `sbxctl`. They must be created manually by the user if custom settings are needed. If they are missing, `sbxctl` uses built-in defaults.

## Constraints

- **Package Cache**: `sbxctl` prefers `mmdebstrap` for creating root filesystems. If `mmdebstrap` is not available, it falls back to `debootstrap`. The shared `pkg-cache/` is only supported when using `mmdebstrap`. When falling back to `debootstrap`, the cache is ignored and packages are downloaded directly into the sandbox.
- **Whitespace Constraint**: Because `sbxctl create` uses `mmdebstrap` `sync-in`/`sync-out` special hooks for `pkg-cache/`, the root directory path **must not contain whitespace**. This constraint is enforced even if `debootstrap` is available, to ensure portability and consistent behavior across environments.
