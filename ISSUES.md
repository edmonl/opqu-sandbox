# Issues

## Major Issues

1. Archive extraction does not restore regular file modes after creation.

   `Extract` creates regular files with the archived mode, but does not call
   `chmod` after writing them. The process umask can silently strip permission
   bits, making restored snapshots differ from the original rootfs.

## Minor Issues

1. Snapshot creation can overwrite the previous archive when repeated within the same second.

   `CreateSnapshot` writes directly to `{snapshot name}.{timestamp}.tar.zst`
   using second-level timestamp precision. If a snapshot with the same name is
   recreated in the same second and compression fails, the old archive may be
   truncated and then removed.

2. `IsRunning` treats any `machinectl show` exit status as not running.

   A missing machine should be treated as stopped, but other non-zero failures
   such as DBus, polkit, or systemd errors should be reported instead of letting
   callers proceed on an unknown state.

3. `sbx shell` does not use the standard sudo escalation flow.

   On hosts where `machinectl shell` requires root, normal users get a raw
   command failure instead of the confirmation and re-exec behavior used by
   lifecycle commands.

## Ignored Issues

1. `RequireInactiveRootfs` could wrap `HasMounts` errors with operation-specific context. Too trivial for rare cases.

2. `loadLines` deduplicates via a map, so package and mount line order is not stable. Acceptable because apt handles package dependencies, and mount order is
   not currently meaningful.

3. Port mappings should validate numeric ranges. This is left for `systemd-nspawn` to reject at startup.

4. `sbx delete` runs `machinectl remove` before removing the image symlink. Removing the symlink first would likely make the image undiscoverable.

5. `sbx delete` does not warn when the nspawn symlink is missing. It is acceptable.

6. `CreateSnapshot` should validate `snapshotName` internally. Too trivial for something not currently happening.

7. `sbx up` could reject active mounts before starting a stopped sandbox. The case is rare, and of low risk because `systemd-nspawn` can catch and surface mount
   conflicts.

8. `createNspawnFile` does not use `O_NOFOLLOW` when writing the nspawn file. It just handles a rare theoretically possible case but not worth the extra low-level file-open complexity for the current local root-run workflow.

9. `sbx create` help does not explicitly say base snapshot creation is best-effort. Runtime behavior makes the best-effort nature visible by warning. Help should remain concise.
