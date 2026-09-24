package main

import (
	"context"

	mine "github.com/niklas/containerd-shim-mine"

	"github.com/containerd/containerd/v2/pkg/shim"
)

func main() {
	shim.RunShim(context.Background(), mine.NewManager("io.containerd.mine.v1"))
}
