# Issues

## Major Issues

1. User and ownership model needs to be made consistent.

   The project currently has at least three relevant identities: the invoking
   user, the configured sandbox user, and root after privilege escalation.
   Snapshot directories are intended to be invoking-user owned, while snapshot
   archives are chowned back from root via `SUDO_USER` when available. Other
   sandbox directories and files may follow different ownership paths. We should
   decide whether the invoking user must match the sandbox user and then make
   directory/file ownership rules explicit and consistently enforced.

## Minor Issues

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
