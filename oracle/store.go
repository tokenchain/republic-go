package oracle

import (
	"errors"
	"sync"
)

// ErrCursorOutOfRange is returned when an iterator cursor is used to read a
// value outside the range of the iterator.
var ErrCursorOutOfRange = errors.New("cursor out of range")

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

// midpointPriceStorer implements MidpointPriceStorer interface with an
// in-memory map implementation. Data will be lost every time it restarts.
type midpointPriceStorer struct {
	mu     *sync.Mutex
	prices MidpointPrice
}

// NewMidpointPriceStorer returns a new MidpointPriceStorer.
func NewMidpointPriceStorer() MidpointPriceStorer {
	return &midpointPriceStorer{
		mu:     new(sync.Mutex),
		prices: MidpointPrice{},
	}
}

// PutMidpointPrice implements the MidpointPriceStorer interface.
func (storer *midpointPriceStorer) PutMidpointPrice(midPointPrice MidpointPrice) {
	storer.mu.Lock()
	defer storer.mu.Unlock()

	storer.prices = midPointPrice
}

func (storer *midpointPriceStorer) Nonce() uint64 {
	storer.mu.Lock()
	defer storer.mu.Unlock()

	return storer.prices.Nonce
}

// MidpointPrice implements the MidpointPriceStorer interface.
func (storer *midpointPriceStorer) MidpointPrice() MidpointPrices {
	storer.mu.Lock()
	defer storer.mu.Unlock()

	midPointPrices := map[uint64]uint64{}
	for i, tokenPair := range storer.prices.TokenPairs {
		midPointPrices[tokenPair] = storer.prices.Prices[i]
	}
	return midPointPrices
}
