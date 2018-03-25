package compute

import (
	"math/big"

	"github.com/republicprotocol/go-do"
	"github.com/republicprotocol/republic-go/network/rpc"
	"github.com/republicprotocol/republic-go/order"
)

type DeltaBuilder struct {
	do.GuardedObject

	k                      int64
	prime                  *big.Int
	deltas                 map[string]*Delta
	deltaFragments         map[string]*DeltaFragment
	deltasToDeltaFragments map[string][]*DeltaFragment
	newDeltaFragment       chan *DeltaFragment
}

func NewDeltaBuilder(k int64, prime *big.Int) *DeltaBuilder {
	return &DeltaBuilder{
		GuardedObject:          do.NewGuardedObject(),
		k:                      k,
		prime:                  prime,
		deltas:                 map[string]*Delta{},
		deltaFragments:         map[string]*DeltaFragment{},
		deltasToDeltaFragments: map[string][]*DeltaFragment{},
		newDeltaFragment:       make(chan *DeltaFragment, 100),
	}
}

func (builder *DeltaBuilder) UnconfirmedOrders() chan *rpc.Order {
	orders := make(chan *rpc.Order, 100)
	sentOrders := map[string]bool{}
	go func() {
		builder.EnterReadOnly(nil)
		builder.Exit()

		for _, delta := range builder.deltas {
			buyOrder := new(rpc.Order)
			buyOrder.Id = delta.BuyOrderID
			buyOrder.Parity = int64(order.ParityBuy)
			if _, ok := sentOrders[string(delta.BuyOrderID)]; !ok {
				orders <- buyOrder
				sentOrders[string(delta.BuyOrderID)] = true
			}

			sellOrder := new(rpc.Order)
			sellOrder.Id = delta.SellOrderID
			sellOrder.Parity = int64(order.ParitySell)
			if _, ok := sentOrders[string(delta.SellOrderID)]; !ok {
				orders <- sellOrder
				sentOrders[string(delta.SellOrderID)] = true
			}
		}

		for delta := range builder.newDeltaFragment {
			buyOrder := new(rpc.Order)
			buyOrder.Id = delta.BuyOrderID
			buyOrder.Parity = int64(order.ParityBuy)
			if _, ok := sentOrders[string(delta.BuyOrderID)]; !ok {
				orders <- buyOrder
				sentOrders[string(delta.BuyOrderID)] = true
			}

			sellOrder := new(rpc.Order)
			sellOrder.Id = delta.SellOrderID
			sellOrder.Parity = int64(order.ParitySell)
			if _, ok := sentOrders[string(delta.SellOrderID)]; !ok {
				orders <- sellOrder
				sentOrders[string(delta.SellOrderID)] = true
			}
		}
	}()

	return orders
}

func (builder *DeltaBuilder) InsertDeltaFragment(deltaFragment *DeltaFragment) *Delta {
	if len(builder.newDeltaFragment) == 100{
		<- builder.newDeltaFragment
	}
	builder.newDeltaFragment<- deltaFragment

	builder.Enter(nil)
	defer builder.Exit()

	return builder.insertDeltaFragment(deltaFragment)
}

func (builder *DeltaBuilder) insertDeltaFragment(deltaFragment *DeltaFragment) *Delta {
	// Is the delta already built, or are we adding a delta fragment that we
	// have already seen
	if builder.hasDelta(deltaFragment.DeltaID) {
		return nil // Only return new deltas
	}
	if builder.hasDeltaFragment(deltaFragment.ID) {
		return nil // Only return new deltas
	}

	// Add the delta fragment to the builder and attach it to the appropriate
	// delta
	builder.deltaFragments[string(deltaFragment.ID)] = deltaFragment
	if deltaFragments, ok := builder.deltasToDeltaFragments[string(deltaFragment.DeltaID)]; ok {
		deltaFragments = append(deltaFragments, deltaFragment)
		builder.deltasToDeltaFragments[string(deltaFragment.DeltaID)] = deltaFragments
	} else {
		builder.deltasToDeltaFragments[string(deltaFragment.DeltaID)] = []*DeltaFragment{deltaFragment}
	}

	// Build the delta if possible and return it
	deltaFragments := builder.deltasToDeltaFragments[string(deltaFragment.DeltaID)]
	if int64(len(deltaFragments)) >= builder.k {
		delta := NewDelta(deltaFragments, builder.prime)
		if delta == nil {
			return nil
		}
		builder.deltas[string(delta.ID)] = delta
		return delta
	}

	return nil
}

