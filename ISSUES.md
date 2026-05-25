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
