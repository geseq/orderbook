package orderbook

import (
	decimal "github.com/geseq/udecimal"
)

//go:generate gotemplate "github.com/geseq/redblacktree" Tree(udecimal.Decimal,*OrderQueue)

// PriceLevel implements facade to operations with order queue
type PriceLevel struct {
	priceTree *Tree
	priceType PriceType

	volume    decimal.Decimal
	numOrders uint64
	depth     int
}

// Comparator compares two Decimal objects
func Comparator(a, b decimal.Decimal) int {
	return a.Cmp(b)
}

// PriceType represents the type of price levels
type PriceType byte

const (
	// BidPrice is for big orders
	BidPrice PriceType = iota

	// AskPrice is for ask orders
	AskPrice

	// StopPrice is for stop loss orders
	StopPrice
)

// NewPriceLevel creates new PriceLevel manager
func NewPriceLevel(priceType PriceType) *PriceLevel {
	return &PriceLevel{
		priceTree: NewWithTree(Comparator),
		priceType: priceType,
		volume:    decimal.Zero,
	}
}

// Len returns amount of orders
func (pl *PriceLevel) Len() uint64 {
	return pl.numOrders
}

// Depth returns depth of market
func (pl *PriceLevel) Depth() int {
	return pl.depth
}

// Volume returns total amount of quantity in side
func (pl *PriceLevel) Volume() decimal.Decimal {
	return pl.volume
}

// Append appends order to definite price level
func (pl *PriceLevel) Append(o *Order) *Order {
	price := o.GetPrice(pl.priceType)

	priceQueue, ok := pl.priceTree.Get(price)
	if !ok {
		priceQueue = NewOrderQueue(price)
		pl.priceTree.Put(price, priceQueue)
		pl.depth++
	}
	pl.numOrders++
	pl.volume = pl.volume.Add(o.Qty)
	return priceQueue.Append(o)
}

// Remove removes order from definite price level
func (pl *PriceLevel) Remove(o *Order) *Order {
	price := o.GetPrice(pl.priceType)

	priceQueue, ok := pl.priceTree.Get(price)
	if !ok {
		return nil
	}
	o = priceQueue.Remove(o)

	if priceQueue.Len() == 0 {
		pl.priceTree.Remove(price)
		pl.depth--
	}

	pl.numOrders--
	pl.volume = pl.volume.Sub(o.Qty)
	return o
}

// MaxPriceQueue returns maximal level of price
func (pl *PriceLevel) MaxPriceQueue() *OrderQueue {
	if pl.depth > 0 {
		if value, found := pl.priceTree.GetMax(); found {
			return value.Value
		}
	}
	return nil
}

// MinPriceQueue returns maximal level of price
func (pl *PriceLevel) MinPriceQueue() *OrderQueue {
	if pl.depth > 0 {
		if value, found := pl.priceTree.GetMin(); found {
			return value.Value
		}
	}
	return nil
}

// LargestLessThan returns largest OrderQueue with price less than given
func (pl *PriceLevel) LargestLessThan(price decimal.Decimal) *OrderQueue {
	if node, ok := pl.priceTree.LargestLessThan(price); ok {
		return node.Value
	}

	return nil
}

// SmallestGreaterThan returns smallest OrderQueue with price greater than given
func (pl *PriceLevel) SmallestGreaterThan(price decimal.Decimal) *OrderQueue {
	if node, ok := pl.priceTree.SmallestGreaterThan(price); ok {
		return node.Value
	}

	return nil
}

// Orders returns all of * orders
func (pl *PriceLevel) Orders() (orders []*Order) {
	it := pl.priceTree.Iterator()
	for i := 0; it.Next(); i++ {
		iter := it.Value().Head()
		for iter != nil {
			orders = append(orders, iter)
			iter = iter.next
		}
	}
	return
}

// GetQueue returns the max/min order queue by the price type
func (pl *PriceLevel) GetQueue() *OrderQueue {
	switch pl.priceType {
	case BidPrice:
		return pl.MaxPriceQueue()
	case AskPrice:
		return pl.MinPriceQueue()
	default:
		panic("invalid call to GetQueue")
	}
}

// GetNextQueue returns the next level order queue by the price type
func (pl *PriceLevel) GetNextQueue(price decimal.Decimal) *OrderQueue {
	switch pl.priceType {
	case BidPrice:
		return pl.LargestLessThan(price)
	case AskPrice:
		return pl.SmallestGreaterThan(price)
	default:
		panic("invalid call to GetQueue")
	}
}

func (pl *PriceLevel) processMarketOrder(ob *OrderBook, takerOrderID uint64, qty decimal.Decimal, aon, fok bool) (qtyProcessed decimal.Decimal) {

	// TODO: This wont work as  PriceLevel volumes aren't accounted for corectly
	if (aon || fok) && qty.GreaterThan(pl.Volume()) {
		return decimal.Zero
	}

	qtyLeft := qty
	qtyProcessed = decimal.Zero
	for orderQueue := pl.GetQueue(); qtyLeft.GreaterThan(decimal.Zero) && orderQueue != nil; orderQueue = pl.GetQueue() {
		_, q := orderQueue.process(ob, takerOrderID, qtyLeft)
		qtyLeft = qtyLeft.Sub(q)
		qtyProcessed = qtyProcessed.Add(q)
	}

	return
}

func (pl *PriceLevel) processLimitOrder(ob *OrderBook, compare func(price decimal.Decimal) bool, takerOrderID uint64, qty decimal.Decimal, aon, fok bool) (qtyProcessed decimal.Decimal) {
	orderQueue := pl.GetQueue()
	if orderQueue == nil || !compare(orderQueue.Price()) {
		return
	}

	// TODO: Fix AoN
	if aon || fok {
		if qty.GreaterThan(pl.Volume()) {
			return decimal.Zero
		}

		canFill := false
		for orderQueue != nil && compare(orderQueue.Price()) {
			if qty.LessThanOrEqual(orderQueue.TotalQty()) {
				canFill = true
				break
			}
			qty = qty.Sub(orderQueue.TotalQty())
			orderQueue = pl.GetNextQueue(orderQueue.Price())
		}

		if !canFill {
			return decimal.Zero
		}
	}

	orderQueue = pl.GetQueue()
	qtyLeft := qty
	qtyProcessed = decimal.Zero
	for orderQueue := pl.GetQueue(); qtyLeft.GreaterThan(decimal.Zero) && orderQueue != nil; orderQueue = pl.GetQueue() {
		_, q := orderQueue.process(ob, takerOrderID, qtyLeft)
		qtyLeft = qtyLeft.Sub(q)
		qtyProcessed = qtyProcessed.Add(q)
	}

	return
}
