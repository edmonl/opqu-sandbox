# Sandbox Names

## Name Validation

Sandbox names are validated as follows:
- Must not be empty.
- Allowed characters: lowercase alphanumeric and hyphens.
- Maximum length of 12 characters (see [Networking](#networking)).

## Networking

Each sandbox uses a network `zone name` calculated from the sandbox name. To stay under the 15-character Linux interface name limit including the mandatory `vz-` prefix for bridges, the zone prefix `opqu-` is dynamically shortened if sandbox `{name}` is long, as follows:
1. Use `opqu-{name}` if possible.
2. Otherwise use a truncated `opqu` prefix with a hyphen, e.g., `opq-{name}`, `op-{name}`, `o-{name}`.
3. Otherwise use `o{name}`.
4. Otherwise use the first 12 characters of `{name}`.

Examples of sandbox names to bridge names:
- `test` → Bridge: `vz-opqu-test` (12 chars)
- `12345678` (8) → Bridge: `vz-opq-12345678` (15 chars)
- `1234567890` (10) → Bridge: `vz-o-1234567890` (15 chars)
- `12345678901` (11) → Bridge: `vz-o12345678901` (15 chars)
- `123456789012` (12) → Bridge: `vz-123456789012` (15 chars)
