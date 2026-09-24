package mine

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	bootapi "github.com/containerd/containerd/api/runtime/bootstrap/v1"
	taskAPI "github.com/containerd/containerd/api/runtime/task/v2"
	apitypes "github.com/containerd/containerd/api/types"
	"github.com/containerd/containerd/v2/defaults"
	"github.com/containerd/containerd/v2/pkg/protobuf"
	ptypes "github.com/containerd/containerd/v2/pkg/protobuf/types"
	"github.com/containerd/containerd/v2/pkg/shim"
	"github.com/containerd/containerd/v2/pkg/shutdown"
	"github.com/containerd/containerd/v2/plugins"
	"github.com/containerd/errdefs"
	"github.com/containerd/plugin"
	"github.com/containerd/plugin/registry"
	"github.com/containerd/ttrpc"
)

func init() {
	registry.Register(&plugin.Registration{
		Type: plugins.TTRPCPlugin,
		ID:   "task",
		Requires: []plugin.Type{
			plugins.EventPlugin,
			plugins.InternalPlugin,
		},
		InitFn: func(ic *plugin.InitContext) (any, error) {
			pp, err := ic.GetByID(plugins.EventPlugin, "publisher")
			if err != nil {
				return nil, err
			}
			ss, err := ic.GetByID(plugins.InternalPlugin, "shutdown")
			if err != nil {
				return nil, err
			}
			return newTaskService(ic.Context, pp.(shim.Publisher), ss.(shutdown.Service))
		},
	})
}

// NewManager returns the shim.Shim implementation containerd's shim binary
// entrypoint (RunShim) needs to bootstrap and tear down the shim process.
func NewManager(name string) shim.Shim {
	return manager{name: name}
}

type manager struct {
	name string
}

func (m manager) Name() string {
	return m.name
}

// Start is invoked once per `ctr run` for the "start" bootstrap action. It
// self-execs the shim binary (without "-start") as a detached daemon that
// will serve the task ttrpc API, and hands containerd back the socket
// address to connect to.
//
// One shim process per container, no pod/group sharing: the child owns and
// creates its own socket (via -socket), so there's no need to pre-bind it
// here and pass it down as an inherited fd.
func (m manager) Start(ctx context.Context, opts *bootapi.BootstrapParams) (*bootapi.BootstrapResult, error) {
	id := opts.GetInstanceID()

	socketDir := opts.GetSocketDir()
	if socketDir == "" {
		socketDir = filepath.Join(defaults.DefaultStateDir, "s")
	}
	address, err := shim.CreateSocketAddress(ctx, socketDir, opts.GetContainerdGrpcAddress(), id, false)
	if err != nil {
		return nil, err
	}

	// A shim for this exact namespace+id+containerd instance is already
	// listening here - hand back its address instead of trying (and failing)
	// to bind a second one.
	if shim.CanConnect(address) {
		return &bootapi.BootstrapResult{Version: 3, Protocol: "ttrpc", Address: address}, nil
	}

	rawPath := strings.TrimPrefix(address, "unix://")

	// socketDir is tmpfs-backed and nobody guarantees it exists before us
	if err := os.MkdirAll(filepath.Dir(rawPath), 0700); err != nil {
		return nil, err
	}

	self, err := os.Executable()
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(self,
		"-namespace", opts.GetNamespace(),
		"-id", id,
		"-address", opts.GetContainerdGrpcAddress(),
		"-socket", rawPath,
	)
	cmd.Env = append(os.Environ(), "TTRPC_ADDRESS="+opts.GetContainerdTtrpcAddress())
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go cmd.Wait() // reap it; the daemon detaches and outlives this call

	for i := 0; i < 100 && !shim.CanConnect(address); i++ {
		time.Sleep(10 * time.Millisecond)
	}

	return &bootapi.BootstrapResult{Version: 3, Protocol: "ttrpc", Address: address}, nil
}

// Stop handles the "delete" bootstrap action - containerd's fallback path
// for cleaning up after a shim it can no longer reach over ttrpc (see
// core/runtime/v2/shim.go's "cleaning up after shim disconnected"). The
// happy path never gets here: taskService.Delete + Shutdown tear the shim
// down over the live connection instead.
func (m manager) Stop(ctx context.Context, id string) (shim.StopStatus, error) {
	return shim.StopStatus{}, errdefs.ErrNotImplemented
}

func (m manager) Info(ctx context.Context, optionsR io.Reader) (*apitypes.RuntimeInfo, error) {
	return &apitypes.RuntimeInfo{
		Name: "io.containerd.mine.v1",
		Version: &apitypes.RuntimeVersion{
			Version: "v0.0.0",
		},
	}, nil
}

