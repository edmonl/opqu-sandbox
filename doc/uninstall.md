# Uninstall

Use `sbxctl status` to check running sandboxes. Use `sbxctl stop {name}` to stop running sandboxes.
Then the whole sandbox root directory and the single executable binary may be deleted.
Remove the export of `OPQU_SBX_ROOT` if you persisted it before.

You may also delete an individual sandbox's `rootfs/{name}` and its base image `rootfs/{name}.base.tar.zst` if you do not want to delete the entire root directory.

Note that files in the `conf/` directory (like `{name}.conf`, `{name}.packages`, and `{name}.mounts`) are not deleted by `sbxctl delete {name}`, so you can easily recreate the sandbox later. Delete them manually when they are not needed any more.

## Underlying system resources

`machinectl list` may be used to show the underlying containers whose names are prefixed with `opqu-sbx-`.
`machinectl poweroff opqu-sbx-{name}` can stop the running container. Other `machinectl` commands should work accordingly.

Virtual network bridges are created on the host to implement *Network Zones*. `ip link show` can list them, whose names are prefixed with `vz-` (e.g., `vz-opqu-sbx`). See [System Requirements](system-requirements.md#network-zones-and-bridges) for details on their lifecycle.

Transient systemd units are used to keep the sandboxes running.

In rare cases where any virtual bridges or transient systemd units remain after all sandboxes in a zone have stopped, they should clear automatically after reboot. In case reboot is not wanted, `ip link delete "vz-{zone_name}"` may be used to delete the bridges, and `systemctl reset-failed 'opqu-sbx-*'` may be used to clear the transient units.
