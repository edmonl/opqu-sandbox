# User Model

No user namespaces are used. UIDs inside the sandbox are real host UIDs.

| Inside Sandbox | Host UID | Notes |
|---|---|---|
| `root` | 0 (real root) | Reachable via host sudo; password locked by default unless `ROOT_USER_PASSWORD` is set |
| `SANDBOX_USER` | same user on host | Default shell entry point; password locked |

Key properties:
- Bind-mounted files are owned by the same users on both sides, and no remapping is required.
- `root` inside sandboxes is protected by locking the password by default, requiring host sudo to access. If `ROOT_USER_PASSWORD` is configured, the password is set accordingly.
- `SANDBOX_USER` has no sudo access inside the sandbox, and thus has no privilege escalation path from inside. `sudo` may be present depending on `VARIANT`, but `SANDBOX_USER` is never added to sudoers. The bootstrapping process (via `mmdebstrap` hooks or `debootstrap` commands) deliberately omits this, so the absence of privilege escalation is by omission rather than by an explicit deny rule. If the user is absent on the host, `sbx create` will explicitly fail.

See [Configuration](configuration.md#configuration-files) for more information on these settings.
