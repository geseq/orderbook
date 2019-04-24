package orderbook

import (
	decimal "github.com/geseq/udecimal"
)

// NewTrade creates new constant object Trade
func NewTrade(makerOrderID, takerOrderID uint64, price, qty decimal.Decimal, makerStatus, takerStatus OrderStatus) Trade {
	return Trade{
		MakerOrderID: makerOrderID,
		TakerOrderID: takerOrderID,
		MakerStatus:  makerStatus,
		TakerStatus:  takerStatus,
		Price:        price,
		Qty:          qty,
	}
}
