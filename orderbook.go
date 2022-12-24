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

var oPool = newOrderPool(1)
var ntPool = newNodeTreePool(1)
var oqPool = newOrderQueuePool(1)

// OrderBook implements standard matching algorithm
type OrderBook struct {
	asks         *priceLevel
	bids         *priceLevel
	triggerUnder *priceLevel       // orders triggering under last price i.e. Stop Sell or Take Buy
	triggerOver  *priceLevel       // orders that trigger over last price i.e. Stop Buy or Take Sell
	orders       map[uint64]*Order // orderId -> *Order
	trigOrders   map[uint64]*Order // orderId -> *Order
	trigQueue    *triggerQueue

	notification NotificationHandler

	lastPrice decimal.Decimal
	lastToken uint64

	matching bool

	orderPoolSize      uint64
	nodeTreePoolSize   uint64
	orderQueuePoolSize uint64
}

// NewOrderBook creates Orderbook object
func NewOrderBook(n NotificationHandler, opts ...Option) *OrderBook {
	ob := &OrderBook{
		orders:       map[uint64]*Order{},
		trigOrders:   map[uint64]*Order{},
		trigQueue:    newTriggerQueue(),
		bids:         newPriceLevel(BidPrice),
		asks:         newPriceLevel(AskPrice),
		triggerUnder: newPriceLevel(TrigPrice),
		triggerOver:  newPriceLevel(TrigPrice),
		notification: n,
	}

	options(defaultOpts).applyTo(ob)
	options(opts).applyTo(ob)

	oPool = newOrderPool(ob.orderPoolSize)
	ntPool = newNodeTreePool(ob.nodeTreePoolSize)
	oqPool = newOrderQueuePool(ob.orderQueuePoolSize)

	return ob
}

