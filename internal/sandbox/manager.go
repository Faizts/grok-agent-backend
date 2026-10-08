package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"github.com/docker/docker/errdefs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
)

// Manager manages Docker sandbox containers for agents.
type Manager struct {
	docker        *client.Client
	image         string
	workspacePath string
}

// SandboxInfo holds runtime info about a running sandbox.
type SandboxInfo struct {
	ContainerID string
	VNCPort     string
	NoVNCPort   string
}

func NewManager(image, workspacePath string) (*Manager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &Manager{docker: cli, image: image, workspacePath: workspacePath}, nil
}

// Create provisions a new sandbox container for the given agentID.
func (m *Manager) Create(ctx context.Context, agentID string) (*SandboxInfo, error) {
	workspaceMount := mount.Mount{Type: mount.TypeVolume, Source: "grok-agent-workspace-" + agentID, Target: "/workspace"}
	if m.workspacePath != "" {
		hostWorkspace := fmt.Sprintf("%s/%s", m.workspacePath, agentID)
		if err := os.MkdirAll(hostWorkspace, 0o755); err != nil {
			return nil, fmt.Errorf("create workspace dir: %w", err)
		}
		workspaceMount = mount.Mount{Type: mount.TypeBind, Source: hostWorkspace, Target: "/workspace"}
	}

	resp, err := m.docker.ContainerCreate(ctx,
		&container.Config{
			Image: m.image,
			Labels: map[string]string{
				"grok-agent": "sandbox",
				"agent-id":   agentID,
			},
			ExposedPorts: nat.PortSet{
				"5900/tcp": struct{}{},
				"6080/tcp": struct{}{},
			},
		},
		&container.HostConfig{
			Mounts: []mount.Mount{workspaceMount},
			PortBindings: nat.PortMap{
				"5900/tcp": []nat.PortBinding{{HostIP: "127.0.0.1", HostPort: "0"}},
				"6080/tcp": []nat.PortBinding{{HostIP: "127.0.0.1", HostPort: "0"}},
			},
			Resources: container.Resources{
				Memory:   int64(positiveEnv("SANDBOX_MEMORY_MB", 1024)) * 1024 * 1024,
				NanoCPUs: int64(positiveEnv("SANDBOX_CPUS", 1)) * 1e9,
			},
			NetworkMode: "bridge",
			ShmSize:     256 * 1024 * 1024,
		},
		nil, nil,
		fmt.Sprintf("grok-agent-%s", agentID),
	)
	if err != nil {
		return nil, fmt.Errorf("container create: %w", err)
	}

	if err := m.docker.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("container start: %w", err)
	}

	info, err := m.docker.ContainerInspect(ctx, resp.ID)
	if err != nil {
		return nil, fmt.Errorf("container inspect: %w", err)
	}

	vncPort := ""
	novncPort := ""
	if bindings, ok := info.NetworkSettings.Ports["5900/tcp"]; ok && len(bindings) > 0 {
		vncPort = bindings[0].HostPort
	}
	if bindings, ok := info.NetworkSettings.Ports["6080/tcp"]; ok && len(bindings) > 0 {
		novncPort = bindings[0].HostPort
	}

	return &SandboxInfo{
		ContainerID: resp.ID,
		VNCPort:     vncPort,
		NoVNCPort:   novncPort,
	}, nil
}

// GetOrCreate returns an existing sandbox for the agent or creates a new one.
func (m *Manager) GetOrCreate(ctx context.Context, agentID string) (*SandboxInfo, error) {
	f := filters.NewArgs(filters.Arg("label", fmt.Sprintf("agent-id=%s", agentID)))
	containers, err := m.docker.ContainerList(ctx, container.ListOptions{All: true, Filters: f})
	if err != nil {
		return nil, err
	}

	for _, c := range containers {
		if c.State == "running" {
			info, err := m.docker.ContainerInspect(ctx, c.ID)
			if err != nil {
				continue
			}
			si := &SandboxInfo{ContainerID: c.ID}
			if bindings, ok := info.NetworkSettings.Ports["5900/tcp"]; ok && len(bindings) > 0 {
				si.VNCPort = bindings[0].HostPort
			}
			if bindings, ok := info.NetworkSettings.Ports["6080/tcp"]; ok && len(bindings) > 0 {
				si.NoVNCPort = bindings[0].HostPort
			}
			return si, nil
		}
		// Restart stopped container
		if c.State == "exited" || c.State == "created" {
			if err := m.docker.ContainerStart(ctx, c.ID, container.StartOptions{}); err == nil {
				info, err := m.docker.ContainerInspect(ctx, c.ID)
				if err != nil {
					return nil, err
				}
				si := &SandboxInfo{ContainerID: c.ID}
				if bindings, ok := info.NetworkSettings.Ports["5900/tcp"]; ok && len(bindings) > 0 {
					si.VNCPort = bindings[0].HostPort
				}
				if bindings, ok := info.NetworkSettings.Ports["6080/tcp"]; ok && len(bindings) > 0 {
					si.NoVNCPort = bindings[0].HostPort
				}
				return si, nil
			}
		}
	}

	return m.Create(ctx, agentID)
}

