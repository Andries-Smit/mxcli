// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ContainerCLI returns the container runtime binary ("docker" or "podman").
// Resolution order:
//  1. MXCLI_CONTAINER_CLI env var (explicit override)
//  2. "docker" if available on PATH
//  3. "podman" if available on PATH
//  4. "docker" as fallback (will fail with a clear error at exec time)
func ContainerCLI() string {
	if cli := os.Getenv("MXCLI_CONTAINER_CLI"); cli != "" {
		return cli
	}
	if _, err := exec.LookPath("docker"); err == nil {
		return "docker"
	}
	if _, err := exec.LookPath("podman"); err == nil {
		return "podman"
	}
	return "docker"
}

// RuntimeOptions configures docker runtime commands.
type RuntimeOptions struct {
	// ProjectPath is the path to the .mpr file.
	ProjectPath string

	// DockerDir is the .docker/ directory. Resolved from ProjectPath if empty.
	DockerDir string

	// Stdout for output messages.
	Stdout io.Writer

	// Stderr for error output.
	Stderr io.Writer
}

// resolveDockerDir finds the .docker/ directory for a project.
func resolveDockerDir(opts RuntimeOptions) (string, error) {
	dir := opts.DockerDir
	if dir == "" {
		dir = filepath.Join(filepath.Dir(opts.ProjectPath), ".docker")
	}

	composePath := filepath.Join(dir, "docker-compose.yml")
	if _, err := os.Stat(composePath); err != nil {
		return "", fmt.Errorf("docker-compose.yml not found in %s (run 'mxcli docker init' first)", dir)
	}

	warnIfComposeStale(composePath, opts.Stderr)

	return dir, nil
}

// warnIfComposeStale prints a warning when the project's docker-compose.yml
// predates the current embedded template. `mxcli docker init` skips an existing
// compose file (no --force), so template fixes never reach projects that ran
// init before the fix — the warning tells the user how to pick them up.
func warnIfComposeStale(composePath string, stderr io.Writer) {
	if stderr == nil {
		stderr = os.Stderr
	}
	current := currentComposeTemplateVersion()
	have := composeFileVersion(composePath)
	if current > 0 && have < current {
		fmt.Fprintf(stderr, "warning: %s is out of date (template v%d, current v%d); "+
			"regenerate with 'mxcli docker init --force' to pick up template fixes\n",
			composePath, have, current)
	}
}

// Up starts the Docker Compose stack.
func Up(opts RuntimeOptions, detach bool, fresh bool) error {
	dockerDir, err := resolveDockerDir(opts)
	if err != nil {
		return err
	}

	if fresh {
		if err := runCompose(dockerDir, opts, "down", "-v"); err != nil {
			return fmt.Errorf("stopping existing containers: %w", err)
		}
	}

	args := []string{"up", "--force-recreate"}
	if detach {
		args = append(args, "--detach")
	}

	return runCompose(dockerDir, opts, args...)
}

// WaitForReady tails docker compose logs until the Mendix runtime reports
// successful startup, or until the timeout expires.
// Returns nil if the runtime started successfully, or an error on timeout/failure.
func WaitForReady(opts RuntimeOptions, timeout time.Duration) error {
	dockerDir, err := resolveDockerDir(opts)
	if err != nil {
		return err
	}

	w := opts.Stdout
	if w == nil {
		w = os.Stdout
	}

	fmt.Fprintf(w, "Waiting for Mendix runtime to start (timeout: %s)...\n", timeout)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, ContainerCLI(), "compose", "logs", "--follow", "--no-log-prefix", "--since", "1s", "mendix")
	cmd.Dir = dockerDir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating log pipe: %w", err)
	}
	cmd.Stderr = cmd.Stdout // merge stderr into stdout

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting log follow: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	ready := false
	failed := false
	var failMsg string
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Fprintln(w, line)

		if strings.Contains(line, "Runtime successfully started") ||
			strings.Contains(line, "the application is now available") {
			ready = true
			cancel()
			break
		}
		if strings.Contains(line, "is not a file") ||
			strings.Contains(line, "Error starting runtime") ||
			strings.Contains(line, "Critical error") {
			failed = true
			failMsg = line
			cancel()
			break
		}
	}

	// Kill the log process (it may already be dead from context cancel)
	cmd.Process.Kill()
	cmd.Wait()

	if ready {
		appPort := "8080"
		if envVals, err := parseEnvFile(filepath.Join(dockerDir, ".env")); err == nil {
			if v, ok := envVals["APP_PORT"]; ok && v != "" {
				appPort = v
			}
		}
		fmt.Fprintln(w, "")
		fmt.Fprintf(w, "Application is ready at http://localhost:%s\n", appPort)
		return nil
	}

	if failed {
		return fmt.Errorf("runtime failed to start: %s", failMsg)
	}

	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("timeout after %s waiting for runtime to start (check logs with 'mxcli docker logs')", timeout)
	}

	return fmt.Errorf("log stream ended without startup confirmation")
}

