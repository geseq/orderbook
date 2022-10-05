package orderbook

const (
	MsgNewOrder MsgType = iota
	MsgCancelOrder
)

// String implements fmt.Stringer interface
func (c MsgType) String() string {
	switch c {
	case MsgNewOrder:
		return "NewOrder"
	case MsgCancelOrder:
		return "CancelOrder"
	default:
		return ""
	}
}
