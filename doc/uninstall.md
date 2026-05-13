# Uninstall

To clean up, first use `sbx status` to check for existing sandboxes. Stop running sandboxes with `sbx stop {name}`, and remove them using `sbx delete {name}`.

Alternatively, you can manually stop containers and delete their images and `systemd` units using `machinectl`. Note that `.nspawn` files in `/etc/systemd/nspawn/` must be removed manually.

Virtual network bridges are generally removed automatically by the steps above, but you can verify and delete them manually using `networkctl` and `ip` commands.

Once all sandboxes are deleted, you can safely remove the sandbox directory and the `sbx` executable.
Finally, remove `OPQU_SBX_DIRECTORY` from your environment or shell profile if it was previously exported.
