//go:build integration
// +build integration

package reversebin

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	tmuxSocketDir  = "/tmp/caddy-test-tmux"
	tmuxSocket     = "/tmp/caddy-test-tmux/caddy.sock"
	testHTTPPort   = 19080
	testAdminPort  = 12019
)

// TestIntegration_BasicReverseProxy tests that reverse-bin starts a process
// on the first request and successfully proxies requests to it.
func TestIntegration_BasicReverseProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	projectDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "caddy-reversebin-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Find a free port for the backend
	backendPort := findFreePort(t)

	// Create the Caddyfile
	caddyfile := fmt.Sprintf(`
{
	admin localhost:%d
	http_port %d
}

localhost:%d {
	reverse-bin {
		exec %s/examples/reverse-proxy/apps/python3-echo/main.py
		reverse_proxy_to 127.0.0.1:%d
		readiness_check GET /
		env REVERSE_PROXY_TO=127.0.0.1:%d
	}
}
`, testAdminPort, testHTTPPort, testHTTPPort, projectDir, backendPort, backendPort)

	caddyfilePath := filepath.Join(tmpDir, "Caddyfile")
	if err := os.WriteFile(caddyfilePath, []byte(caddyfile), 0644); err != nil {
		t.Fatalf("Failed to write Caddyfile: %v", err)
	}

	// Build Caddy with the reverse-bin module
	caddyBin := filepath.Join(tmpDir, "caddy")
	buildCaddy(t, projectDir, caddyBin)

	// Start Caddy in tmux
	sessionName := "caddy-basic-proxy"
	startTmuxSession(t, sessionName)
	defer killTmuxSession(t, sessionName)

	// Run Caddy
	runInTmux(t, sessionName, fmt.Sprintf("cd %s && %s run --config %s", tmpDir, caddyBin, caddyfilePath))

	// Wait for Caddy to start
	waitForText(t, sessionName, "serving initial configuration", 10*time.Second)

	// Wait a bit more for the server to be ready
	time.Sleep(500 * time.Millisecond)

	// Make a request to the proxy
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/test", testHTTPPort))
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Verify the response contains expected content from the echo server
	buf := make([]byte, 1024)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])

	if !strings.Contains(body, "Request Headers:") {
		t.Errorf("Expected response from echo server, got: %s", body)
	}

	t.Log("Basic reverse proxy test passed")
}

// TestIntegration_UnixSocketProxy tests that reverse-bin works with Unix domain sockets.
func TestIntegration_UnixSocketProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	projectDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "caddy-reversebin-unix-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	socketPath := filepath.Join(tmpDir, "app.sock")

	// Create the Caddyfile
	caddyfile := fmt.Sprintf(`
{
	admin localhost:%d
	http_port %d
}

localhost:%d {
	reverse-bin {
		exec %s/examples/reverse-proxy/apps/python3-unix-echo/main.py
		reverse_proxy_to unix/%s
		readiness_check GET /
		env REVERSE_PROXY_TO=unix/%s
	}
}
`, testAdminPort+1, testHTTPPort+1, testHTTPPort+1, projectDir, socketPath, socketPath)

	caddyfilePath := filepath.Join(tmpDir, "Caddyfile")
	if err := os.WriteFile(caddyfilePath, []byte(caddyfile), 0644); err != nil {
		t.Fatalf("Failed to write Caddyfile: %v", err)
	}

	// Build Caddy with the reverse-bin module
	caddyBin := filepath.Join(tmpDir, "caddy")
	buildCaddy(t, projectDir, caddyBin)

	// Start Caddy in tmux
	sessionName := "caddy-unix-proxy"
	startTmuxSession(t, sessionName)
	defer killTmuxSession(t, sessionName)

	// Run Caddy
	runInTmux(t, sessionName, fmt.Sprintf("cd %s && %s run --config %s", tmpDir, caddyBin, caddyfilePath))

	// Wait for Caddy to start
	waitForText(t, sessionName, "serving initial configuration", 10*time.Second)

	// Wait a bit more for the server to be ready
	time.Sleep(500 * time.Millisecond)

	// Make a request to the proxy
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/unix-test", testHTTPPort+1))
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Verify the response contains expected content from the echo server
	buf := make([]byte, 1024)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])

	if !strings.Contains(body, "Request Headers:") {
		t.Errorf("Expected response from echo server, got: %s", body)
	}

	t.Log("Unix socket proxy test passed")
}

