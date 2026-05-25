# Notes

- `Sudo` resolves the running binary with `os.Executable()`. Do not use `PATH`,
  which may find a different executable.
- `Extract` restores directory metadata after all entries are extracted. This
  preserves directory mtimes, fixes modes after umask or temporary write access,
  and avoids blocking child extraction with early ownership/mode changes. Do not
  switch to stack-based early restore unless archive traversal becomes strictly
  depth-first. Prefer standard iterator helpers such as `slices.Backward` when
  they clarify reverse traversal.
- `sbx create` intentionally respects symlinked user-managed directories such
  as `rootfs`, `snapshots`, and `pkg-cache`; symlinks are a valid way to
  relocate them.
- `sbx snapshot` creates the snapshot directory before sudo. Snapshot paths are
  user-owned output locations, and a different user should fail instead of root
  creating or taking ownership of that path.
- `getSetupScript` is for commands that must run inside the chroot. Write static
  rootfs files such as `/etc/hostname` and `/etc/hosts` from Go.
- Sandbox user creation mirrors only the host user's UID and primary GID.
  Supplementary groups are intentionally omitted.
- UID and GID values from `os/user` are used directly in shell commands. Add
  validation only if a caller starts accepting untrusted numeric IDs.
- Sandbox primary groups are created in a fresh rootfs with the sandbox username
  as the group name. Do not add `getent` checks unless creation stops targeting a
  clean filesystem.
