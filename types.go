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
	StopPrice decimal.Decimal `json:"stopPrice" `
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
	Price        decimal.Decimal `json:"price" `
	Qty          decimal.Decimal `json:"qty"`
}

func (t Trade) String() string {
	return fmt.Sprintf("%d %d %s %s %s %s", t.MakerOrderID, t.TakerOrderID, t.MakerStatus, t.TakerStatus, t.Qty.String(), t.Price.String())
}
