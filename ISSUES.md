# Issues

## Major Issues

1. Snapshots should reject active mounts before archiving a rootfs.

   `CreateSnapshot` calls `Compress` on the rootfs without checking `HasMounts`.
   If a stopped sandbox still has a bind mount or other active mount under its
   rootfs, snapshot creation can archive host-mounted content into the snapshot.
   This should be guarded before walking the rootfs, similar to restore/delete
   mount checks.

2. `CreateSnapshot` should validate `snapshotName` internally.

   The CLI validates snapshot names, but `CreateSnapshot` itself builds glob and
   output paths directly from `snapshotName`. An unsafe future internal caller
   could use path separators to write outside `snapshotsDir` or make the old
   snapshot cleanup glob match paths outside the intended snapshot directory.

## Minor Issues

1. `Extract` should return regular-file close errors.

   `internal/sandbox/archive.go` closes restored regular files without checking
   the returned error. File close can report delayed write failures, so
   extraction should return that error with path context.

2. Directory metadata is not fully restored by `Extract`.

   Directory creation uses the mode from the archive before children are
   extracted. The process umask can still affect permissions, and later child
   creation can update directory mtimes after `os.Chtimes` has already run for
   the directory entry. Directory modes and timestamps should be restored after
   all entries are extracted.

3. `Extract` reports unsupported tar entry types indirectly.

   Unsupported tar type flags fall through the switch without creating `target`,
   then ownership restoration fails with a less useful error. Return an explicit
   unsupported-entry-type error instead.

## Ignored Issues

1. `requireInactiveRootfs` could wrap `HasMounts` errors with restore-specific
   context.

   This would improve diagnostics, but the current raw error still preserves the
   underlying failure and does not affect behavior.

2. `loadLines` deduplicates via a map, so package and mount line order is not
   stable.

   This is acceptable for package lists because apt handles dependencies. Mount
   ordering is not currently treated as meaningful by the configuration model.

3. Port mappings should validate numeric ranges.

   `internal/config/config.go` validates the shape of `PORTS`, but it does not
   reject ports outside `1..65535`. Invalid values are deferred to
   `systemd-nspawn` startup errors instead of being caught during configuration
   loading.