// Stop stops a sandbox container.
func (m *Manager) Stop(ctx context.Context, containerID string) error {
	return m.docker.ContainerStop(ctx, containerID, container.StopOptions{})
}

// DesktopConnected reports an established VNC session, including noVNC clients.
func (m *Manager) DesktopConnected(ctx context.Context, containerID string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	result, err := m.Exec(ctx, containerID, []string{"sh", "-c", `awk '$2 ~ /:170C$/ && $4 == "01" { active=1 } END { print active ? "active" : "idle" }' /proc/net/tcp /proc/net/tcp6`})
	if err != nil {
		return false, err
	}
	if result.ExitCode != 0 {
		return false, fmt.Errorf("check desktop connection: %s", result.Stderr)
	}
	return strings.TrimSpace(result.Stdout) == "active", nil
}

// Remove stops and removes a sandbox container.
func (m *Manager) Remove(ctx context.Context, containerID string) error {
	return m.docker.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
}

// ExecResult holds output from a container exec.
type ExecResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// Exec runs a command inside a sandbox container.
func (m *Manager) Close() error { return m.docker.Close() }

func (m *Manager) Exec(ctx context.Context, containerID string, cmd []string) (*ExecResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	cmd = append([]string{"timeout", "--signal=TERM", "--kill-after=5", "115"}, cmd...)
	execResp, err := m.docker.ContainerExecCreate(ctx, containerID, types.ExecConfig{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return nil, err
	}

	attach, err := m.docker.ContainerExecAttach(ctx, execResp.ID, types.ExecStartCheck{})
	if err != nil {
		return nil, err
	}
	defer attach.Close()

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			attach.Close()
		case <-done:
		}
	}()
	var stdout, stderr bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &stderr, attach.Reader); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	inspect, err := m.docker.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		return nil, err
	}

	return &ExecResult{
		ExitCode: inspect.ExitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}, nil
}

// ExecShell runs a bash script inside the sandbox.
func (m *Manager) ExecShell(ctx context.Context, containerID, script string) (*ExecResult, error) {
	return m.Exec(ctx, containerID, []string{"bash", "-c", script})
}

// ExecPython runs a Python snippet inside the sandbox.
func (m *Manager) ExecPython(ctx context.Context, containerID, code string) (*ExecResult, error) {
	return m.Exec(ctx, containerID, []string{"python3", "-c", code})
}

func positiveEnv(name string, fallback int) int {
	n, err := strconv.Atoi(os.Getenv(name))
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

// Retain existing per-agent computers to preserve their data during upgrades.
func (m *Manager) ComputerKey(ctx context.Context, userID, agentID string) (string, error) {
	legacy, err := m.docker.ContainerList(ctx, container.ListOptions{All: true, Filters: filters.NewArgs(filters.Arg("label", "agent-id="+agentID), filters.Arg("label", "grok-agent=sandbox"))})
	if err != nil {
		return "", err
	}
	if len(legacy) > 0 {
		return agentID, nil
	}
	return "user-" + userID, nil
}
func (m *Manager) RemoveComputer(ctx context.Context, key string) error {
	computers, err := m.docker.ContainerList(ctx, container.ListOptions{All: true, Filters: filters.NewArgs(filters.Arg("label", "agent-id="+key), filters.Arg("label", "grok-agent=sandbox"))})
	if err != nil {
		return err
	}
	for _, computer := range computers {
		if err = m.Remove(ctx, computer.ID); err != nil && !errdefs.IsNotFound(err) {
			return err
		}
	}
	if m.workspacePath != "" {
		return os.RemoveAll(filepath.Join(m.workspacePath, key))
	}
	err = m.docker.VolumeRemove(ctx, "grok-agent-workspace-"+key, false)
	if errdefs.IsNotFound(err) {
		return nil
	}
	return err
}
