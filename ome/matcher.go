package ome

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/republicprotocol/republic-go/logger"
	"github.com/republicprotocol/republic-go/oracle"
	"github.com/republicprotocol/republic-go/order"
	"github.com/republicprotocol/republic-go/shamir"
	"github.com/republicprotocol/republic-go/smpc"
)

// ErrUnexpectedResolveStage is returned when a ResolveStage is not one of the
// explicitly enumerated values.
var ErrUnexpectedResolveStage = errors.New("unexpected resolve stage")

// ResolveStage defines the various stages that resolving can be in for any
// given Computation.
type ResolveStage byte

// Values for ResolveStage.
const (
	ResolveStageNil ResolveStage = iota
	ResolveStageMidpointPriceExp
	ResolveStageMidpointPriceCo
	ResolveStagePriceExp
	ResolveStagePriceCo
	ResolveStageBuyVolumeExp
	ResolveStageBuyVolumeCo
	ResolveStageSellVolumeExp
	ResolveStageSellVolumeCo
	ResolveStageTokens
	ResolveStageSettlement
)

// String returns the human-readable representation of a ResolveStage.
func (stage ResolveStage) String() string {
	switch stage {
	case ResolveStageNil:
		return "nil"
	case ResolveStageMidpointPriceExp:
		return "midpointPriceExp"
	case ResolveStageMidpointPriceCo:
		return "midpointPriceCo"
	case ResolveStagePriceExp:
		return "priceExp"
	case ResolveStagePriceCo:
		return "priceCo"
	case ResolveStageBuyVolumeExp:
		return "buyVolumeExp"
	case ResolveStageBuyVolumeCo:
		return "buyVolumeCo"
	case ResolveStageSellVolumeExp:
		return "sellVolumeExp"
	case ResolveStageSellVolumeCo:
		return "sellVolumeCo"
	case ResolveStageTokens:
		return "tokens"
	}
	return ""
}

// A MatchCallback is called when a Computation is finished. The Computation
// can then be inspected to determine if the result is a match.
type MatchCallback func(Computation)

// A Matcher resolves Computations into a matched, or mismatched, result.
type Matcher interface {

	// Resolve a Computation to determine whether or not the orders involved
	// are a match. The epoch hash of the Computation and is used to
	// differentiate between the various networks required for SMPC. The
	// MatchCallback is called when a result has be determined.
	Resolve(com Computation, callback MatchCallback)
}

type matcher struct {
	computationStore   ComputationStorer
	midpointPriceStore oracle.MidpointPriceStorer
	smpcer             smpc.Smpcer
}

// NewMatcher returns a Matcher that will resolve Computations by resolving
// each component in a pipeline. If a mismatch is encountered at any stage of
// the pipeline, the Computation is short circuited and the MatchCallback will
// be called immediately.
func NewMatcher(computationStore ComputationStorer, smpcer smpc.Smpcer, midpointPriceStore oracle.MidpointPriceStorer) Matcher {
	return &matcher{
		computationStore:   computationStore,
		midpointPriceStore: midpointPriceStore,
		smpcer:             smpcer,
	}
}

// Resolve implements the Matcher interface.
func (matcher *matcher) Resolve(com Computation, callback MatchCallback) {
	if com.Buy.OrderSettlement != com.Sell.OrderSettlement {
		// Store the computation as a mismatch
		com.State = ComputationStateMismatched
		com.Match = false
		if err := matcher.computationStore.PutComputation(com); err != nil {
			logger.Compute(logger.LevelError, fmt.Sprintf("cannot store mismatched computation buy = %v, sell = %v", com.Buy.OrderID, com.Sell.OrderID))
		}
		// Trigger the callback with a mismatch
		logger.Compute(logger.LevelDebug, fmt.Sprintf("✗ settlement => buy = %v, sell = %v", com.Buy.OrderID, com.Sell.OrderID))
		callback(com)
		return
	}

	// If either of the fragments is a mid-point order, proceed to resolve
	// mid-point prices.
	if isMidpointOrder(com.Buy) || isMidpointOrder(com.Sell) {
		// TODO: get midpoint from store and convert to a Share and store it
		// in the computation.
		matcher.resolve(smpc.NetworkID(com.Epoch), com, callback, ResolveStageMidpointPriceExp)
		return
	}
	matcher.resolve(smpc.NetworkID(com.Epoch), com, callback, ResolveStagePriceExp)
}

