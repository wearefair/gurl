package proxy

import (
	"context"

	"github.com/wearefair/gurl/pkg/jsonpb"
)

// Caller is the interface that wraps calling across the wire
type Caller interface {
	Invoke(context.Context, *jsonpb.Request) ([]byte, error)
}
