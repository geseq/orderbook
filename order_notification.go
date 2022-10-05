package orderbook

import (
	"fmt"

	decimal "github.com/geseq/udecimal"
)

type OrderNotification struct {
	MsgType MsgType
	Status  OrderStatus
	OrderID uint64
	Qty     decimal.Decimal
	Error   error
}

func (o OrderNotification) String() string {
	if o.Error != nil {
		return fmt.Sprintf("%s %s %d %s %s", o.MsgType, o.Status, o.OrderID, o.Qty.String(), o.Error.Error())
	}
	return fmt.Sprintf("%s %s %d %s", o.MsgType, o.Status, o.OrderID, o.Qty.String())
}