func (matcher *matcher) resolve(networkID smpc.NetworkID, com Computation, callback MatchCallback, stage ResolveStage) {
	if isExpired(com) {
		com.State = ComputationStateRejected
		if err := matcher.computationStore.PutComputation(com); err != nil {
			logger.Error(fmt.Sprintf("cannot store expired computation buy = %v, sell = %v: %v", com.Buy.OrderID, com.Sell.OrderID, err))
		}
		return
	}

	join, err := buildJoin(com, stage)
	if err != nil {
		logger.Compute(logger.LevelError, fmt.Sprintf("cannot build %v join: %v", stage, err))
		return
	}
	err = matcher.smpcer.Join(networkID, join, func(joinID smpc.JoinID, values []uint64) {
		matcher.resolveValues(values, networkID, com, callback, stage)
	})
	if err != nil {
		logger.Compute(logger.LevelError, fmt.Sprintf("cannot resolve %v: cannot join computation = %v: %v", stage, com.ID, err))
	}
}

func (matcher *matcher) resolveValues(values []uint64, networkID smpc.NetworkID, com Computation, callback MatchCallback, stage ResolveStage) {
	if len(values) < 1 {
		logger.Compute(logger.LevelError, fmt.Sprintf("cannot resolve %v: unexpected number of values: %v", stage, len(values)))
		return
	}

	switch stage {
	case ResolveStageMidpointPriceExp:
		// nextStage := ResolveStageNil

		// for _, value := range values {
		// 	if !isGreaterThanOrEqualToZero(value) {

		// 	}
		// 	if isGreaterThanZero(value) {
		// 		nextStage = stage + 2
		// 		if isMidpointOrder(com.Buy) && isMidpointOrder(com.Sell) {
		// 			nextStage = stage + 6
		// 		}
		// 		matcher.resolve(networkID, com, callback, nextStage)
		// 		return
		// 	}
		// 	if isEqualToZero(value) {
		// 		nextStage = stage + 1
		// 		matcher.resolve(networkID, com, callback, stage+1)
		// 		return
		// 	}
		// }

	case ResolveStageMidpointPriceCo:
		if isGreaterThanOrEqualToZero(values[0]) {
			nextStage := stage + 1
			if isMidpointOrder(com.Buy) && isMidpointOrder(com.Sell) {
				nextStage = stage + 3
			}
			matcher.resolve(networkID, com, callback, nextStage)
			return
		}

	case ResolveStagePriceExp, ResolveStageBuyVolumeExp, ResolveStageSellVolumeExp:
		if isGreaterThanZero(values[0]) {
			matcher.resolve(networkID, com, callback, stage+2)
			return
		}
		if isEqualToZero(values[0]) {
			matcher.resolve(networkID, com, callback, stage+1)
			return
		}

	case ResolveStagePriceCo, ResolveStageBuyVolumeCo, ResolveStageSellVolumeCo:
		if isGreaterThanOrEqualToZero(values[0]) {
			matcher.resolve(networkID, com, callback, stage+1)
			return
		}

	case ResolveStageTokens:
		if isEqualToZero(values[0]) {
			// Store the computation as a match
			com.State = ComputationStateMatched
			com.Match = true
			if err := matcher.computationStore.PutComputation(com); err != nil {
				logger.Compute(logger.LevelError, fmt.Sprintf("cannot store matched computation buy = %v, sell = %v", com.Buy.OrderID, com.Sell.OrderID))
			}

			// Trigger the callback with a match
			callback(com)
			return
		}

	default:
		// If the stage is unknown it is always considered a mismatch
	}

	// Store the computation as a mismatch
	com.State = ComputationStateMismatched
	com.Match = false
	if err := matcher.computationStore.PutComputation(com); err != nil {
		logger.Compute(logger.LevelError, fmt.Sprintf("cannot store mismatched computation buy = %v, sell = %v", com.Buy.OrderID, com.Sell.OrderID))
	}

	// Trigger the callback with a mismatch
	log.Printf("[debug] (%v) ✗ buy = %v, sell = %v", stage, com.Buy.OrderID, com.Sell.OrderID)
	callback(com)
}

