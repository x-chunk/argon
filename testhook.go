package argon

import (
	"go.xchunk.org/argon/internal/sdkadapter"
	"go.xchunk.org/argon/internal/testhook"
)

func init() {
	testhook.BuildServer = func(s any) (*sdkadapter.Server, error) { return s.(*Server).build() }
}
