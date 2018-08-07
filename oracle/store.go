package oracle

import (
	"errors"

	"github.com/republicprotocol/republic-go/order"
)

// ErrMidpointPriceNotFound is returned when attempting to read a midpoint
// price that cannot be found.
var ErrMidpointPriceNotFound = errors.New("midpoint price not found")

// MidpointPriceStorer is used for retrieving/storing MidpointPrice.
type MidpointPriceStorer interface {

	// PutMidpointPrice stores the given midPointPrice into the storer.
	// It doesn't do any nonce check and will always overwrite previous record.
	PutMidpointPrice(MidpointPrice) error

	// Nonce returns the current nonce of the mid-point prices.
	Nonce() (uint64, error)

	// MidpointPrice returns the mid-point prices of all stored token-pairs.
	MidpointPrice(tokens order.Tokens) (uint64, error)
}
