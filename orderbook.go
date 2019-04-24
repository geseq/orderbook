package orderbook

import (
	decimal "github.com/geseq/udecimal"
)

// OrderStatus of the order
type OrderStatus byte

// String implements Stringer interface
func (o OrderStatus) String() string {
	switch o {
	case OrderRejected:
		return "OrderRejected"
	case OrderCanceled:
		return "OrderCanceled"
	case OrderFilledPartial:
		return "OrderFilledPartial"
	case OrderFilledComplete:
		return "OrderFilledComplete"
	case OrderCancelRejected:
		return "OrderCancelRejected"
	case OrderQueued:
		return "OrderQueued"
	}

	return ""
}

// Market or Limit
const (
	OrderRejected OrderStatus = iota
	OrderCanceled
	OrderFilledPartial
	OrderFilledComplete
	OrderCancelRejected
	OrderQueued
)

// OrderStateHandler handles order state updates
type OrderStateHandler interface {
	Put(OrderNotification)
}

// TradeHandler handles trade notifications
type TradeHandler interface {
	Put(Trade)
}

// OrderBook implements standard matching algorithm
type OrderBook struct {
	asks       *PriceLevel
	bids       *PriceLevel
	stops      *PriceLevel
	orders     map[uint64]*Order // orderId -> *Order
	stopOrders map[uint64]*Order // orderId -> *Order

	orderNotification OrderStateHandler
	tradeNotification TradeHandler

	lastStopPrice decimal.Decimal
	lastPrice     decimal.Decimal
	lastOrderID   uint64
}

// NewOrderBook creates Orderbook object
func NewOrderBook(orderStateHandler OrderStateHandler, tradeHandler TradeHandler) *OrderBook {
	return &OrderBook{
		orders:            map[uint64]*Order{},
		stopOrders:        map[uint64]*Order{},
		bids:              NewPriceLevel(BidPrice),
		asks:              NewPriceLevel(AskPrice),
		stops:             NewPriceLevel(StopPrice),
		orderNotification: orderStateHandler,
		tradeNotification: tradeHandler,
	}
}

// ProcessOrder places new order to the OrderBook
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
func (ob *OrderBook) ProcessOrder(id uint64, class ClassType, side SideType, quantity, price, stopPrice decimal.Decimal, flag FlagType) {
	switch flag {
	case Cancel:
		ob.CancelOrder(id)
		return
	}

	if id <= ob.lastOrderID {
		ob.orderNotification.Put(OrderNotification{id, OrderRejected, quantity, ErrOrderId})
		return
	}

	ob.lastOrderID = id

	if stopPrice.GreaterThan(decimal.Zero) {
		ob.stopOrders[id] = ob.stops.Append(NewOrder(id, class, side, quantity, price, stopPrice, flag))
		return
	}

	ob.processOrder(id, class, side, quantity, price, flag)
	ob.processStopOrders()
	return
}

func (ob *OrderBook) processOrder(id uint64, class ClassType, side SideType, quantity, price decimal.Decimal, flag FlagType) {
	if quantity.Equal(decimal.Zero) {
		ob.orderNotification.Put(OrderNotification{id, OrderRejected, quantity, ErrInvalidQuantity})
		return
	}

	if class == Market {
		if side == Buy {
			ob.asks.processMarketOrder(ob, id, quantity, flag == AoN, flag == FoK)
		} else {
			ob.bids.processMarketOrder(ob, id, quantity, flag == AoN, flag == FoK)
		}

		return
	}

	if _, ok := ob.orders[id]; ok {
		ob.orderNotification.Put(OrderNotification{id, OrderRejected, decimal.Zero, ErrOrderExists})
		return
	}

	if price.Equal(decimal.Zero) {
		ob.orderNotification.Put(OrderNotification{id, OrderRejected, decimal.Zero, ErrInvalidPrice})
		return
	}

	qtyProcessed := decimal.Zero
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

	ob.orderNotification.Put(OrderNotification{id, OrderQueued, quantityLeft, nil})
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
func (ob *OrderBook) CancelOrder(orderID uint64) {
	o := ob.cancelOrder(orderID)
	if o == nil {
		ob.orderNotification.Put(OrderNotification{orderID, OrderCancelRejected, decimal.Zero, ErrOrderNotExists})
		return
	}

	ob.orderNotification.Put(OrderNotification{o.ID, OrderCanceled, o.Qty, nil})
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
		level *OrderQueue
		iter  func(decimal.Decimal) *OrderQueue
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
