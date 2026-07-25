package orderbook

const (
	MsgCreateOrder MsgType = iota
	MsgCancelOrder
	MsgAmendOrder
)

// String implements fmt.Stringer interface
func (c MsgType) String() string {
	switch c {
	case MsgCreateOrder:
		return "CreateOrder"
	case MsgCancelOrder:
		return "CancelOrder"
	case MsgAmendOrder:
		return "AmendOrder"
	default:
		return ""
	}
}
