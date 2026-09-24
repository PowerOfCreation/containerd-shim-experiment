# containerd-shim-mine

Skeleton containerd v2 runtime v2 shim (`io.containerd.mine.v1`). All task
methods return `ErrNotImplemented` — bootstrap/plumbing only, no engine yet.

## Build

```sh
go build -o out/containerd-shim-mine-v1 ./cmd && sudo cp out/containerd-shim-mine-v1 /usr/local/bin/containerd-shim-mine-v1
```

Binary name matters: containerd derives it from the runtime name
(`io.containerd.mine.v1` → `containerd-shim-mine-v1`), so it must be on `PATH`.

## Test

Needs containerd **2.x** (bootstrap protocol differs from 1.7).

```sh
sudo ctr image pull docker.io/library/busybox:1.37

sudo ctr run --rm --runtime io.containerd.mine.v1 -t docker.io/library/busybox:1.37 test sh

# Cleanup (only if ran ctr run without --rm)
sudo ctr snapshot rm test; sudo ctr container rm test
```

Check the shim is picked up:

```sh
ps aux | grep containerd-shim-mine-v1
```

## Dev loop

```sh
go build ./...      # compile check
go vet ./...
```
