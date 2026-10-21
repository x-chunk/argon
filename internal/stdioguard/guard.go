// Package stdioguard keeps stdout reserved for protocol messages on the stdio
// transport: stray writes from user code would corrupt the JSON-RPC stream.
package stdioguard

import "os"

// Capture takes ownership of the process stdout for the protocol and points
// os.Stdout at stderr, so fmt.Print and friends end up in logs instead.
// The returned restore function undoes the redirection.
//
// TODO(v0.1): also dup2 file descriptor 1 so writes from cgo code and child
// processes are caught, not just writes through os.Stdout.
func Capture() (protocol *os.File, restore func()) {
	orig := os.Stdout
	os.Stdout = os.Stderr
	return orig, func() { os.Stdout = orig }
}
