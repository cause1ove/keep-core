package beacon

import "github.com/keep-network/keep-core/pkg/sortition"

// Handle for interaction with the Random Beacon module contracts.
type Handle interface {
	sortition.Handle
}