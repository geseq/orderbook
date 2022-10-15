package orderbook

import "errors"

// OrderBook erros
var (
	ErrInvalidQuantity      = errors.New("orderbook: invalid order quantity")
	ErrInvalidPrice         = errors.New("orderbook: invalid order price")
	ErrInvalidTriggerPrice  = errors.New("orderbook: invalid trigger price")
	ErrOrderID              = errors.New("orderbook: order id is less than previous order ids")
	ErrOrderExists          = errors.New("orderbook: order already exists")
	ErrOrderNotExists       = errors.New("orderbook: order does not exist")
	ErrInsufficientQuantity = errors.New("orderbook: insufficient quantity to calculate price")
	ErrNoMatching           = errors.New("orderbook: matching disabled")
)
