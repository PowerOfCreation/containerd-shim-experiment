# containerd-shim-mine

Companion code for the blog post
[The long way from `kubectl apply` to a running container](https://medium.com/@questerdesura/the-long-way-from-kubectl-apply-to-a-running-container-7ef4536d1312).

A minimal containerd v2 runtime shim (`io.containerd.mine.v1`) that shows how
containerd finds, launches and talks to a shim.

## Scope

**Only the shim bootstrap is demonstrated.** containerd starts the binary, the
shim spawns its daemon, serves the ttrpc task API on a socket, and containerd
connects to it.

The task API is intentionally left unimplemented: `Create`, `Start`, `Exec`,
`State`, `Kill`, `Wait`, … all return `ErrNotImplemented`. **No container can be
started.** The expected result of a run is:

```
failed to create shim task: not implemented
```

Two `delete` paths exist, and they behave differently:

- Task API `Delete` (`taskService.Delete`) only acknowledges the request and
  returns the current time as `ExitedAt`. There is nothing to kill or clean up.
- Bootstrap `delete` (`manager.Stop`, containerd's fallback when the shim is
  unreachable) returns `ErrNotImplemented`.

## Prerequisites

- Linux with containerd **2.x** and `ctr` (checked with containerd v2.3.4;
  the module depends on `containerd/v2` v2.3.5)
- Go 1.26.3 (see `go.mod`)
- root access (`sudo`) for `ctr` and installing the binary

## Build

```sh
go build -o out/containerd-shim-mine-v1 ./cmd
sudo cp out/containerd-shim-mine-v1 /usr/local/bin/
```

The binary name must be `containerd-shim-mine-v1`: containerd derives it from
the runtime name `io.containerd.mine.v1` and looks it up on `PATH`.

## Run

```sh
sudo ctr image pull docker.io/library/busybox:1.37
sudo ctr run --rm --runtime io.containerd.mine.v1 -t docker.io/library/busybox:1.37 test sh
```

## Expected result

`ctr run` fails with `failed to create shim task: not implemented`. That means
containerd found the shim, bootstrapped it and reached the task API, which
declined `Create` as designed. Without `--rm`, remove leftovers with
`sudo ctr container rm test`.
