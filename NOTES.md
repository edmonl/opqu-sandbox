# Notes

- `Sudo` resolves the running binary with `os.Executable()`. Do not use `PATH`,
  which may find a different executable.
- `Extract` restores directory metadata after all entries are extracted. This
  preserves directory mtimes, fixes modes after umask or temporary write access,
  and avoids blocking child extraction with early ownership/mode changes. Do not
  switch to stack-based early restore unless archive traversal becomes strictly
  depth-first. Prefer standard iterator helpers such as `slices.Backward` when
  they clarify reverse traversal.
- `Compress` defers each source file close inside the `filepath.Walk` callback.
  That defer is scoped to one callback invocation, so each file closes after its
  entry is copied rather than staying open until the whole walk completes. Do
  not flag this as file descriptor accumulation.
- `sbx create` intentionally respects symlinked user-managed directories such
  as `rootfs`, `snapshots`, and `pkg-cache`; symlinks are a valid way to
  relocate them.
- `sbx snapshot` creates the snapshot directory before sudo. Snapshot paths are
  user-owned output locations, and a different user should fail instead of root
  creating or taking ownership of that path.
- `sbx restore` accepts a snapshot name only, not an arbitrary archive path. We
  considered falling back from a missing snapshot name to treating the argument
  as a path, but rejected that because restoring arbitrary paths weakens the
  ownership boundary and complicates sudo timing. We also considered resolving
  before sudo and passing rewritten or hidden arguments into the elevated
  process to avoid duplicate prompts, but rejected that because we do not know
  exactly which original CLI argument should be replaced once flags, aliases, or
  future options are involved. Confirmation prompts tied to restore-specific
  resolution must happen after sudo to avoid asking once before re-exec and
  again in the elevated process; as a consequence, restore does not check the
  snapshot archive file before sudo. Resolve the name inside
  `snapshots/{sandbox}/` after sudo and require exactly one matching regular
  archive. Missing, duplicate, and non-regular matches are errors that the user
  should fix manually. Do not auto-select the latest duplicate; duplicates mean
  the snapshot store invariant has already been broken.
- `sbx restore` uses `.bak` only as an operation-local temporary backup. An
  existing `.bak` is stale and may be deleted before extracting the new archive.
- `getSetupScript` is for commands that must run inside the chroot. Write static
  rootfs files such as `/etc/hostname` and `/etc/hosts` from Go.
- Sandbox user creation mirrors only the host user's UID and primary GID.
  Supplementary groups are intentionally omitted.
- UID and GID values from `os/user` are used directly in shell commands. Add
  validation only if a caller starts accepting untrusted numeric IDs.
- Sandbox primary groups are created in a fresh rootfs with the sandbox username
  as the group name. Do not add `getent` checks unless creation stops targeting a
  clean filesystem.
- Tests may use Go's system temporary directory through `t.TempDir()`, and may
  use stable device paths such as `/dev/full` or `/dev/null` for behavior that
  depends on those devices. The important invariant is cleanup and isolation:
  test-created files and directories must be removed after the test, and
  temporary state from one test must not block another test.
