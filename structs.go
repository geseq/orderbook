package orderbook

import (
	decimal "github.com/geseq/udecimal"
)

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
	prev      *Order
	next      *Order
}

// Trade strores information about request
type Trade struct {
	MakerOrderID uint64          `json:"makerOrderId" `
	TakerOrderID uint64          `json:"takerOrderId" `
	MakerQtyLeft decimal.Decimal `json:"makerQtyLeft" `
	TakerQtyLeft decimal.Decimal `json:"takerQtyLeft" `
	MakerStatus  OrderStatus     `json:"makerStatus" `
	TakerStatus  OrderStatus     `json:"takerStatus" `
	Price        decimal.Decimal `json:"price" `
	Qty          decimal.Decimal `json:"qty"`
}
