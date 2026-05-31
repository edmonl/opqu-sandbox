# User Model

User namespaces are not used. UIDs within the sandbox map directly to real host UIDs.

The sandbox user is the invoking user:

1. If `sbx` is launched by a non-root user, that user is mirrored inside the sandbox.
2. After `sbx` invokes `sudo`, the sandbox user is derived from `SUDO_USER`.
3. If root launches `sbx` directly, root is the invoking user and no separate non-root sandbox user is created.

User identity comes from the process that launched `sbx`.

| Inside Sandbox | Host UID | Notes |
|---|---|---|
| `root` | 0 (real root) | Reachable via host sudo; password locked by default unless `ROOT_USER_PASSWORD` is set |
| invoking user | same user on host | Default shell entry point for non-root invocations; password locked |

Key properties:
- Bind-mounted files retain the same ownership on both the host and the sandbox without requiring UID remapping.
- `sbx` does not broadly repair ownership of existing files and directories. Normal filesystem permissions determine whether a user can read or write user-managed paths.
- When `sbx` creates user-managed outputs through root, such as snapshot archives, it changes those outputs back to the invoking user where needed.
