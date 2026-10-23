//go:build unix

package argon_test

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestRunHelper is not a real test: it is the server process started by the
// tests below, which re-execute the test binary. It serves with Run, so
// transport selection and signal handling are exercised for real.
func TestRunHelper(t *testing.T) {
	if os.Getenv("ARGON_RUN_HELPER") != "1" {
		t.Skip("helper process for the Run tests")
	}
	if err := pingServer().Run(); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

// runHelper starts the helper process with extra arguments and environment.
func runHelper(t *testing.T, args []string, env ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], append([]string{"-test.run=^TestRunHelper$", "--"}, args...)...)
	cmd.Env = append(append(os.Environ(), "ARGON_RUN_HELPER=1", "ARGON_HTTP="), env...)
	return cmd
}

// waitExit waits for cmd and returns its exit code.
func waitExit(t *testing.T, cmd *exec.Cmd) int {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err == nil {
			return 0
		}
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		t.Fatalf("wait: %v", err)
	case <-time.After(10 * time.Second):
		cmd.Process.Kill()
		t.Fatal("server process did not exit")
	}
	return -1
}

func TestRunHTTPFromFlagStopsOnSIGTERM(t *testing.T) {
	addr := freeAddr(t)
	cmd := runHelper(t, []string{"--http", addr})
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waitListening(t, addr)
	if code := postInitialize(t, "http://"+addr+"/"); code != 200 {
		t.Fatalf("initialize: status %d", code)
	}
	cmd.Process.Signal(syscall.SIGTERM)
	if code := waitExit(t, cmd); code != 0 {
		t.Fatalf("exit code after SIGTERM = %d, want 0", code)
	}
}

func TestRunHTTPFromEnvStopsOnSIGINT(t *testing.T) {
	addr := freeAddr(t)
	cmd := runHelper(t, nil, "ARGON_HTTP="+addr)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waitListening(t, addr)
	cmd.Process.Signal(syscall.SIGINT)
	if code := waitExit(t, cmd); code != 0 {
		t.Fatalf("exit code after SIGINT = %d, want 0", code)
	}
}

func TestRunDefaultsToStdio(t *testing.T) {
	cmd := runHelper(t, nil)
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	io.WriteString(stdin, initializeRequest+"\n")
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || !strings.Contains(line, `"id":1`) || !strings.Contains(line, `"serverInfo"`) {
		t.Fatalf("stdio initialize response = %q, %v", line, err)
	}
	stdin.Close() // EOF ends the stdio session
	if code := waitExit(t, cmd); code != 0 {
		t.Fatalf("exit code after stdin EOF = %d, want 0", code)
	}
}
