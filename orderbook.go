package orderbook

import (
	"sync/atomic"

	decimal "github.com/geseq/udecimal"
)

// NotificationHandler handles notification updates
type NotificationHandler interface {
	PutOrder(m MsgType, s OrderStatus, orderID uint64, qty decimal.Decimal, err error)
	PutTrade(makerOrderID, takerOrderID uint64, makerStatus, takerStatus OrderStatus, qty, price decimal.Decimal)
}

// OrderBook implements standard matching algorithm
type OrderBook struct {
	asks       *priceLevel
	bids       *priceLevel
	stops      *priceLevel
	orders     map[uint64]*Order // orderId -> *Order
	stopOrders map[uint64]*Order // orderId -> *Order

	notification NotificationHandler

	lastStopPrice decimal.Decimal
	lastPrice     decimal.Decimal
	lastToken     uint64
	lastOrderID   uint64

	matching bool
}

// NewOrderBook creates Orderbook object
func NewOrderBook(n NotificationHandler, opts ...Option) *OrderBook {
	ob := &OrderBook{
		orders:       map[uint64]*Order{},
		stopOrders:   map[uint64]*Order{},
		bids:         newPriceLevel(BidPrice),
		asks:         newPriceLevel(AskPrice),
		stops:        newPriceLevel(StopPrice),
		notification: n,
	}

	options(defaultOpts).applyTo(ob)
	options(opts).applyTo(ob)

	return ob
}

// AddOrder places new order to the OrderBook
// Arguments:
//      orderID   - unique order ID in depth (uint64)
//      class     - what class of order do you want to place (ob.Market or ob.Limit)
//      side      - what do you want to do (ob.Sell or ob.Buy)
//      quantity  - how much quantity you want to sell or buy (decimal)
//      price     - no more expensive (or cheaper) this price (decimal)
//      stopPrice - create a stop order until market price crosses this price (decimal)
//      flag      - immediate or cancel, all or none, fill or kill, Cancel (ob.IoC or ob.AoN or ob.FoK or ob.Cancel)
//      * to create new decimal number you should use udecimal.New() func
//        read more at https://github.com/geseq/udecimal
func (ob *OrderBook) AddOrder(tok, id uint64, class ClassType, side SideType, quantity, price, stopPrice decimal.Decimal, flag FlagType) {
	if !atomic.CompareAndSwapUint64(&ob.lastToken, tok-1, tok) {
		panic("invalid token received: cannot maintain determinism")
	}

	if id <= ob.lastOrderID {
		ob.notification.PutOrder(MsgCreateOrder, Rejected, id, quantity, ErrOrderID)
		return
	}

	ob.lastOrderID = id

	if !ob.matching {
		// If matching is disabled reject all orders that cross the book
		if class == Market {
			ob.notification.PutOrder(MsgCreateOrder, Rejected, id, quantity, ErrNoMatching)
			return
		}

		if side == Buy {
			q := ob.asks.GetQueue()
			if q != nil && q.Price().LessThanOrEqual(price) {
				ob.notification.PutOrder(MsgCreateOrder, Rejected, id, quantity, ErrNoMatching)
				return
			}
		} else {
			q := ob.bids.GetQueue()
			if q != nil && q.Price().GreaterThanOrEqual(price) {
				ob.notification.PutOrder(MsgCreateOrder, Rejected, id, quantity, ErrNoMatching)

				return
			}
		}
	}

	if quantity.Equal(decimal.Zero) {
		ob.notification.PutOrder(MsgCreateOrder, Rejected, id, quantity, ErrInvalidQuantity)
		return
	}

	if stopPrice.GreaterThan(decimal.Zero) {
		ob.notification.PutOrder(MsgCreateOrder, Accepted, id, quantity, nil)
		if side == Buy && stopPrice.LessThanOrEqual(ob.lastPrice) {
			// Stop buy set under stop price, condition satisfied to trigger
			ob.processOrder(id, class, side, quantity, price, flag)
			return
		}

		if side == Sell && ob.lastPrice.LessThanOrEqual(stopPrice) {
			// Stop sell set over stop price, condition satisfied to trigger
			ob.processOrder(id, class, side, quantity, price, flag)
			return
		}

		ob.stopOrders[id] = ob.stops.Append(NewOrder(id, class, side, quantity, price, stopPrice, flag))
		return
	}

	if class != Market {
		if _, ok := ob.orders[id]; ok {
			ob.notification.PutOrder(MsgCreateOrder, Rejected, id, decimal.Zero, ErrOrderExists)
			return
		}

		if price.Equal(decimal.Zero) {
			ob.notification.PutOrder(MsgCreateOrder, Rejected, id, decimal.Zero, ErrInvalidPrice)
			return
		}
	}

	ob.notification.PutOrder(MsgCreateOrder, Accepted, id, quantity, nil)

	ob.processOrder(id, class, side, quantity, price, flag)
	ob.processStopOrders()
	return
}

