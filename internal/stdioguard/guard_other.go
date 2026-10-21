//go:build !unix

package stdioguard

import "os"

// Capture takes ownership of the process stdout for the protocol and points
// os.Stdout at stderr. On this platform only writes through os.Stdout are
// redirected; writes to the raw handle (cgo, child processes) are not.
func Capture() (protocol *os.File, restore func(), err error) {
	orig := os.Stdout
	os.Stdout = os.Stderr
	return orig, func() { os.Stdout = orig }, nil
}
