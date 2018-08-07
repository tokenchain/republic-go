package leveldb

import (
	"sync"

	"github.com/republicprotocol/republic-go/oracle"
	"github.com/republicprotocol/republic-go/order"
)

// midpointPriceStorer implements MidpointPriceStorer interface with an
// in-memory map implementation. Data will be lost every time it restarts.
type midpointPriceStorer struct {
	mu     *sync.Mutex
	prices map[order.Tokens]uint64
	nonce  uint64
}

// NewMidpointPriceStorer returns a new MidpointPriceStorer.
func NewMidpointPriceStorer() oracle.MidpointPriceStorer {
	return &midpointPriceStorer{
		mu:     new(sync.Mutex),
		prices: map[order.Tokens]uint64{},
		nonce:  0,
	}
}

// PutMidpointPrice implements the MidpointPriceStorer interface.
func (storer *midpointPriceStorer) PutMidpointPrice(midPointPrice oracle.MidpointPrice) error {
	storer.mu.Lock()
	defer storer.mu.Unlock()

	for i, token := range midPointPrice.TokenPairs {
		storer.prices[order.Tokens(token)] = midPointPrice.Prices[i]
	}
	storer.nonce = midPointPrice.Nonce
	return nil
}

// MidpointPrice implements the MidpointPriceStorer interface.
func (storer *midpointPriceStorer) MidpointPrice(token order.Tokens) (uint64, error) {
	storer.mu.Lock()
	defer storer.mu.Unlock()

	if price, ok := storer.prices[token]; ok {
		return price, nil
	}
	return 0, oracle.ErrMidpointPriceNotFound
}

// Nonce implements the MidpointPriceStorer interface.
func (storer *midpointPriceStorer) Nonce() (uint64, error) {
	storer.mu.Lock()
	defer storer.mu.Unlock()

	return storer.nonce, nil
}
