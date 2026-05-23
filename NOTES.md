# Notes

- `Sudo` resolves the binary to re-execute with `os.Executable()` only. If that
  fails, return an error instead of falling back to `PATH` lookup, because a
  command name from `PATH` may resolve to a different executable than the
  currently running process.
