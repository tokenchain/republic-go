package ome

import (
	"errors"
	"fmt"
	"log"
	"math"
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
	ResolveStageMidpointBuyPriceExp
	ResolveStageMidpointBuyPriceCo
	ResolveStageMidpointSellPriceExp
	ResolveStageMidpointSellPriceCo
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
	case ResolveStageMidpointBuyPriceExp:
		return "midpointBuyPriceExp"
	case ResolveStageMidpointBuyPriceCo:
		return "midpointBuyPriceCo"
	case ResolveStageMidpointSellPriceExp:
		return "midpointSellPriceExp"
	case ResolveStageMidpointSellPriceCo:
		return "midpointSellPriceCo"
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
		if err := matcher.updateMidpointData(&com); err != nil {
			logger.Compute(logger.LevelError, err.Error())
			callback(com)
			return
		}
		if isMidpointOrder(com.Buy) {
			matcher.resolve(smpc.NetworkID(com.Epoch), com, callback, ResolveStageMidpointBuyPriceExp)
			return
		}
		matcher.resolve(smpc.NetworkID(com.Epoch), com, callback, ResolveStageMidpointSellPriceExp)
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
	if len(values) != 1 {
		logger.Compute(logger.LevelError, fmt.Sprintf("cannot resolve %v: unexpected number of values: %v", stage, len(values)))
		return
	}

	switch stage {
	case ResolveStageMidpointBuyPriceExp:
		if isGreaterThanZero(values[0]) {
			com.Buy.Price = com.MidpointPrice
			if isMidpointOrder(com.Sell) {
				matcher.resolve(networkID, com, callback, stage+2)
				return
			}
			matcher.resolve(networkID, com, callback, stage+4)
			return
		}
		if isEqualToZero(values[0]) {
			matcher.resolve(networkID, com, callback, stage+1)
			return
		}

	case ResolveStageMidpointBuyPriceCo:
		if isGreaterThanOrEqualToZero(values[0]) {
			com.Buy.Price = com.MidpointPrice
			if isMidpointOrder(com.Sell) {
				matcher.resolve(networkID, com, callback, stage+1)
				return
			}
			matcher.resolve(networkID, com, callback, stage+3)
			return
		}

	case ResolveStageMidpointSellPriceExp:
		if isGreaterThanZero(values[0]) {
			com.Sell.Price = com.MidpointPrice
			if isMidpointOrder(com.Buy) {
				matcher.resolve(networkID, com, callback, stage+4)
				return
			}
			matcher.resolve(networkID, com, callback, stage+2)
			return
		}
		if isEqualToZero(values[0]) {
			matcher.resolve(networkID, com, callback, stage+1)
			return
		}

	case ResolveStageMidpointSellPriceCo:
		if isGreaterThanOrEqualToZero(values[0]) {
			com.Sell.Price = com.MidpointPrice
			if isMidpointOrder(com.Buy) {
				matcher.resolve(networkID, com, callback, stage+3)
				return
			}
			matcher.resolve(networkID, com, callback, stage+1)
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

// updateMidpointData will retrieve the mid-point price and nonce from the
// midpointPriceStore and update computation with the new data.
func (matcher *matcher) updateMidpointData(com *Computation) error {
	// midpointPrice, err := matcher.midpointPriceStore.MidpointPrice(com.Buy.Tokens)
	// if err != nil {
	// 	return fmt.Errorf("cannot retrieve midpoint price for buy = %v, sell = %v", com.Buy.OrderID, com.Sell.OrderID)
	// }
	// midpointNonce, err := matcher.midpointPriceStore.Nonce()
	// if err != nil {
	// 	return fmt.Errorf("cannot retrieve midpoint nonce for buy = %v, sell = %v", com.Buy.OrderID, com.Sell.OrderID)
	// }
	// midpointPriceCoExp := PriceFloatToCoExp(float64(midpointPrice))
	// com.MidpointPrice = midpointPriceCoExp
	// com.MidpointNonce = midpointNonce
	return nil
}

func buildJoin(com Computation, stage ResolveStage) (smpc.Join, error) {
	var share shamir.Share

	switch stage {
	case ResolveStageMidpointBuyPriceExp:
		share = com.MidpointPrice.Exp.Sub(&com.Buy.Price.Exp)

	case ResolveStageMidpointBuyPriceCo:
		share = com.MidpointPrice.Co.Sub(&com.Buy.Price.Co)

	case ResolveStageMidpointSellPriceExp:
		share = com.Sell.Price.Exp.Sub(&com.MidpointPrice.Exp)

	case ResolveStageMidpointSellPriceCo:
		share = com.Sell.Price.Co.Sub(&com.MidpointPrice.Exp)

	case ResolveStagePriceExp:
		share = com.Buy.Price.Exp.Sub(&com.Sell.Price.Exp)

	case ResolveStagePriceCo:
		share = com.Buy.Price.Co.Sub(&com.Sell.Price.Co)

	case ResolveStageBuyVolumeExp:
		share = com.Buy.Volume.Exp.Sub(&com.Sell.MinimumVolume.Exp)

	case ResolveStageBuyVolumeCo:
		share = com.Buy.Volume.Co.Sub(&com.Sell.MinimumVolume.Co)

	case ResolveStageSellVolumeExp:
		share = com.Sell.Volume.Exp.Sub(&com.Buy.MinimumVolume.Exp)

	case ResolveStageSellVolumeCo:
		share = com.Sell.Volume.Co.Sub(&com.Buy.MinimumVolume.Co)

	case ResolveStageTokens:
		share = com.Buy.Tokens.Sub(&com.Sell.Tokens)
	default:
		return smpc.Join{}, ErrUnexpectedResolveStage
	}
	join := smpc.Join{
		Index:  smpc.JoinIndex(share.Index),
		Shares: []shamir.Share{share},
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

// PriceFloatToCoExp converts a float number price to a CoExp format.
func PriceFloatToCoExp(price float64) order.CoExp {
	if price > 10 {
		prev := PriceFloatToCoExp(price / 10)
		return order.CoExp{
			Co:  prev.Co,
			Exp: prev.Exp + 1,
		}
	} else if price < 0.005 {
		prev := PriceFloatToCoExp(price * 10)
		return order.CoExp{
			Co:  prev.Co,
			Exp: prev.Exp - 1,
		}
	} else {
		if price == 0 {
			return order.CoExp{
				Co:  0,
				Exp: 0,
			}
		}
		if price < 1 {
			prev := PriceFloatToCoExp(price * 10)
			return order.CoExp{
				Co:  prev.Co,
				Exp: prev.Exp - 1,
			}
		}
	}
	try := math.Round(price / 0.005)
	return order.CoExp{
		Co:  uint64(try),
		Exp: 38,
	}
}