// AddOrder places new order to the OrderBook
// Arguments:
//      orderID   - unique order ID in depth (uint64)
//      class     - what class of order do you want to place (ob.Market or ob.Limit)
//      side      - what do you want to do (ob.Sell or ob.Buy)
//      quantity  - how much quantity you want to sell or buy (decimal)
//      price     - no more expensive (or cheaper) this price (decimal)
//      trigPrice - create a stop/take order until market price reaches this price (decimal)
//      flag      - immediate or cancel, all or none, fill or kill, Cancel (ob.IoC or ob.AoN or ob.FoK or ob.Cancel)
//      * to create new decimal number you should use udecimal.New() func
//        read more at https://github.com/geseq/udecimal
func (ob *OrderBook) AddOrder(tok, id uint64, class ClassType, side SideType, quantity, price, trigPrice decimal.Decimal, flag FlagType) {
	if !atomic.CompareAndSwapUint64(&ob.lastToken, tok-1, tok) {
		panic("invalid token received: cannot maintain determinism")
	}

	if quantity.Equal(decimal.Zero) {
		ob.notification.PutOrder(MsgCreateOrder, Rejected, id, quantity, ErrInvalidQuantity)
		return
	}

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

	if flag&(StopLoss|TakeProfit) != 0 {
		if trigPrice.IsZero() {
			ob.notification.PutOrder(MsgCreateOrder, Rejected, id, quantity, ErrInvalidTriggerPrice)
			return
		}

		ob.notification.PutOrder(MsgCreateOrder, Accepted, id, quantity, nil)
		ob.addTrigOrder(id, class, side, quantity, price, trigPrice, flag)
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

	return
}

func (ob *OrderBook) addTrigOrder(id uint64, class ClassType, side SideType, quantity, price, stPrice decimal.Decimal, flag FlagType) {
	switch flag {
	case StopLoss:
		switch side {
		case Buy:
			if stPrice.LessThanOrEqual(ob.lastPrice) {
				// Stop buy set under stop price, condition satisfied to trigger
				ob.processOrder(id, class, side, quantity, price, flag)
				return
			}

			ob.trigOrders[id] = ob.triggerOver.Append(NewOrder(id, class, side, quantity, price, stPrice, flag))
		case Sell:
			if ob.lastPrice.LessThanOrEqual(stPrice) {
				// Stop sell set over stop price, condition satisfied to trigger
				ob.processOrder(id, class, side, quantity, price, flag)
				return
			}

			ob.trigOrders[id] = ob.triggerUnder.Append(NewOrder(id, class, side, quantity, price, stPrice, flag))
		}
	case TakeProfit:
		switch side {
		case Buy:
			if ob.lastPrice.LessThanOrEqual(stPrice) {
				// Stop buy set under stop price, condition satisfied to trigger
				ob.processOrder(id, class, side, quantity, price, flag)
				return
			}

			ob.trigOrders[id] = ob.triggerUnder.Append(NewOrder(id, class, side, quantity, price, stPrice, flag))
		case Sell:
			if stPrice.LessThanOrEqual(ob.lastPrice) {
				// Stop sell set over stop price, condition satisfied to trigger
				ob.processOrder(id, class, side, quantity, price, flag)
				return
			}

			ob.trigOrders[id] = ob.triggerOver.Append(NewOrder(id, class, side, quantity, price, stPrice, flag))
		}
	}
}

func (ob *OrderBook) processOrder(id uint64, class ClassType, side SideType, quantity, price decimal.Decimal, flag FlagType) {
	lp := ob.lastPrice
	defer func() {
		if lp == ob.lastPrice {
			return
		}
		ob.queueTriggeredOrders()
		ob.processTriggeredOrders()
	}()

	if class == Market {
		if side == Buy {
			ob.asks.processMarketOrder(ob, id, quantity, flag)
		} else {
			ob.bids.processMarketOrder(ob, id, quantity, flag)
		}

		return
	}

	var qtyProcessed decimal.Decimal
	if side == Buy {
		qtyProcessed = ob.asks.processLimitOrder(ob, price.GreaterThanOrEqual, id, quantity, flag)
	} else {
		qtyProcessed = ob.bids.processLimitOrder(ob, price.LessThanOrEqual, id, quantity, flag)
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

func (ob *OrderBook) queueTriggeredOrders() {
	if ob.lastPrice.IsZero() {
		return
	}

	lastPrice := ob.lastPrice

	for q := ob.triggerOver.MaxPriceQueue(); q != nil && lastPrice.LessThanOrEqual(q.price); q = ob.triggerOver.MaxPriceQueue() {
		for q.Len() > 0 {
			o := q.Head()
			ob.triggerOver.Remove(o)
			ob.trigQueue.Push(o)
		}
	}

	for q := ob.triggerUnder.MinPriceQueue(); q != nil && lastPrice.GreaterThanOrEqual(q.price); q = ob.triggerUnder.MinPriceQueue() {
		for q.Len() > 0 {
			o := q.Head()
			ob.triggerUnder.Remove(o)
			ob.trigQueue.Push(o)
		}
	}
}

func (ob *OrderBook) processTriggeredOrders() {
	for o := ob.trigQueue.Pop(); o != nil; o = ob.trigQueue.Pop() {
		ob.processOrder(o.ID, o.Class, o.Side, o.Qty, o.Price, o.Flag)
	}
}

// Order returns order by id
func (ob *OrderBook) Order(orderID uint64) *Order {
	o, ok := ob.orders[orderID]
	if !ok {
		o, ok := ob.trigOrders[orderID]
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
	o.Release()
}

// CancelOrder removes order with given ID from the order book
func (ob *OrderBook) cancelOrder(orderID uint64) *Order {
	o, ok := ob.orders[orderID]
	if !ok {
		return ob.cancelTrigOrders(orderID)
	}

	delete(ob.orders, orderID)

	if o.Side == Buy {
		return ob.bids.Remove(o)
	}

	return ob.asks.Remove(o)
}

func (ob *OrderBook) cancelTrigOrders(orderID uint64) *Order {
	o, ok := ob.trigOrders[orderID]
	if !ok {
		return nil
	}

	delete(ob.trigOrders, orderID)

	if (o.Flag & StopLoss) != 0 {
		if o.Side == Buy {
			return ob.triggerOver.Remove(o)
		}
		return ob.triggerUnder.Remove(o)
	}

	if o.Side == Buy {
		return ob.triggerUnder.Remove(o)
	}
	return ob.triggerOver.Remove(o)
}
