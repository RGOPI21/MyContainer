# MyContainer
A lightweight Linux container runtime built with Go and C that separates control-plane management from kernel-level isolation.
A Go daemon listens over a Unix domain socket to create container IDs, assemble minimal root filesystems with required binaries and libraries, and launch a C runtime binary that uses clone3(), Linux namespaces, pivot_root(), and pseudo-filesystems(proc and sysfs) to run an isolated shell.
