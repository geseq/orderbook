package orderbook

import (
	decimal "github.com/geseq/udecimal"
)

//go:generate gotemplate "github.com/geseq/redblacktree" tree(udecimal.Decimal,*orderQueue)

// priceLevel implements facade to operations with order queue
type priceLevel struct {
	priceTree *tree
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

// newPriceLevel creates new priceLevel manager
func newPriceLevel(priceType PriceType) *priceLevel {
	return &priceLevel{
		priceTree: newWithTree(Comparator),
		priceType: priceType,
		volume:    decimal.Zero,
	}
}

// Len returns amount of orders
func (pl *priceLevel) Len() uint64 {
	return pl.numOrders
}

// Depth returns depth of market
func (pl *priceLevel) Depth() int {
	return pl.depth
}

// Volume returns total amount of quantity in side
func (pl *priceLevel) Volume() decimal.Decimal {
	return pl.volume
}

// Append appends order to definite price level
func (pl *priceLevel) Append(o *Order) *Order {
	price := o.GetPrice(pl.priceType)

	priceQueue, ok := pl.priceTree.Get(price)
	if !ok {
		priceQueue = newOrderQueue(price)
		pl.priceTree.Put(price, priceQueue)
		pl.depth++
	}
	pl.numOrders++
	pl.volume = pl.volume.Add(o.Qty)
	o.queue = priceQueue
	return priceQueue.Append(o)
}

// Remove removes order from definite price level
func (pl *priceLevel) Remove(o *Order) *Order {
	price := o.GetPrice(pl.priceType)

	priceQueue := o.queue
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
func (pl *priceLevel) MaxPriceQueue() *orderQueue {
	if pl.depth > 0 {
		if value, found := pl.priceTree.GetMax(); found {
			return value.Value
		}
	}
	return nil
}

// MinPriceQueue returns maximal level of price
func (pl *priceLevel) MinPriceQueue() *orderQueue {
	if pl.depth > 0 {
		if value, found := pl.priceTree.GetMin(); found {
			return value.Value
		}
	}
	return nil
}

// LargestLessThan returns largest orderQueue with price less than given
func (pl *priceLevel) LargestLessThan(price decimal.Decimal) *orderQueue {
	if node, ok := pl.priceTree.LargestLessThan(price); ok {
		return node.Value
	}

	return nil
}

// SmallestGreaterThan returns smallest orderQueue with price greater than given
func (pl *priceLevel) SmallestGreaterThan(price decimal.Decimal) *orderQueue {
	if node, ok := pl.priceTree.SmallestGreaterThan(price); ok {
		return node.Value
	}

	return nil
}

// Orders returns all of * orders
func (pl *priceLevel) Orders() (orders []*Order) {
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
func (pl *priceLevel) GetQueue() *orderQueue {
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
func (pl *priceLevel) GetNextQueue(price decimal.Decimal) *orderQueue {
	switch pl.priceType {
	case BidPrice:
		return pl.LargestLessThan(price)
	case AskPrice:
		return pl.SmallestGreaterThan(price)
	default:
		panic("invalid call to GetQueue")
	}
}

func (pl *priceLevel) processMarketOrder(ob *OrderBook, takerOrderID uint64, qty decimal.Decimal, aon, fok bool) (qtyProcessed decimal.Decimal) {

	// TODO: This wont work as  priceLevel volumes aren't accounted for corectly
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

func (pl *priceLevel) processLimitOrder(ob *OrderBook, compare func(price decimal.Decimal) bool, takerOrderID uint64, qty decimal.Decimal, aon, fok bool) (qtyProcessed decimal.Decimal) {
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
