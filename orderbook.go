package orderbook

import (
	"sync/atomic"

	"github.com/geseq/orderbook/pkg/pool"
	local_tree "github.com/geseq/orderbook/pkg/tree"
	decimal "github.com/geseq/udecimal"
)

// NotificationHandler handles notification updates
type NotificationHandler interface {
	PutOrder(m MsgType, s OrderStatus, orderID uint64, qty decimal.Decimal, err error)
	PutTrade(makerOrderID, takerOrderID uint64, makerStatus, takerStatus OrderStatus, qty, price decimal.Decimal)
}

var oPool = pool.NewItemPoolV2[Order](1)
var oqPool = pool.NewItemPoolV2[orderQueue](1)

// OrderBook implements standard matching algorithm
type OrderBook struct {
	asks         *priceLevel
	bids         *priceLevel
	triggerUnder *priceLevel                      // orders triggering under last price i.e. Stop Sell or Take Buy
	triggerOver  *priceLevel                      // orders that trigger over last price i.e. Stop Buy or Take Sell
	orders       *local_tree.Tree[uint64, *Order] // orderId -> *Order
	trigOrders   *local_tree.Tree[uint64, *Order] // orderId -> *Order
	trigQueue    *triggerQueue

	notification NotificationHandler

	lastPrice decimal.Decimal
	lastToken uint64

	matching bool

	orderPoolSize         uint64
	nodeTreePoolSize      uint64
	orderTreeNodePoolSize uint64
	orderQueuePoolSize    uint64
}

// Uint64Cmp compares two uint64.
func Uint64Cmp(a, b uint64) int {
	if a == b {
		return 0
	}
	if a < b {
		return -1
	}
	return 1
}

// NewOrderBook creates Orderbook object
func NewOrderBook(n NotificationHandler, opts ...Option) *OrderBook {
	ob := &OrderBook{
		orders:       local_tree.NewWithTree[uint64, *Order](Uint64Cmp, 2),
		trigOrders:   local_tree.NewWithTree[uint64, *Order](Uint64Cmp, 2),
		trigQueue:    newTriggerQueue(),
		bids:         newPriceLevel(BidPrice),
		asks:         newPriceLevel(AskPrice),
		triggerUnder: newPriceLevel(TrigPrice),
		triggerOver:  newPriceLevel(TrigPrice),
		notification: n,
	}

	options(defaultOpts).applyTo(ob)
	options(opts).applyTo(ob)

	oPool = pool.NewItemPoolV2[Order](ob.orderPoolSize)
	oqPool = pool.NewItemPoolV2[orderQueue](ob.orderQueuePoolSize)

	return ob
}

// AddOrder places new order to the OrderBook
// Arguments:
//
//	orderID   - unique order ID in depth (uint64)
//	class     - what class of order do you want to place (ob.Market or ob.Limit)
//	side      - what do you want to do (ob.Sell or ob.Buy)
//	quantity  - how much quantity you want to sell or buy (decimal)
//	price     - no more expensive (or cheaper) this price (decimal)
//	trigPrice - create a stop/take order until market price reaches this price (decimal)
//	flag      - immediate or cancel, all or none, fill or kill, Cancel (ob.IoC or ob.AoN or ob.FoK or ob.Cancel)
//	* to create new decimal number you should use udecimal.New() func
//	  read more at https://github.com/geseq/udecimal
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
		if _, ok := ob.orders.Get(id); ok {
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

			ob.trigOrders.Put(id, ob.triggerOver.Append(NewOrder(id, class, side, quantity, price, stPrice, flag)))
		case Sell:
			if ob.lastPrice.LessThanOrEqual(stPrice) {
				// Stop sell set over stop price, condition satisfied to trigger
				ob.processOrder(id, class, side, quantity, price, flag)
				return
			}

			ob.trigOrders.Put(id, ob.triggerUnder.Append(NewOrder(id, class, side, quantity, price, stPrice, flag)))
		}
	case TakeProfit:
		switch side {
		case Buy:
			if ob.lastPrice.LessThanOrEqual(stPrice) {
				// Stop buy set under stop price, condition satisfied to trigger
				ob.processOrder(id, class, side, quantity, price, flag)
				return
			}

			ob.trigOrders.Put(id, ob.triggerUnder.Append(NewOrder(id, class, side, quantity, price, stPrice, flag)))
		case Sell:
			if stPrice.LessThanOrEqual(ob.lastPrice) {
				// Stop sell set over stop price, condition satisfied to trigger
				ob.processOrder(id, class, side, quantity, price, flag)
				return
			}

			ob.trigOrders.Put(id, ob.triggerOver.Append(NewOrder(id, class, side, quantity, price, stPrice, flag)))
		}
	}
}

