package service

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"clickhouse-tui/internal/config"
)

type Status int

const (
	StatusUnknown Status = iota
	StatusRunning
	StatusStopped
)

func (s Status) String() string {
	switch s {
	case StatusRunning:
		return "Running"
	case StatusStopped:
		return "Stopped"
	default:
		return "Unknown"
	}
}

// Check returns whether ClickHouse server is running for the given connection.
func Check(conn config.Connection) Status {
	// Check if clickhouse-server process is listening on the configured port
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		cmd = exec.Command("lsof", "-i", fmt.Sprintf(":%s", conn.Port), "-sTCP:LISTEN")
	} else {
		cmd = exec.Command("ss", "-tlnp", fmt.Sprintf("sport = :%s", conn.Port))
	}

	out, err := cmd.Output()
	if err != nil {
		return StatusStopped
	}
	if len(strings.TrimSpace(string(out))) > 0 {
		return StatusRunning
	}
	return StatusStopped
}

// Start attempts to start the ClickHouse server.
func Start(conn config.Connection) error {
	// Try clickhouse-server first, fall back to docker
	if path, err := exec.LookPath("clickhouse-server"); err == nil {
		cmd := exec.Command(path, "--daemon",
			"--config-file=/etc/clickhouse-server/config.xml",
		)
		return cmd.Start()
	}

	// Try docker
	if _, err := exec.LookPath("docker"); err == nil {
		cmd := exec.Command("docker", "run", "-d",
			"--name", fmt.Sprintf("clickhouse-%s", conn.Name),
			"-p", fmt.Sprintf("%s:9000", conn.Port),
			"-e", fmt.Sprintf("CLICKHOUSE_USER=%s", conn.User),
			"-e", fmt.Sprintf("CLICKHOUSE_PASSWORD=%s", conn.Password),
			"-e", fmt.Sprintf("CLICKHOUSE_DB=%s", conn.Database),
			"clickhouse/clickhouse-server:latest",
		)
		return cmd.Run()
	}

	return fmt.Errorf("neither clickhouse-server nor docker found in PATH")
}

// Stop attempts to stop the ClickHouse server.
func Stop(conn config.Connection) error {
	// Try docker stop first
	if _, err := exec.LookPath("docker"); err == nil {
		containerName := fmt.Sprintf("clickhouse-%s", conn.Name)
		cmd := exec.Command("docker", "stop", containerName)
		if err := cmd.Run(); err == nil {
			exec.Command("docker", "rm", containerName).Run()
			return nil
		}
	}

	// Try killing clickhouse-server process on the port
	if runtime.GOOS == "darwin" {
		cmd := exec.Command("bash", "-c",
			fmt.Sprintf("lsof -i :%s -sTCP:LISTEN -t | xargs kill", conn.Port))
		return cmd.Run()
	}

	cmd := exec.Command("bash", "-c",
		fmt.Sprintf("fuser -k %s/tcp", conn.Port))
	return cmd.Run()
}
