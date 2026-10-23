//go:build unix

package stdioguard

import (
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// Capture takes ownership of the process stdout for the protocol. File
// descriptor 1 is pointed at stderr, so writes through os.Stdout, from cgo
// code and from child processes all end up in logs instead of the protocol
// stream. The returned file is a private duplicate of the original stdout
// (close-on-exec, so children never inherit it). restore undoes everything.
func Capture() (protocol *os.File, restore func(), err error) {
	// Hold ForkLock across dup and CloseOnExec, as the standard library does:
	// otherwise a child started by another goroutine in between would
	// inherit the protocol stream. (F_DUPFD_CLOEXEC is not available on
	// every Unix.)
	syscall.ForkLock.RLock()
	fd, err := unix.Dup(1)
	if err == nil {
		unix.CloseOnExec(fd)
	}
	syscall.ForkLock.RUnlock()
	if err != nil {
		return nil, nil, fmt.Errorf("stdioguard: dup stdout: %w", err)
	}
	if err := unix.Dup2(2, 1); err != nil {
		unix.Close(fd)
		return nil, nil, fmt.Errorf("stdioguard: redirect stdout: %w", err)
	}
	protocol = os.NewFile(uintptr(fd), "/dev/stdout")
	return protocol, func() {
		unix.Dup2(fd, 1)
		protocol.Close()
	}, nil
}
