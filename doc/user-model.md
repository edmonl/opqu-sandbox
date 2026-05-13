# User Model

User namespaces are not used. UIDs within the sandbox map directly to real host UIDs.

| Inside Sandbox | Host UID | Notes |
|---|---|---|
| `root` | 0 (real root) | Reachable via host sudo; password locked by default unless `ROOT_USER_PASSWORD` is set |
| `SANDBOX_USER` | same user on host | Default shell entry point; password locked |

Key properties:
- If `SANDBOX_USER` does not exist on the host, `sbx create` will fail.
- Bind-mounted files retain the same ownership on both the host and the sandbox without requiring UID remapping.
