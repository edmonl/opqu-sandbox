# Uninstall

Use `sbx status` to check sandboxes. Use `sbx stop {name}` to stop running sandboxes. Use `sbx delete {name}` to delete sandboxes.
Then the whole sandbox directory and the single executable binary may be deleted.
Remove the export of `OPQU_SBX_ROOT` if you persisted it before.

You may also delete the whole sandbox directory and the executable binary directly, then use `machinectl` to manage the running containers, the images, and relevant systemd units. There also may be `.nspawn` files left in `/var/lib/machines`.

Virtual network bridges are created on the host to implement *Network Zones*. `ip link show` can list them, whose names are prefixed with `vz-` (e.g., `vz-opqu-sbx`). See [System Requirements](system-requirements.md#network-zones-and-bridges) for details.