func newTaskService(ctx context.Context, publisher shim.Publisher, sd shutdown.Service) (taskAPI.TTRPCTaskService, error) {
	return &taskService{sd: sd}, nil
}

var _ = shim.TTRPCService(&taskService{})

type taskService struct {
	sd shutdown.Service
}

// RegisterTTRPC allows TTRPC services to be registered with the underlying server
func (s *taskService) RegisterTTRPC(server *ttrpc.Server) error {
	taskAPI.RegisterTTRPCTaskService(server, s)
	return nil
}

func (s *taskService) Create(ctx context.Context, r *taskAPI.CreateTaskRequest) (*taskAPI.CreateTaskResponse, error) {
	return nil, errdefs.ErrNotImplemented
}

func (s *taskService) Start(ctx context.Context, r *taskAPI.StartRequest) (*taskAPI.StartResponse, error) {
	return nil, errdefs.ErrNotImplemented
}

// Delete tears down a task. There's no real container lifecycle behind it
// yet (Create is still ErrNotImplemented), so there's nothing to kill - this
// just acknowledges the delete so containerd proceeds to call Shutdown.
func (s *taskService) Delete(ctx context.Context, r *taskAPI.DeleteRequest) (*taskAPI.DeleteResponse, error) {
	return &taskAPI.DeleteResponse{
		ExitedAt: protobuf.ToTimestamp(time.Now()),
	}, nil
}

func (s *taskService) Exec(ctx context.Context, r *taskAPI.ExecProcessRequest) (*ptypes.Empty, error) {
	return nil, errdefs.ErrNotImplemented
}

func (s *taskService) ResizePty(ctx context.Context, r *taskAPI.ResizePtyRequest) (*ptypes.Empty, error) {
	return nil, errdefs.ErrNotImplemented
}

func (s *taskService) State(ctx context.Context, r *taskAPI.StateRequest) (*taskAPI.StateResponse, error) {
	return nil, errdefs.ErrNotImplemented
}

func (s *taskService) Pause(ctx context.Context, r *taskAPI.PauseRequest) (*ptypes.Empty, error) {
	return nil, errdefs.ErrNotImplemented
}

func (s *taskService) Resume(ctx context.Context, r *taskAPI.ResumeRequest) (*ptypes.Empty, error) {
	return nil, errdefs.ErrNotImplemented
}

func (s *taskService) Kill(ctx context.Context, r *taskAPI.KillRequest) (*ptypes.Empty, error) {
	return nil, errdefs.ErrNotImplemented
}

func (s *taskService) Pids(ctx context.Context, r *taskAPI.PidsRequest) (*taskAPI.PidsResponse, error) {
	return nil, errdefs.ErrNotImplemented
}

func (s *taskService) CloseIO(ctx context.Context, r *taskAPI.CloseIORequest) (*ptypes.Empty, error) {
	return nil, errdefs.ErrNotImplemented
}

func (s *taskService) Checkpoint(ctx context.Context, r *taskAPI.CheckpointTaskRequest) (*ptypes.Empty, error) {
	return nil, errdefs.ErrNotImplemented
}

func (s *taskService) Connect(ctx context.Context, r *taskAPI.ConnectRequest) (*taskAPI.ConnectResponse, error) {
	return nil, errdefs.ErrNotImplemented
}

// Shutdown ends the shim daemon. One shim per container, no group sharing,
// so unlike runc-v2 there's no "other containers still running" check -
// this is always the last (only) task, so shut down unconditionally.
// s.sd.Shutdown() cancels the serve() context, which runs the normal
// cleanupSockets path before the process exits - no manual socket/pid
// bookkeeping needed.
func (s *taskService) Shutdown(ctx context.Context, r *taskAPI.ShutdownRequest) (*ptypes.Empty, error) {
	s.sd.Shutdown()
	return &ptypes.Empty{}, nil
}

func (s *taskService) Stats(ctx context.Context, r *taskAPI.StatsRequest) (*taskAPI.StatsResponse, error) {
	return nil, errdefs.ErrNotImplemented
}

func (s *taskService) Update(ctx context.Context, r *taskAPI.UpdateTaskRequest) (*ptypes.Empty, error) {
	return nil, errdefs.ErrNotImplemented
}

func (s *taskService) Wait(ctx context.Context, r *taskAPI.WaitRequest) (*taskAPI.WaitResponse, error) {
	return nil, errdefs.ErrNotImplemented
}
