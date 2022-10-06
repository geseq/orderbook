package orderbook

const (
	MsgCreateOrder MsgType = iota
	MsgCancelOrder
)

// String implements fmt.Stringer interface
func (c MsgType) String() string {
	switch c {
	case MsgCreateOrder:
		return "CreateOrder"
	case MsgCancelOrder:
		return "CancelOrder"
	default:
		return ""
	}
}
