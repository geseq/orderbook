package orderbook

import (
	decimal "github.com/geseq/udecimal"
)

// orderQueue stores and manage chain of orders
type orderQueue struct {
	size uint64
	head *Order
	tail *Order

	totalQty decimal.Decimal
	price    decimal.Decimal
}

// newOrderQueue creates and initialize orderQueue object
func newOrderQueue(price decimal.Decimal) *orderQueue {
	return &orderQueue{
		price:    price,
		totalQty: decimal.Zero,
	}
}

// Len returns amount of orders in queue
func (oq *orderQueue) Len() uint64 {
	return oq.size
}

// Price returns price level of the queue
func (oq *orderQueue) Price() decimal.Decimal {
	return oq.price
}

// TotalQty returns total order qty
func (oq *orderQueue) TotalQty() decimal.Decimal {
	return oq.totalQty
}

// Head returns top order in queue
func (oq *orderQueue) Head() *Order {
	return oq.head
}

// Append adds order to tail of the queue
func (oq *orderQueue) Append(o *Order) *Order {
	oq.totalQty = oq.totalQty.Add(o.Qty)
	tail := oq.tail
	oq.tail = o
	if tail != nil {
		tail.next = o
		o.prev = tail
	}
	if oq.head == nil {
		oq.head = o
	}
	oq.size++
	return o
}

// Remove removes order from the queue and link order chain
func (oq *orderQueue) Remove(o *Order) *Order {
	oq.totalQty = oq.totalQty.Sub(o.Qty)
	prev := o.prev
	next := o.next
	if prev != nil {
		prev.next = next
	}
	if next != nil {
		next.prev = prev
	}
	o.next = nil
	o.prev = nil

	oq.size--
	if oq.head == o {
		oq.head = next
	}
	if oq.tail == o {
		oq.tail = prev
	}
	return o
}

func (oq *orderQueue) process(ob *OrderBook, takerOrderID uint64, qty decimal.Decimal) (ordersClosed int, qtyProcessed decimal.Decimal) {
	for ho := oq.head; ho != nil && qty.GreaterThan(decimal.Zero); ho = oq.head {
		switch qty.Cmp(ho.Qty) {
		case -1:
			qtyProcessed = qtyProcessed.Add(qty)
			ho.Qty = ho.Qty.Sub(qty)
			oq.totalQty = oq.totalQty.Sub(qty)
			ob.notification.PutTrade(NewTrade(ho.ID, takerOrderID, ho.Price, qty, FilledPartial, FilledComplete))
			ob.lastPrice = ho.Price
			return
		case 1:
			qtyProcessed = qtyProcessed.Add(ho.Qty)
			qty = qty.Sub(ho.Qty)
			ob.cancelOrder(ho.ID)
			ob.notification.PutTrade(NewTrade(ho.ID, takerOrderID, ho.Price, ho.Qty, FilledComplete, FilledPartial))
			ob.lastPrice = ho.Price
			ordersClosed++
		case 0:
			qtyProcessed = qtyProcessed.Add(ho.Qty)
			qty = qty.Sub(ho.Qty)
			ob.cancelOrder(ho.ID)
			ob.notification.PutTrade(NewTrade(ho.ID, takerOrderID, ho.Price, ho.Qty, FilledComplete, FilledComplete))
			ob.lastPrice = ho.Price
			ordersClosed++
		}
	}
	return
}
