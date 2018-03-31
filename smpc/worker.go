package smpc

import (
	"context"
	"time"

	"github.com/republicprotocol/republic-go/order"
)

// OrderFragmentReceiver receives order Fragments from an input channel and
// uses a DeltaFragmentMatrix to compute new DeltaFragments with them.
// Cancelling the context will shutdown the DeltaFragmentReader.It returns an
// error, or nil.
func OrderFragmentReceiver(ctx context.Context, orderFragments chan order.Fragment, matrix *DeltaFragmentMatrix) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case orderFragment := <-orderFragments:
			if orderFragment.OrderParity == order.ParityBuy {
				matrix.ComputeBuyOrder(&orderFragment)
			} else {
				matrix.ComputeSellOrder(&orderFragment)
			}
		}
	}
}

// DeltaFragmentReceiver receives DeltaFragments from an input cahnnel and uses
// a DeltaBuilder to build new Deltas with them. Cancelling the context will
// shutdown the DeltaFragmentReader. It returns an error, or nil.
func DeltaFragmentReceiver(ctx context.Context, deltaFragments chan DeltaFragment, builder *DeltaBuilder) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case deltaFragment := <-deltaFragments:
			builder.ComputeDelta(DeltaFragments{deltaFragment})
		}
	}
}

// DeltaFragmentBroadcaster reads DeltaFragments from the DeltaFragmentMatrix
// and writes them to an output channel. It can be used for broadcasting
// DeltaFragments locally, and remotely. Cancelling the context will shutdown
// the DeltaFragmentReader. Errors are written to an error channel
func DeltaFragmentBroadcaster(ctx context.Context, matrix *DeltaFragmentMatrix, bufferLimit int) (chan DeltaFragment, chan error) {
	deltaFragments := make(chan DeltaFragment, bufferLimit)
	errors := make(chan error, bufferLimit)

	go func() {
		defer close(deltaFragments)
		defer close(errors)

		buffer := make(DeltaFragments, bufferLimit)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				errors <- ctx.Err()
				return
			case <-ticker.C:
				for i, n := 0, matrix.WaitForDeltaFragments(buffer[:]); i < n; i++ {
					deltaFragments <- buffer[i]
				}
			}
		}
	}()

	return deltaFragments, errors
}

// DeltaBroadcaster reads Deltas from the DeltaBuilder and writes them to an
// output channel. Cancelling the context will shutdown the DeltaBroadcaster.
// Errors are written to an error channel
func DeltaBroadcaster(ctx context.Context, builder *DeltaBuilder, bufferLimit int) (chan Delta, chan error) {
	deltas := make(chan Delta, bufferLimit)
	errors := make(chan error, bufferLimit)

	go func() {
		defer close(deltas)
		defer close(errors)

		buffer := make(Deltas, bufferLimit)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				errors <- ctx.Err()
				return
			case <-ticker.C:
				for i, n := 0, builder.WaitForDeltas(buffer[:]); i < n; i++ {
					deltas <- buffer[i]
				}
			}
		}
	}()

	return deltas, errors
}
