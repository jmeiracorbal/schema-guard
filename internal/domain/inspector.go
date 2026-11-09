// Package domain defines the port implemented by each database introspection adapter.
package domain

import (
	"context"

	"github.com/jmeiracorbal/schema-guard/schema"
)

// Inspector reads structural metadata without modifying the source database.
// Implementations must report unsupported metadata explicitly, rather than
// silently substituting a lossy representation.
type Inspector interface {
	Engine() schema.Engine
	Inspect(context.Context) (schema.Snapshot, error)
}
