# Issues

## Major Issues

## Minor Issues

## Ignored Issues

1. `RequireInactiveRootfs` could wrap `HasMounts` errors with operation-specific
   context.

   This would improve diagnostics, but the raw error preserves the failure and
   does not affect behavior.

2. `loadLines` deduplicates via a map, so package and mount line order is not
   stable.

   This is acceptable because apt handles package dependencies, and mount order is
   not currently meaningful.

3. Port mappings should validate numeric ranges.

   `PORTS` shape is validated, but values outside `1..65535` are left for
   `systemd-nspawn` to reject at startup.

4. `sbx delete` runs `machinectl remove` before removing the image symlink.

   Removing the symlink first would likely make the image undiscoverable. The
   current flow verifies the image symlink target before calling `machinectl`, so
   any resolved rootfs removal stays within the sandbox-managed path.

5. `sbx delete` does not warn when the nspawn symlink is missing.

   Missing runtime nspawn symlinks are acceptable: `sbx up` may not have
   completed, `sbx down` may have removed them, or a user may have cleaned them.
   Suspicious existing symlinks still warn and are left untouched.

6. `CreateSnapshot` should validate `snapshotName` internally.

   The CLI validates snapshot names, but `CreateSnapshot` builds glob and output
   paths directly from `snapshotName`. A future internal caller could use path
   separators to write or clean up outside `snapshotsDir`.

7. `sbx up` could reject active mounts before starting a stopped sandbox.

   A stopped sandbox can still have stale mounts under its rootfs. Startup does
   not currently call `RequireInactiveRootfs`, but `systemd-nspawn` is the
   component that consumes the rootfs at startup and should fail or surface mount
   conflicts. Snapshot/delete/restore remain the higher-risk paths because they
   walk, remove, or replace rootfs contents.

8. `createNspawnFile` does not use `O_NOFOLLOW` when writing the nspawn file.

   `createNspawnFile` rejects symlink paths with `Lstat` before writing. A
   time-of-check/time-of-use symlink swap between that check and `WriteFile` is
   theoretically possible, but this is not worth the extra low-level file-open
   complexity for the current local root-run workflow.

9. `sbx create` help does not explicitly say base snapshot creation is
   best-effort.

   The command warns when base snapshot creation fails, so runtime behavior makes
   the best-effort nature visible. The short help remains concise.