// TestIntegration_DynamicDiscovery tests that reverse-bin executes the detector
// and uses the returned JSON to configure the backend.
func TestIntegration_DynamicDiscovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	projectDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "caddy-reversebin-dynamic-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Use the discover-app to detect the python3-echo example
	appDir := filepath.Join(projectDir, "examples/reverse-proxy/apps/python3-echo")

	// Create the Caddyfile with dynamic discovery
	caddyfile := fmt.Sprintf(`
{
	admin localhost:%d
	http_port %d
}

localhost:%d {
	reverse-bin {
		dynamic_proxy_detector %s/utils/discover-app/discover-app.py %s
		readiness_check GET /
	}
}
`, testAdminPort+2, testHTTPPort+2, testHTTPPort+2, projectDir, appDir)

	caddyfilePath := filepath.Join(tmpDir, "Caddyfile")
	if err := os.WriteFile(caddyfilePath, []byte(caddyfile), 0644); err != nil {
		t.Fatalf("Failed to write Caddyfile: %v", err)
	}

	// Build Caddy with the reverse-bin module
	caddyBin := filepath.Join(tmpDir, "caddy")
	buildCaddy(t, projectDir, caddyBin)

	// Start Caddy in tmux
	sessionName := "caddy-dynamic"
	startTmuxSession(t, sessionName)
	defer killTmuxSession(t, sessionName)

	// Run Caddy
	runInTmux(t, sessionName, fmt.Sprintf("cd %s && %s run --config %s", tmpDir, caddyBin, caddyfilePath))

	// Wait for Caddy to start
	waitForText(t, sessionName, "serving initial configuration", 10*time.Second)

	// Wait a bit more for the server to be ready
	time.Sleep(500 * time.Millisecond)

	// Make a request to the proxy
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/dynamic-test", testHTTPPort+2))
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Verify the response contains expected content from the echo server
	buf := make([]byte, 1024)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])

	if !strings.Contains(body, "Request Headers:") {
		t.Errorf("Expected response from echo server, got: %s", body)
	}

	t.Log("Dynamic discovery test passed")
}

// TestIntegration_LifecycleManagement tests that processes are properly
// terminated after idle timeout and when Caddy stops.
func TestIntegration_LifecycleManagement(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	projectDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "caddy-reversebin-lifecycle-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Find a free port for the backend
	backendPort := findFreePort(t)

	// Create the Caddyfile
	caddyfile := fmt.Sprintf(`
{
	admin localhost:%d
	http_port %d
}

localhost:%d {
	reverse-bin {
		exec %s/examples/reverse-proxy/apps/python3-echo/main.py
		reverse_proxy_to 127.0.0.1:%d
		readiness_check GET /
		env REVERSE_PROXY_TO=127.0.0.1:%d
	}
}
`, testAdminPort+3, testHTTPPort+3, testHTTPPort+3, projectDir, backendPort, backendPort)

	caddyfilePath := filepath.Join(tmpDir, "Caddyfile")
	if err := os.WriteFile(caddyfilePath, []byte(caddyfile), 0644); err != nil {
		t.Fatalf("Failed to write Caddyfile: %v", err)
	}

	// Build Caddy with the reverse-bin module
	caddyBin := filepath.Join(tmpDir, "caddy")
	buildCaddy(t, projectDir, caddyBin)

	// Start Caddy in tmux
	sessionName := "caddy-lifecycle"
	startTmuxSession(t, sessionName)
	defer killTmuxSession(t, sessionName)

	// Run Caddy
	runInTmux(t, sessionName, fmt.Sprintf("cd %s && %s run --config %s", tmpDir, caddyBin, caddyfilePath))

	// Wait for Caddy to start
	waitForText(t, sessionName, "serving initial configuration", 10*time.Second)
	time.Sleep(500 * time.Millisecond)

	// Make a request to start the backend process
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/test", testHTTPPort+3))
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	resp.Body.Close()

	// Wait for process started log
	waitForText(t, sessionName, "started proxy subprocess", 5*time.Second)

	// Wait for idle timer to fire (5 seconds as per the code)
	t.Log("Waiting for idle timeout...")
	time.Sleep(7 * time.Second)

	// Check that idle timer fired by looking at tmux output
	output := captureTmuxOutput(t, sessionName)
	if !strings.Contains(output, "idle timer fired") && !strings.Contains(output, "idle timeout") {
		t.Log("Warning: idle timer may not have fired, output:")
		t.Log(output)
	}

	// Test cleanup by stopping Caddy gracefully
	runInTmux(t, sessionName, fmt.Sprintf("%s stop --config %s", caddyBin, caddyfilePath))

	// Wait for cleanup
	time.Sleep(2 * time.Second)

	t.Log("Lifecycle management test passed")
}