// Down stops the Docker Compose stack.
func Down(opts RuntimeOptions, volumes bool) error {
	dockerDir, err := resolveDockerDir(opts)
	if err != nil {
		return err
	}

	args := []string{"down"}
	if volumes {
		args = append(args, "--volumes")
	}

	return runCompose(dockerDir, opts, args...)
}

// Logs shows logs from the Mendix container.
func Logs(opts RuntimeOptions, follow bool, tail int) error {
	dockerDir, err := resolveDockerDir(opts)
	if err != nil {
		return err
	}

	args := []string{"logs"}
	if follow {
		args = append(args, "--follow")
	}
	if tail > 0 {
		args = append(args, "--tail", fmt.Sprintf("%d", tail))
	}
	args = append(args, "mendix")

	return runCompose(dockerDir, opts, args...)
}

// containerStatus holds parsed docker compose ps output.
type containerStatus struct {
	Name    string `json:"Name"`
	Service string `json:"Service"`
	State   string `json:"State"`
	Status  string `json:"Status"`
	Ports   string `json:"Ports"`
}

// Status shows the status of Docker Compose services.
func Status(opts RuntimeOptions) error {
	dockerDir, err := resolveDockerDir(opts)
	if err != nil {
		return err
	}

	w := opts.Stdout
	if w == nil {
		w = os.Stdout
	}

	// Get JSON output from docker compose ps
	cmd := exec.Command(ContainerCLI(), "compose", "ps", "--format", "json")
	cmd.Dir = dockerDir

	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("docker compose ps: %w", err)
	}

	if len(strings.TrimSpace(string(output))) == 0 {
		fmt.Fprintln(w, "No containers running.")
		return nil
	}

	// docker compose ps --format json outputs one JSON object per line
	var containers []containerStatus
	for line := range strings.SplitSeq(strings.TrimSpace(string(output)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var cs containerStatus
		if err := json.Unmarshal([]byte(line), &cs); err != nil {
			continue
		}
		containers = append(containers, cs)
	}

	if len(containers) == 0 {
		fmt.Fprintln(w, "No containers running.")
		return nil
	}

	// Print table
	fmt.Fprintf(w, "%-20s %-12s %-30s %s\n", "SERVICE", "STATE", "STATUS", "PORTS")
	fmt.Fprintf(w, "%-20s %-12s %-30s %s\n", "-------", "-----", "------", "-----")
	for _, c := range containers {
		fmt.Fprintf(w, "%-20s %-12s %-30s %s\n", c.Service, c.State, c.Status, c.Ports)
	}

	return nil
}

// Shell opens an interactive shell or executes a command in the Mendix container.
func Shell(opts RuntimeOptions, execCmd string) error {
	dockerDir, err := resolveDockerDir(opts)
	if err != nil {
		return err
	}

	var args []string
	if execCmd != "" {
		args = []string{"exec", "mendix", "sh", "-c", execCmd}
	} else {
		args = []string{"exec", "-it", "mendix", "sh"}
	}

	cmd := exec.Command(ContainerCLI(), append([]string{"compose"}, args...)...)
	cmd.Dir = dockerDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = opts.Stdout
	if cmd.Stdout == nil {
		cmd.Stdout = os.Stdout
	}
	cmd.Stderr = opts.Stderr
	if cmd.Stderr == nil {
		cmd.Stderr = os.Stderr
	}

	return cmd.Run()
}

// runCompose executes a docker compose command in the given directory.
func runCompose(dockerDir string, opts RuntimeOptions, args ...string) error {
	cmd := exec.Command(ContainerCLI(), append([]string{"compose"}, args...)...)
	cmd.Dir = dockerDir
	cmd.Stdout = opts.Stdout
	if cmd.Stdout == nil {
		cmd.Stdout = os.Stdout
	}
	cmd.Stderr = opts.Stderr
	if cmd.Stderr == nil {
		cmd.Stderr = os.Stderr
	}
	cmd.Stdin = os.Stdin

	return cmd.Run()
}
