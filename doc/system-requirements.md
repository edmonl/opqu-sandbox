# System Requirements

You can use `sbx status` to verify the system requirements.

## Host Commands

The following host commands are used by `sbx`:
- `mmdebstrap`: Used to provision the rootfs.
- `debootstrap`: Used as a fallback if `mmdebstrap` is not available.
- `systemd-nspawn`: The core container engine used to run the sandboxes.
- `systemd-run`: Used to daemonize sandbox processes as `systemd` services.
- `machinectl`: Used to manage container lifecycle and provide shell access.
- `systemctl`: Used to manage `systemd` units.
- `sudo`: Used when privileges are required and `sbx` is not started with `sudo`.
- `su`: Used as a fallback when `sudo` is not available.

Other commands that are not directly used but may be useful for administration:
- `networkctl`: Check the status of virtual interfaces.
- `ip`: Check network bridge and interface management.
- `sysctl`: Check and configure IP forwarding.
- `journald`: Read logs using `journalctl --machine opqu-sbx-{name}`.

On Debian, these *optional priority* packages provide some of the commands:
- `mmdebstrap`: provides `mmdebstrap`.
- `debootstrap`: provides `debootstrap`.
- `systemd-container`: provides `systemd-nspawn` and `machinectl`.
- `sudo`: provides `sudo`.

A standard Debian installation should already have these packages and commands available:
- `systemd` (important): provides `systemd-run`, `systemctl`, `networkctl`, and `systemd-networkd`.
- `iproute2` (important): provides `ip`.
- `procps` (important): provides `sysctl`.
- `util-linux` (required): provides `su`.

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
4. Because the bridge is temporary, a DHCP server (like `systemd-networkd`) must be configured on the host to listen for and serve these `vz-*` interfaces as they appear.

#### Network Management

The virtual bridges (prefixed with `vz-`) require `systemd-networkd` on the host to provide DHCP leases and perform NAT (Network Address Translation).

**Configuration Requirement:**
For the bridges to be managed automatically, a `systemd-networkd` configuration file (e.g., `/usr/lib/systemd/network/80-container-vz.network` or similar) must exist on the host to match `vz-*` interfaces.
- On Debian, this file is provided automatically by `systemd`.
- You can verify its presence by looking for a file containing `[Match] Name=vz-*` in `/usr/lib/systemd/network/` (or check the man page for `systemd.network` for other locations).

**Verification:**
You can use the `networkctl` command to verify the status of the virtual interfaces:
- `networkctl list`: Lists all interfaces and their setup status (should show `managed` for `vz-*` interfaces when sandboxes are running).
- `networkctl status vz-*`: Shows detailed information, including assigned IP addresses and DHCP server status.

You can check if the service is active with `systemctl status systemd-networkd`, or enable and start it using `systemctl enable --now systemd-networkd`.

#### IP Forwarding
For the host to act as a gateway for the sandboxes, the Linux kernel must allow packet forwarding between interfaces.
To check this, `cat /proc/sys/net/ipv4/ip_forward` must return `1`.
See the man pages for `sysctl.d`, `sysctl.conf`, or `sysctl` if you need to change this setting or persist it using `net.ipv4.ip_forward=1`.

#### Firewall Configuration

`sbx` does not modify firewalls on the host.
Regardless of which firewall you use, you must ensure it does not block virtual traffic. Verify the following necessary rules are in place:
1. **Input**: Allow traffic from the virtual interfaces (`vz-*` and `vb-*`) to the host. This is required for the sandbox to request a DHCP IP address and reach host DNS services.
2. **Routing**: Allow traffic to be forwarded/routed from the virtual interfaces out to your physical internet interface, and vice versa for return traffic.
3. **Masquerading (NAT)**: The firewall must perform "Masquerading" so that traffic leaving the sandbox appears to come from the host's IP address.

### User Requirement

`SANDBOX_USER` (see [Configuration](configuration.md#configuration-files)) must already exist as a real user on the host before `sbx create` is run.
This is because the system uses real host UIDs inside the sandbox to ensure that bind-mounted files have consistent ownership on both sides without requiring user namespaces or UID remapping.

### Privileges

Most `sbx` commands require `sudo` (root) privileges on the host.
This is necessary because `systemd-nspawn`, `mmdebstrap`, and `debootstrap` run in their real-root modes to manage system-level resources (like network bridges and root filesystems) directly.
Confirmation will be requested before `sudo` is invoked.
