# User Model

No user namespaces are used. UIDs inside the sandbox are real host UIDs.

| Inside Sandbox | Host UID | Notes |
|---|---|---|
| `root` | 0 (real root) | Password locked; reachable only via host sudo |
| `SANDBOX_USER` | same user on host | Default shell entry point; password locked |

Key properties:
- Bind-mounted files owned by the same users on both sides, no remapping required.
- `root` inside sandboxes are protected by locking the password, requiring host sudo to reach.
- `SANDBOX_USER` has no sudo access inside the sandbox and thus has no escalation path from inside. `sudo` may be present depending on `VARIANT`. It may be included in `standard`, but `SANDBOX_USER` is never added to sudoers. The bootstrapping process (via `mmdebstrap` hooks or `debootstrap` commands) deliberately omits this, so the absence of privilege escalation is by omission, not by an explicit deny rule. If the user is absent on the host, `sbxctl create` fails explicitly.
