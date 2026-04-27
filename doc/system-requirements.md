# System Requirements

`sbxctl status` may be used to check the requirements.

## Host Commands

The following host commands are used:
- `mmdebstrap`: Used to bootstrap the Debian rootfs securely and efficiently. (Primary)
- `debootstrap`: Used as a fallback if `mmdebstrap` is not available.
- `systemd-nspawn`: The core container engine used to run the sandboxes.
- `systemd-run`: Used to daemonize sandbox processes as transient systemd services.
- `machinectl`: Used to manage container lifecycle and provide shell access.
- `tar` (with Zstandard support) and `zstd`: Used for creating and extracting base images and snapshots.
- `ip`: Used for managing and cleaning up network bridges.
- `systemctl`: Used for managing and cleaning up transient systemd units.

### Networking Requirements

For sandboxes to have outbound internet access, the host must be configured the host system must be configured to handle virtual routing and DHCP, as well as firewall traversal. Bridge `vz-{zone name}` (see [Sandbox Names](sandbox-names.md#networking) about the zone name) is created and a DHCP server runs on it. Inside sandboxes `systemd-networkd` is enabled and stared at bootstrap as DHCP client.

#### Network Management

The sandbox uses `systemd-nspawn` zones, which create virtual bridges on the host (prefixed with `vz-`). These bridges require `systemd-networkd` to provide DHCP leases and perform NAT (Network Address Translation).
You may check if the service is already active by `systemctl status systemd-networkd`, or start the service using `sudo systemctl enable --now systemd-networkd`, which also auto-starts the service. Note to check your current network management to make sure no conflicts with `systemd-networkd`.

#### IP Forwarding
For the host to act as a gateway for the sandboxes, the Linux kernel must allow packet forwarding between interfaces.
To check it, `cat /proc/sys/net/ipv4/ip_forward` must return `1`.
2.  **IP Forwarding**: IPv4 forwarding must be enabled in the kernel (`net.ipv4.ip_forward=1`).

#### Firewall Configuration

`sbxctl` does not touch firewalls on the host.
Regardless of which firewall you use (ufw, firewalld, or raw nftables/iptables), you must ensure it does not block the virtual traffic. Check for the following necessary rules (in natural language):
1. **Input**: Allow traffic from the virtual interfaces (`vz-*` and `vb-*`) to the host. This is required for the sandbox to request a DHCP IP address and reach host DNS services.
2. **Routing**: Allow traffic to be forwarded/routed from the virtual interfaces out to your physical internet interface, and vice versa for return traffic.
3. **Masquerading (NAT)**: The firewall must perform "Masquerading" so that traffic leaving the sandbox appears to come from the host's IP address.

### User Requirement

The `SANDBOX_USER` (see [Configuration](configuration.md#configuration-files) must already exist as a real user on the host before `sbxctl create` is run.
This is because the system uses real host UIDs inside the sandbox to ensure that bind-mounted files have consistent ownership on both sides without requiring user namespaces or UID remapping.

### Privileges

Most `sbxctl` commands require `sudo` (root) privileges on the host.
This is necessary because `systemd-nspawn`, `mmdebstrap`, and `debootstrap` are run in their real-root modes to manage system-level resources (like network bridges and root filesystems) directly.
Confirmation will be asked for before `sudo` is invoked.
