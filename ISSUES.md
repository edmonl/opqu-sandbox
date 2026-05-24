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

1. Write a `Warn` function in `util` to issue warnings.

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
