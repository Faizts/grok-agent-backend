package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
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
	hostWorkspace := fmt.Sprintf("%s/%s", m.workspacePath, agentID)
	if err := os.MkdirAll(hostWorkspace, 0o755); err != nil {
		return nil, fmt.Errorf("create workspace dir: %w", err)
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
			Mounts: []mount.Mount{
				{
					Type:   mount.TypeBind,
					Source: hostWorkspace,
					Target: "/workspace",
				},
			},
			PortBindings: nat.PortMap{
				"5900/tcp": []nat.PortBinding{{HostIP: "127.0.0.1", HostPort: "0"}},
				"6080/tcp": []nat.PortBinding{{HostIP: "127.0.0.1", HostPort: "0"}},
			},
			Resources: container.Resources{
				Memory:   2 * 1024 * 1024 * 1024, // 2 GB
				NanoCPUs: 2 * 1e9,                 // 2 vCPUs
			},
			NetworkMode: "bridge",
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
		if c.State == "exited" {
			if err := m.docker.ContainerStart(ctx, c.ID, container.StartOptions{}); err == nil {
				info, _ := m.docker.ContainerInspect(ctx, c.ID)
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
func (m *Manager) Exec(ctx context.Context, containerID string, cmd []string) (*ExecResult, error) {
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

	var stdout, stderr bytes.Buffer
	io.Copy(&stdout, attach.Reader)

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
