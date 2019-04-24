package orderbook

import (
	decimal "github.com/geseq/udecimal"
)

// OrderNotification contains information about changes to an order
type OrderNotification struct {
	OrderID uint64
	Status  OrderStatus
	Qty     decimal.Decimal
	Error   error
}