func buildJoin(com Computation, stage ResolveStage) (smpc.Join, error) {
	var shares []shamir.Share

	switch stage {
	case ResolveStageMidpointPriceExp:
		if isMidpointOrder(com.Buy) {
			shares = append(shares, com.MidpointPrice.Exp.Sub(&com.Buy.Price.Exp))
		}
		if isMidpointOrder(com.Sell) {
			shares = append(shares, com.Sell.Price.Exp.Sub(&com.MidpointPrice.Exp))
		}

	case ResolveStageMidpointPriceCo:
		if isMidpointOrder(com.Buy) {
			shares = append(shares, com.MidpointPrice.Co.Sub(&com.Buy.Price.Co))
		}
		if isMidpointOrder(com.Sell) {
			shares = append(shares, com.Sell.Price.Co.Sub(&com.MidpointPrice.Exp))
		}

	case ResolveStagePriceExp:
		shares = append(shares, com.Buy.Price.Exp.Sub(&com.Sell.Price.Exp))

	case ResolveStagePriceCo:
		shares = append(shares, com.Buy.Price.Co.Sub(&com.Sell.Price.Co))

	case ResolveStageBuyVolumeExp:
		shares = append(shares, com.Buy.Volume.Exp.Sub(&com.Sell.MinimumVolume.Exp))

	case ResolveStageBuyVolumeCo:
		shares = append(shares, com.Buy.Volume.Co.Sub(&com.Sell.MinimumVolume.Co))

	case ResolveStageSellVolumeExp:
		shares = append(shares, com.Sell.Volume.Exp.Sub(&com.Buy.MinimumVolume.Exp))

	case ResolveStageSellVolumeCo:
		shares = append(shares, com.Sell.Volume.Co.Sub(&com.Buy.MinimumVolume.Co))

	case ResolveStageTokens:
		shares = append(shares, com.Buy.Tokens.Sub(&com.Sell.Tokens))
	default:
		return smpc.Join{}, ErrUnexpectedResolveStage
	}
	join := smpc.Join{
		Index:  smpc.JoinIndex(shares[0].Index),
		Shares: shares,
	}
	copy(join.ID[:], com.ID[:])
	join.ID[32] = byte(stage)
	return join, nil
}

func isGreaterThanOrEqualToZero(value uint64) bool {
	return value >= 0 && value < shamir.Prime/2
}

func isGreaterThanZero(value uint64) bool {
	return value > 0 && value < shamir.Prime/2
}

func isEqualToZero(value uint64) bool {
	return value == 0 || value == shamir.Prime
}

func isExpired(com Computation) bool {
	if time.Now().After(com.Buy.OrderExpiry) || time.Now().After(com.Sell.OrderExpiry) {
		logger.Compute(logger.LevelDebug, fmt.Sprintf("⧖ expired => buy = %v, sell = %v", com.Buy.OrderID, com.Sell.OrderID))
		return true
	}
	return false
}

func isMidpointOrder(fragment order.Fragment) bool {
	return fragment.OrderType == order.TypeMidpoint || fragment.OrderType == order.TypeMidpointFOK
}

func isMidpointPriceOutOfRange(fragment order.Fragment, midpointPriceStore oracle.MidpointPriceStorer) bool {
	// if fragment.OrderParity == order.ParityBuy && fragment.Price > midpointPriceStore.MidpointPrice()[fragment.Tokens] {
	// 	return false
	// }
	// if fragment.OrderParity == order.ParitySell && fragment.Price > midpointPriceStore.MidpointPrice() {
	// 	return false
	// }
	return true
}
