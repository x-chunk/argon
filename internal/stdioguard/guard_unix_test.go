//go:build unix

package stdioguard

import (
	"io"
	"os"
	"os/exec"
	"testing"

	"golang.org/x/sys/unix"
)

// swapFD points fd at the write end of a new pipe and returns the read end
// plus a function that puts the original fd back (which also closes the
// last write end, so reads on the pipe terminate).
func swapFD(t *testing.T, fd int) (*os.File, func()) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved, err := unix.Dup(fd)
	if err != nil {
		t.Fatal(err)
	}
	if err := unix.Dup2(int(w.Fd()), fd); err != nil {
		t.Fatal(err)
	}
	w.Close()
	return r, func() {
		unix.Dup2(saved, fd)
		unix.Close(saved)
	}
}

func TestCapture(t *testing.T) {
	stdout, putStdout := swapFD(t, 1)
	stderr, putStderr := swapFD(t, 2)

	protocol, restore, err := Capture()
	if err != nil {
		putStdout()
		putStderr()
		t.Fatal(err)
	}
	os.Stdout.WriteString("stray-go\n")
	unix.Write(1, []byte("stray-fd\n"))
	child := exec.Command("sh", "-c", "echo stray-child")
	child.Stdout = os.Stdout // a child inheriting fd 1 must not reach the protocol
	childErr := child.Run()
	protocol.WriteString("protocol\n")
	restore()
	putStdout()
	putStderr()

	if childErr != nil {
		t.Fatal(childErr)
	}
	out, _ := io.ReadAll(stdout)
	errOut, _ := io.ReadAll(stderr)
	if string(out) != "protocol\n" {
		t.Fatalf("stdout = %q, want only protocol output", out)
	}
	if want := "stray-go\nstray-fd\nstray-child\n"; string(errOut) != want {
		t.Fatalf("stderr = %q, want %q", errOut, want)
	}
}
