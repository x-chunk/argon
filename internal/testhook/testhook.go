// Package testhook lets argontest reach unexported server internals without
// widening the public API. The root package installs the hooks in init.
package testhook

import "go.xchunk.org/argon/internal/sdkadapter"

// BuildServer builds the SDK server for an *argon.Server (passed as any to
// avoid an import cycle).
var BuildServer func(s any) (*sdkadapter.Server, error)
