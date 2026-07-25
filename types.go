package orderbook

import (
	"fmt"

	decimal "github.com/geseq/udecimal"
)

// MsgType represents the type of message
type MsgType byte

// ClassType of the order
type ClassType byte

// SideType of the order
type SideType byte

// FlagType of the order
type FlagType byte

// Order strores information about request
type Order struct {
	ID        uint64          `json:"id" `
	Class     ClassType       `json:"class" `
	Side      SideType        `json:"side" `
	Flag      FlagType        `json:"flag" `
	Qty       decimal.Decimal `json:"qty" `
	Price     decimal.Decimal `json:"price" `
	TrigPrice decimal.Decimal `json:"trigPrice" `
	queue     *orderQueue
	prev      *Order
	next      *Order
}

// Trade strores information about request
type Trade struct {
	MakerOrderID uint64          `json:"makerOrderId" `
	TakerOrderID uint64          `json:"takerOrderId" `
	MakerStatus  OrderStatus     `json:"makerStatus" `
	TakerStatus  OrderStatus     `json:"takerStatus" `
	Qty          decimal.Decimal `json:"qty"`
	Price        decimal.Decimal `json:"price" `
}

// RestingOrder is a detached copy of an order resting in the book.
type RestingOrder struct {
	ID        uint64
	Class     ClassType
	Side      SideType
	Flag      FlagType
	Qty       decimal.Decimal
	Price     decimal.Decimal
	TrigPrice decimal.Decimal
}

// BookSnapshot contains resting bids and asks in matching-consumption order.
// Bids are ordered from highest to lowest price and asks from lowest to highest
// price. Orders at the same price retain FIFO order.
type BookSnapshot struct {
	Bids []RestingOrder
	Asks []RestingOrder
}

func (t Trade) String() string {
	return fmt.Sprintf("%d %d %s %s %s %s", t.MakerOrderID, t.TakerOrderID, t.MakerStatus, t.TakerStatus, t.Qty.String(), t.Price.String())
}
