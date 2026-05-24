# Notes

- `Sudo` resolves the binary with `os.Executable()`. Do not fall back to `PATH` lookup, because it may resolve to a different executable than the running process.
- `Extract` defers directory metadata restoration until after all archive entries are extracted. This keeps directory mtimes correct after child creation, restores modes after umask and temporary write access, and avoids ownership/mode changes blocking later child extraction. A stack-based early restore would only be correct for depth-first archives; the current extractor only requires parent directories to appear before children, so a later entry can still target a directory that would already have been popped. The project targets a Go version with standard library iterator helpers such as `slices.Backward`; prefer them for reverse slice traversal when they make the loop clearer.
- `sbx create` intentionally respects symlinked user-managed directories such
  as `rootfs`, `snapshots`, and `pkg-cache`. A symlink-rejecting replacement for
  `os.MkdirAll` was considered, but rejected because these directories belong
  to the user's sandbox layout and symlinks are a valid way to relocate them.
- `getSetupScript` focuses on commands that must run inside the chroot. Static rootfs files such as `/etc/hostname` and `/etc/hosts` can be written from Go to reduce shell quoting.
- Sandbox user creation only mirrors the host user's UID and primary GID. Supplementary groups were considered but rejected as unnecessary complexity for the sandbox user model.
- UID and GID from `os/user` system lookups are used directly in shell commands. Do not add vague defensive quoting or validation for those numeric fields unless a concrete caller starts accepting untrusted values.
- Sandbox primary groups are created directly in the fresh rootfs with the sandbox username as the group name. Do not add `getent` checks for this fresh rootfs path unless creation stops targeting a clean filesystem.