func (builder *DeltaBuilder) HasDelta(deltaID DeltaID) bool {
	builder.EnterReadOnly(nil)
	defer builder.ExitReadOnly()
	return builder.hasDelta(deltaID)
}

func (builder *DeltaBuilder) hasDelta(deltaID DeltaID) bool {
	_, ok := builder.deltas[string(deltaID)]
	return ok
}

func (builder *DeltaBuilder) HasDeltaFragment(deltaFragmentID DeltaFragmentID) bool {
	builder.EnterReadOnly(nil)
	defer builder.ExitReadOnly()
	return builder.hasDeltaFragment(deltaFragmentID)
}

func (builder *DeltaBuilder) hasDeltaFragment(deltaFragmentID DeltaFragmentID) bool {
	_, ok := builder.deltaFragments[string(deltaFragmentID)]
	return ok
}

func (builder *DeltaBuilder) SetK(k int64) {
	builder.Enter(nil)
	defer builder.Exit()
	builder.setK(k)
}

func (builder *DeltaBuilder) setK(k int64) {
	builder.k = k
}

type DeltaFragmentMatrix struct {
	do.GuardedObject

	prime                 *big.Int
	buyOrderFragments     map[string]*order.Fragment
	sellOrderFragments    map[string]*order.Fragment
	buySellDeltaFragments map[string]map[string]*DeltaFragment
	newFragment           chan *order.Fragment
}

func NewDeltaFragmentMatrix(prime *big.Int) *DeltaFragmentMatrix {
	return &DeltaFragmentMatrix{
		GuardedObject:         do.NewGuardedObject(),
		prime:                 prime,
		buyOrderFragments:     map[string]*order.Fragment{},
		sellOrderFragments:    map[string]*order.Fragment{},
		buySellDeltaFragments: map[string]map[string]*DeltaFragment{},
		newFragment:           make(chan *order.Fragment, 100),
	}
}

func (matrix *DeltaFragmentMatrix) OpenOrders() chan *rpc.Order{
	matrix.EnterReadOnly(nil)
	defer matrix.Exit()

	openOrders := make (chan *rpc.Order, 100)
	go func() {
		for orderId, orderFragment  := range matrix.buyOrderFragments{
			buyOrder := new(rpc.Order)
			buyOrder.Id = []byte(orderId)
			buyOrder.Parity = int64(order.ParityBuy)
			buyOrder.Expiry = orderFragment.OrderExpiry.Unix()

			openOrders <- buyOrder
		}

		for orderId, orderFragment  := range matrix.sellOrderFragments{
			sellOrder := new(rpc.Order)
			sellOrder.Id = []byte(orderId)
			sellOrder.Parity = int64(order.ParitySell)
			sellOrder.Expiry = orderFragment.OrderExpiry.Unix()

			openOrders <- sellOrder
		}

		for fragment := range matrix.newFragment {
			if _, ok := matrix.buyOrderFragments[string(fragment.OrderID)]; fragment.OrderParity == order.ParityBuy && !ok{
				ord := new(rpc.Order)
				ord.Id = fragment.OrderID
				ord.Parity = int64(fragment.OrderParity)
				ord.Expiry = fragment.OrderExpiry.Unix()

				openOrders <- ord
			}

			if _, ok := matrix.sellOrderFragments[string(fragment.OrderID)]; fragment.OrderParity == order.ParitySell && !ok{
				ord := new(rpc.Order)
				ord.Id = fragment.OrderID
				ord.Parity = int64(fragment.OrderParity)
				ord.Expiry = fragment.OrderExpiry.Unix()

				openOrders <- ord
			}
		}
	}()

	return openOrders
}


func (matrix *DeltaFragmentMatrix) BuyOrders() []*order.Order {
	matrix.EnterReadOnly(nil)
	defer matrix.Exit()

	res := make([]*order.Order, len(matrix.buyOrderFragments))
	index := 0
	for _, fragment := range matrix.buyOrderFragments {
		buyOrder := new(order.Order)
		buyOrder.ID = fragment.OrderID
		res[index] = buyOrder
		index++
	}
	return res
}

