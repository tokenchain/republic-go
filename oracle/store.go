package oracle

type MidpointPrices map[uint64]uint64

// MidpointPriceStorer is used for retrieving/storing MidpointPrice.
type MidpointPriceStorer interface {

	// PutMidpointPrice stores the given midPointPrice into the storer.
	// It doesn't do any nonce check and will always overwrite previous record.
	PutMidpointPrice(MidpointPrice)

	// Nonce returns the current nonce of the mid-point prices.
	Nonce() uint64

	// MidpointPrice returns the mid-point prices of all stored token-pairs.
	MidpointPrice() MidpointPrices
}
