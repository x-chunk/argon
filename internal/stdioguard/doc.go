// Package stdioguard keeps stdout reserved for protocol messages on the stdio
// transport: stray writes from user code would corrupt the JSON-RPC stream.
package stdioguard