func (matrix *DeltaFragmentMatrix) SellOrders() []*order.Order {
	matrix.EnterReadOnly(nil)
	defer matrix.Exit()

	res := make([]*order.Order, len(matrix.sellOrderFragments))
	index := 0
	for _, fragment := range matrix.sellOrderFragments {
		sellOrder := new(order.Order)
		sellOrder.ID = fragment.OrderID
		res[index] = sellOrder
		index++
	}
	return res
}

func (matrix *DeltaFragmentMatrix) InsertOrderFragment(orderFragment *order.Fragment) ([]*DeltaFragment, error) {
	matrix.Enter(nil)
	defer matrix.Exit()
	if orderFragment.OrderParity == order.ParityBuy {
		return matrix.insertBuyOrderFragment(orderFragment)
	}
	return matrix.insertSellOrderFragment(orderFragment)
}

func (matrix *DeltaFragmentMatrix) insertBuyOrderFragment(buyOrderFragment *order.Fragment) ([]*DeltaFragment, error) {
	if _, ok := matrix.buyOrderFragments[string(buyOrderFragment.OrderID)]; ok {
		return []*DeltaFragment{}, nil
	}

	deltaFragments := make([]*DeltaFragment, 0, len(matrix.sellOrderFragments))
	deltaFragmentsMap := map[string]*DeltaFragment{}
	for i := range matrix.sellOrderFragments {
		deltaFragment := NewDeltaFragment(buyOrderFragment, matrix.sellOrderFragments[i], matrix.prime)
		if deltaFragment == nil {
			continue
		}
		deltaFragments = append(deltaFragments, deltaFragment)
		deltaFragmentsMap[string(matrix.sellOrderFragments[i].OrderID)] = deltaFragment
	}

	matrix.buyOrderFragments[string(buyOrderFragment.OrderID)] = buyOrderFragment
	matrix.buySellDeltaFragments[string(buyOrderFragment.OrderID)] = deltaFragmentsMap
	return deltaFragments, nil
}

func (matrix *DeltaFragmentMatrix) insertSellOrderFragment(sellOrderFragment *order.Fragment) ([]*DeltaFragment, error) {
	if _, ok := matrix.sellOrderFragments[string(sellOrderFragment.OrderID)]; ok {
		return []*DeltaFragment{}, nil
	}

	deltaFragments := make([]*DeltaFragment, 0, len(matrix.buyOrderFragments))
	for i := range matrix.buyOrderFragments {
		deltaFragment := NewDeltaFragment(matrix.buyOrderFragments[i], sellOrderFragment, matrix.prime)
		if deltaFragment == nil {
			continue
		}
		if _, ok := matrix.buySellDeltaFragments[string(matrix.buyOrderFragments[i].OrderID)]; ok {
			deltaFragments = append(deltaFragments, deltaFragment)
			matrix.buySellDeltaFragments[string(matrix.buyOrderFragments[i].OrderID)][string(sellOrderFragment.OrderID)] = deltaFragment
		}
	}

	matrix.sellOrderFragments[string(sellOrderFragment.OrderID)] = sellOrderFragment
	return deltaFragments, nil
}

func (matrix *DeltaFragmentMatrix) RemoveOrderFragment(orderID order.ID) error {
	matrix.Enter(nil)
	defer matrix.Exit()
	if err := matrix.removeBuyOrderFragment(orderID); err != nil {
		return err
	}
	return matrix.removeSellOrderFragment(orderID)
}

func (matrix *DeltaFragmentMatrix) removeBuyOrderFragment(buyOrderID order.ID) error {
	if _, ok := matrix.buyOrderFragments[string(buyOrderID)]; !ok {
		return nil
	}

	delete(matrix.buyOrderFragments, string(buyOrderID))
	delete(matrix.buySellDeltaFragments, string(buyOrderID))

	return nil
}

func (matrix *DeltaFragmentMatrix) removeSellOrderFragment(sellOrderID order.ID) error {
	if _, ok := matrix.sellOrderFragments[string(sellOrderID)]; !ok {
		return nil
	}

	for i := range matrix.buySellDeltaFragments {
		delete(matrix.buySellDeltaFragments[i], string(sellOrderID))
	}

	return nil
}