func (ob *OrderBook) processOrder(id uint64, class ClassType, side SideType, quantity, price decimal.Decimal, flag FlagType) {
	if class == Market {
		if side == Buy {
			ob.asks.processMarketOrder(ob, id, quantity, flag == AoN, flag == FoK)
		} else {
			ob.bids.processMarketOrder(ob, id, quantity, flag == AoN, flag == FoK)
		}

		return
	}

	var qtyProcessed decimal.Decimal
	if side == Buy {
		qtyProcessed = ob.asks.processLimitOrder(ob, price.GreaterThanOrEqual, id, quantity, flag == AoN, flag == FoK)
	} else {
		qtyProcessed = ob.bids.processLimitOrder(ob, price.LessThanOrEqual, id, quantity, flag == AoN, flag == FoK)
	}

	if flag == IoC || flag == FoK {
		return
	}

	quantityLeft := quantity.Sub(qtyProcessed)
	if quantityLeft.GreaterThan(decimal.Zero) {
		o := NewOrder(id, class, side, quantityLeft, price, decimal.Zero, flag)
		if side == Buy {
			ob.orders[id] = ob.bids.Append(o)
		} else {
			ob.orders[id] = ob.asks.Append(o)
		}
	}

	return
}

func (ob *OrderBook) processStopOrders() {
	for !ob.lastPrice.Equal(ob.lastStopPrice) {
		if ob.lastPrice.GreaterThan(ob.lastStopPrice) {
			stops := ob.stops.SmallestGreaterThan(ob.lastStopPrice)
			for stops != nil && stops.Price().LessThanOrEqual(ob.lastPrice) {
				for stops.Len() > 0 {
					stop := stops.Head()
					ob.processOrder(stop.ID, stop.Class, stop.Side, stop.Qty, stop.Price, stop.Flag)
					ob.stops.Remove(stop)
				}
				stops = ob.stops.SmallestGreaterThan(ob.lastStopPrice)
			}
		} else {
			stops := ob.stops.LargestLessThan(ob.lastStopPrice)
			for stops != nil && stops.Price().GreaterThanOrEqual(ob.lastPrice) {
				for stops.Len() > 0 {
					stop := stops.Head()
					ob.processOrder(stop.ID, stop.Class, stop.Side, stop.Qty, stop.Price, stop.Flag)
					ob.stops.Remove(stop)
				}
				stops = ob.stops.LargestLessThan(ob.lastStopPrice)
			}
		}

		ob.lastStopPrice = ob.lastPrice
	}
}

// Order returns order by id
func (ob *OrderBook) Order(orderID uint64) *Order {
	o, ok := ob.orders[orderID]
	if !ok {
		o, ok := ob.stopOrders[orderID]
		if !ok {
			return nil
		}

		return o
	}

	return o
}

// CancelOrder removes order with given ID from the order book
func (ob *OrderBook) CancelOrder(tok, orderID uint64) {
	if !atomic.CompareAndSwapUint64(&ob.lastToken, tok-1, tok) {
		panic("invalid token received: cannot maintain determinism")
	}

	o := ob.cancelOrder(orderID)
	if o == nil {
		ob.notification.PutOrder(MsgCancelOrder, Rejected, orderID, decimal.Zero, ErrOrderNotExists)
		return
	}

	ob.notification.PutOrder(MsgCancelOrder, Canceled, o.ID, o.Qty, nil)
}

// CancelOrder removes order with given ID from the order book
func (ob *OrderBook) cancelOrder(orderID uint64) *Order {
	o, ok := ob.orders[orderID]
	if !ok {
		o, ok := ob.stopOrders[orderID]
		if !ok {
			return nil
		}

		delete(ob.stopOrders, orderID)
		return ob.stops.Remove(o)
	}

	delete(ob.orders, orderID)

	if o.Side == Buy {
		return ob.bids.Remove(o)
	}

	return ob.asks.Remove(o)
}

// CalculateMarketPrice returns total market price for requested quantity
// if err is not nil price returns total price of all levels in side
func (ob *OrderBook) CalculateMarketPrice(side SideType, quantity decimal.Decimal) (price decimal.Decimal, err error) {
	price = decimal.Zero

	var (
		level *orderQueue
		iter  func(decimal.Decimal) *orderQueue
	)

	if side == Buy {
		level = ob.asks.MinPriceQueue()
		iter = ob.asks.SmallestGreaterThan
	} else {
		level = ob.bids.MaxPriceQueue()
		iter = ob.bids.LargestLessThan
	}

	for quantity.GreaterThan(decimal.Zero) && level != nil {
		levelQty := level.TotalQty()
		levelPrice := level.Price()
		if quantity.GreaterThanOrEqual(levelQty) {
			price = price.Add(levelPrice.Mul(levelQty))
			quantity = quantity.Sub(levelQty)
			level = iter(levelPrice)
		} else {
			price = price.Add(levelPrice.Mul(quantity))
			quantity = decimal.Zero
		}
	}

	if quantity.GreaterThan(decimal.Zero) {
		err = ErrInsufficientQuantity
	}

	return
}
