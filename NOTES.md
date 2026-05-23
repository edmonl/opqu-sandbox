# Notes

- `Sudo` resolves the binary to re-execute with `os.Executable()` only. If that
  fails, return an error instead of falling back to `PATH` lookup, because a
  command name from `PATH` may resolve to a different executable than the
  currently running process.
- `Extract` defers directory metadata restoration until after all archive
  entries are extracted. This keeps directory mtimes correct after child
  creation, restores modes after umask and temporary write access, and avoids
  ownership/mode changes blocking later child extraction. A stack-based early
  restore would only be correct for depth-first archives; the current extractor
  only requires parent directories to appear before children, so a later entry
  can still target a directory that would already have been popped.
- The project targets a Go version with standard library iterator helpers such
  as `slices.Backward`; prefer them for reverse slice traversal when they make
  the loop clearer.
