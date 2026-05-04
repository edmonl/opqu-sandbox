# Uninstall

Use `sbx status` to check sandboxes. Use `sbx stop {name}` to stop running sandboxes. Use `sbx delete {name}` to delete sandboxes.
Afterward, the entire sandbox directory and the single executable binary may be deleted.
Remove the export of `OPQU_SBX_ROOT` if you persisted it previously.

Alternatively, you may delete the entire sandbox directory and the executable binary directly, then use `machinectl` to manage the running containers, their images, and relevant systemd units. There may also be `.nspawn` files left in `/var/lib/machines`.

Virtual network bridges are created on the host to implement *Network Zones*. You can use `ip link show` to list them. Their names are prefixed with `vz-` (e.g., `vz-opqu-sbx`). See [System Requirements](system-requirements.md#network-zones-and-bridges) for details.