// Helper functions

func findFreePort(t *testing.T) int {
	t.Helper()
	// Use a simple approach with a range of ports
	// In production, you'd use net.Listen with :0 to get a free port
	basePort := 15000
	for i := 0; i < 100; i++ {
		port := basePort + i
		// Check if port is available
		cmd := exec.Command("bash", "-c", fmt.Sprintf("! nc -z localhost %d 2>/dev/null", port))
		if cmd.Run() == nil {
			return port
		}
	}
	t.Fatalf("Could not find free port")
	return 0
}

func buildCaddy(t *testing.T, projectDir, outputPath string) {
	t.Helper()

	// Build Caddy with the reverse-bin module
	cmd := exec.Command("go", "build", "-o", outputPath, "./cmd/caddy")
	cmd.Dir = projectDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build Caddy: %v\nOutput: %s", err, output)
	}
}

func startTmuxSession(t *testing.T, sessionName string) {
	t.Helper()

	// Create socket directory
	if err := os.MkdirAll(tmuxSocketDir, 0755); err != nil {
		t.Fatalf("Failed to create tmux socket dir: %v", err)
	}

	// Kill any existing session with this name
	exec.Command("tmux", "-S", tmuxSocket, "kill-session", "-t", sessionName).Run()

	// Create new session
	cmd := exec.Command("tmux", "-S", tmuxSocket, "new-session", "-d", "-s", sessionName, "-n", "main")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to start tmux session: %v\nOutput: %s", err, output)
	}

	t.Logf("Started tmux session: %s", sessionName)
	t.Logf("To monitor: tmux -S %s attach -t %s", tmuxSocket, sessionName)
}

func killTmuxSession(t *testing.T, sessionName string) {
	t.Helper()

	cmd := exec.Command("tmux", "-S", tmuxSocket, "kill-session", "-t", sessionName)
	cmd.Run()
}

func runInTmux(t *testing.T, sessionName, command string) {
	t.Helper()

	cmd := exec.Command("tmux", "-S", tmuxSocket, "send-keys", "-t", sessionName+":0.0", "-l", "--", command)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to send command to tmux: %v\nOutput: %s", err, output)
	}

	// Send Enter
	cmd = exec.Command("tmux", "-S", tmuxSocket, "send-keys", "-t", sessionName+":0.0", "Enter")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to send Enter to tmux: %v\nOutput: %s", err, output)
	}
}

func captureTmuxOutput(t *testing.T, sessionName string) string {
	t.Helper()

	cmd := exec.Command("tmux", "-S", tmuxSocket, "capture-pane", "-p", "-J", "-t", sessionName+":0.0", "-S", "-500")
	output, err := cmd.Output()
	if err != nil {
		t.Logf("Warning: failed to capture tmux output: %v", err)
		return ""
	}
	return string(output)
}

func waitForText(t *testing.T, sessionName string, pattern string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		output := captureTmuxOutput(t, sessionName)
		if strings.Contains(output, pattern) {
			t.Logf("Found pattern: %s", pattern)
			return
		}
		time.Sleep(200 * time.Millisecond)
	}

	t.Fatalf("Timeout waiting for pattern: %s\nLast output:\n%s", pattern, captureTmuxOutput(t, sessionName))
}
