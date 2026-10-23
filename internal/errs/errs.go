// Package errs holds the sentinel errors shared by Argon's public API and its
// SDK adapter, so the adapter can map them to protocol errors without
// importing the root package.
package errs

import "errors"

var (
	// ResourceNotFound marks a resources/read for a missing resource.
	ResourceNotFound = errors.New("resource not found")
	// InvalidArguments marks rejected prompt (or tool) arguments.
	InvalidArguments = errors.New("invalid arguments")
)