func (ob *OrderBook) postProcess(lp decimal.Decimal) {
	if lp == ob.lastPrice {
		return
	}
	ob.queueTriggeredOrders()
	ob.processTriggeredOrders()
}

func (ob *OrderBook) processOrder(id uint64, class ClassType, side SideType, quantity, price decimal.Decimal, flag FlagType) {
	lp := ob.lastPrice

	if class == Market {
		if side == Buy {
			ob.asks.processMarketOrder(ob, id, quantity, flag)
		} else {
			ob.bids.processMarketOrder(ob, id, quantity, flag)
		}

		ob.postProcess(lp)
		return
	}

	var qtyProcessed decimal.Decimal
	if side == Buy {
		qtyProcessed = ob.asks.processLimitOrder(ob, price.GreaterThanOrEqual, id, quantity, flag)
	} else {
		qtyProcessed = ob.bids.processLimitOrder(ob, price.LessThanOrEqual, id, quantity, flag)
	}

	if flag == IoC || flag == FoK {
		ob.postProcess(lp)
		return
	}

	quantityLeft := quantity.Sub(qtyProcessed)
	if quantityLeft.GreaterThan(decimal.Zero) {
		o := NewOrder(id, class, side, quantityLeft, price, decimal.Zero, flag)
		if side == Buy {
			ob.orders.Put(id, ob.bids.Append(o))
		} else {
			ob.orders.Put(id, ob.asks.Append(o))
		}
	}

	ob.postProcess(lp)
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
	o, ok := ob.orders.Get(orderID)
	if !ok {
		o, ok := ob.trigOrders.Get(orderID)
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
	o, ok := ob.orders.Get(orderID)
	if !ok {
		return ob.cancelTrigOrders(orderID)
	}

	ob.orders.Remove(orderID)

	if o.Side == Buy {
		return ob.bids.Remove(o)
	}

	return ob.asks.Remove(o)
}

func (ob *OrderBook) cancelTrigOrders(orderID uint64) *Order {
	o, ok := ob.trigOrders.Get(orderID)
	if !ok {
		return nil
	}

	ob.trigOrders.Remove(orderID)

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

// Ask returns the best (lowest priced) sell order (Ask) from the order book.
// It compares and swaps the `lastToken` to ensure that the provided token is correct
// and maintains deterministic behavior during the execution of the order matching.
//
// Parameters:
//   - tok: A unique token (uint64) used for deterministic order matching. The method
//     uses atomic operations to ensure that the token is updated correctly.
//
// Returns:
//   - *Order: The lowest-priced sell order (Ask) in the order book. If there are no
//     sell orders, the method returns nil.
//
// The method works as follows:
//  1. It uses `CompareAndSwapUint64` to check and update the last token, ensuring
//     that the token follows the expected order and is correctly managed.
//  2. It retrieves the `MinPriceQueue`, which represents the price level with the
//     lowest Ask (sell) orders.
//  3. It returns the first order in that queue using `Head()`.
//  4. If there are no orders, it returns `nil`.
func (ob *OrderBook) Ask(tok uint64) *Order {
	if !atomic.CompareAndSwapUint64(&ob.lastToken, tok-1, tok) {
		panic("invalid token received: cannot maintain determinism")
	}
	orderQueue := ob.asks.MinPriceQueue()
	if orderQueue == nil {
		return nil
	}

	return orderQueue.Head()
}

// Bid returns the best (highest priced) buy order (Bid) from the order book.
// Similar to `Ask()`, it uses atomic operations to ensure that the provided token
// is correct and ensures deterministic order processing.
//
// Parameters:
//   - tok: A unique token (uint64) used for deterministic order matching. The method
//     ensures the token is valid and uses atomic operations to update the token
//     correctly during execution.
//
// Returns:
//   - *Order: The highest-priced buy order (Bid) in the order book. If there are no
//     buy orders, the method returns nil.
//
// The method works as follows:
//  1. It uses `CompareAndSwapUint64` to ensure that the token is correct and
//     maintains order-matching determinism.
//  2. It retrieves the `MaxPriceQueue`, which represents the price level with the
//     highest Bid (buy) orders.
//  3. It returns the first order in that queue using `Head()`.
//  4. If there are no buy orders, it returns `nil`.
func (ob *OrderBook) Bid(tok uint64) *Order {
	if !atomic.CompareAndSwapUint64(&ob.lastToken, tok-1, tok) {
		panic("invalid token received: cannot maintain determinism")
	}
	orderQueue := ob.bids.MaxPriceQueue()
	if orderQueue == nil {
		return nil
	}

	return orderQueue.Head()
}
