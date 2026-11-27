# MyContainer
A lightweight Linux container runtime built with Go and C that separates control-plane management from kernel-level isolation.
A Go daemon listens over a Unix domain socket to create container IDs, assemble minimal root filesystems with required binaries and libraries, and launch a C runtime binary that uses clone3(), Linux namespaces, pivot_root(), and pseudo-filesystems(proc and sysfs) to run an isolated shell.

## Getting Started


### Step 1: Build the binaries

Ensure `main.go`, `client.go`, `daemon.go`, `go.mod`, and `Makefile` are at the project root, with `main.c` under `runtime/main.c`.
Run `make` to build `my-tool` and `my-container-runtime`:

```bash
make
```

### Step 2: Start the Daemon

Open a terminal and start the background daemon:

```bash
sudo ./my-tool daemon
```

wait until you see:

```text
Listening on socket /tmp/mycontainer.sock...
```
### Step 3: Create and Start a Container

In a second terminal, allocate a new container ID:

```bash
./my-tool create
```
Copy the returned `<id>` and start the isolated container session:

```bash
./my-tool start <id>
```
Inspect the isolated environment (`ls /`, `ps` or `ls /proc`), then type `exit` to leave the container shell.

### Step 4: Clean Up

Stop the daemon (`Ctrl+C`) once no containers are active, then run `make clean` to remove build binaries, lazy-unmount and remaining rootfs mounts, and delete `/tmp/my-containers/*`:

```bash
make clean
```
