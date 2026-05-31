# System Requirements

`sbx` is intended to be used interactively.
You can use `sbx status` to verify the system requirements.

## Host Commands

The following host commands are used by `sbx`:
- `mmdebstrap`: Used to provision the rootfs.
- `debootstrap`: Used as a fallback if `mmdebstrap` is unavailable.
- `systemd-nspawn`: The core container engine used to run the sandboxes.
- `systemd-run`: Used to run sandbox processes as `systemd` background services.
- `machinectl`: Used to manage container lifecycle and provide shell access.
- `systemctl`: Used to manage `systemd` units.
- `sudo`: Used when privileges are required and `sbx` is not started with `sudo`.
- `su`: Used as a fallback when `sudo` is unavailable.

Optional administration commands:
- `networkctl`: Check the status of virtual interfaces.
- `ip`: Manage network bridges and interfaces.
- `sysctl`: Configure and verify IP forwarding.
- `journalctl`: Read logs using `journalctl --machine {name}`.

On Debian, the following *optional* packages provide some of these commands:
- `mmdebstrap`: provides `mmdebstrap`.
- `debootstrap`: provides `debootstrap`.
- `systemd-container`: provides `systemd-nspawn` and `machinectl`.
- `sudo`: provides `sudo`.

A standard Debian installation typically includes these required packages:
- `systemd` (important): provides `systemd-run`, `systemctl`, `networkctl`, and `systemd-networkd`.
- `iproute2` (important): provides `ip`.
- `procps` (important): provides `sysctl`.
- `util-linux` (required): provides `su`.

## System Paths Accessed

`sbx` directly reads the following system paths on the host to gather status and ensure safe operations:
- `/proc/sys/net/ipv4/ip_forward`: Read to verify if the Linux kernel is configured to allow packet forwarding (required for sandbox internet access).
- `/proc/self/mountinfo`: Read before deleting a sandbox to ensure its root filesystem does not contain any active system bind mounts.

### Networking Requirements

For sandboxes to have outbound internet access, the host system must be configured to handle virtual routing, DHCP, and firewall traversal.

#### Network Zones and Bridges

This tool uses the concept of *Network Zones* to group sandboxes together.
- A *Network Zone* is a logical group. All sandboxes assigned to the same zone (via the `NETWORK_ZONE` setting) can communicate with each other.
- A *Bridge* is the underlying virtual switch created on the host to implement the zone. 

**Naming and Lifecycle:**
1. Each zone results in a host bridge named `vz-{zone_name}`.
2. `systemd-nspawn` automatically creates the bridge when the *first* sandbox in that zone starts.
3. `systemd-nspawn` automatically removes the bridge when the *last* sandbox in that zone stops.
4. Because the bridge is created dynamically, a DHCP server (e.g., `systemd-networkd`) must be configured on the host to serve `vz-*` interfaces as they appear.

#### Network Management

The virtual bridges (prefixed with `vz-`) require `systemd-networkd` on the host to provide DHCP leases and perform NAT (Network Address Translation).

**Configuration Requirement:**
For the bridges to be managed automatically, a `systemd-networkd` configuration file (e.g., `/usr/lib/systemd/network/80-container-vz.network` or similar) must exist on the host to match `vz-*` interfaces.
- On Debian, this file is provided automatically by `systemd`.
- You can verify its presence by looking for a file containing `[Match] Name=vz-*` in `/usr/lib/systemd/network/` (refer to the `systemd.network` man page for alternative locations).

**Verification:**
You can use the `networkctl` command to verify the status of the virtual interfaces:
- `networkctl list`: Lists all interfaces and their setup status (should show `managed` for `vz-*` interfaces when sandboxes are running).
- `networkctl status vz-*`: Shows detailed information, including assigned IP addresses and DHCP server status.

You can check if the service is active with `systemctl status systemd-networkd`, or enable and start it using `systemctl enable --now systemd-networkd`.

#### IP Forwarding
For the host to act as a gateway for the sandboxes, the Linux kernel must allow packet forwarding between interfaces.
To verify, ensure `cat /proc/sys/net/ipv4/ip_forward` outputs `1`.
See the man pages for `sysctl.d`, `sysctl.conf`, or `sysctl` to change this setting or persist it using `net.ipv4.ip_forward=1`.

#### Firewall Configuration

The host firewall is not managed by `sbx`.
Ensure your firewall does not block virtual network traffic by verifying the following rules:
1. **Input**: Allow traffic from the virtual interfaces (`vz-*` and `vb-*`) to the host. This is required for the sandbox to request a DHCP IP address and reach host DNS services.
2. **Routing**: Allow traffic to be forwarded/routed from the virtual interfaces out to your physical internet interface, and vice versa for return traffic.
3. **Masquerading (NAT)**: The firewall must perform "Masquerading" so that traffic leaving the sandbox appears to come from the host's IP address.

### User Requirement

The invoking user must exist on the host before running `sbx create`.
This user is mirrored inside the sandbox so bind-mounted files keep consistent ownership across the host and sandbox without user namespaces or UID remapping.

`sbx` does not broadly manage ownership of existing user-managed files. If an existing sandbox directory, cache, configuration directory, or snapshot directory has unsuitable ownership or permissions, fix it with normal system administration tools before retrying the command.

### Privileges

Most `sbx` commands require `sudo` (root) privileges on the host.
This is necessary because `systemd-nspawn`, `mmdebstrap`, and `debootstrap` run in their real-root modes to manage system-level resources (like network bridges and root filesystems) directly.
You will be prompted for confirmation before `sudo` is invoked.
