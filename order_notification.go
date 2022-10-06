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
		var errName string
		switch o.Error {
		case ErrOrderNotExists:
			errName = "ErrOrderNotExists"
		case ErrInvalidQuantity:
			errName = "ErrInvalidQuantity"
		case ErrInvalidPrice:
			errName = "ErrInvalidPrice"
		case ErrOrderID:
			errName = "ErrOrderID"
		case ErrOrderExists:
			errName = "ErrOrderExists"
		case ErrInsufficientQuantity:
			errName = "ErrInsufficientQuantity"
		case ErrNoMatching:
			errName = "ErrNoMatching"
		}

		return fmt.Sprintf("%s %s %d %s %s", o.MsgType, o.Status, o.OrderID, o.Qty.String(), errName)
	}
	return fmt.Sprintf("%s %s %d %s", o.MsgType, o.Status, o.OrderID, o.Qty.String())
}
