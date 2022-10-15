package orderbook

import (
	"bytes"
	"encoding/binary"
	"errors"

	decimal "github.com/geseq/udecimal"
)

// NewOrder creates new constant object Order
func NewOrder(orderID uint64, class ClassType, side SideType, qty, price, trigPrice decimal.Decimal, flag FlagType) *Order {
	if class == Market {
		price = decimal.Zero
	}

	return &Order{
		ID:        orderID,
		Class:     class,
		Side:      side,
		Flag:      flag,
		Qty:       qty,
		Price:     price,
		TrigPrice: trigPrice,
	}
}

// GetPrice returns the price of the Order
func (o *Order) GetPrice(t PriceType) decimal.Decimal {
	if t == TrigPrice {
		return o.TrigPrice
	}
	return o.Price
}

// Compose converts the order to a binary representation
func (o *Order) Compose() []byte {
	buf := new(bytes.Buffer)

	idbuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(idbuf, o.ID)
	buf.Write(idbuf[:n])

	b, _ := o.Qty.MarshalBinary()
	buf.Write(b)

	b, _ = o.Price.MarshalBinary()
	buf.Write(b)

	b, _ = o.TrigPrice.MarshalBinary()
	buf.Write(b)

	buf.WriteByte(byte(o.Class))
	buf.WriteByte(byte(o.Side))
	buf.WriteByte(byte(o.Flag))

	return buf.Bytes()
}

// Decompose loads an object from its binaary representation
func (o *Order) Decompose(b []byte) error {
	id, n := binary.Uvarint(b)
	b = b[n:]
	qty := decimal.Decimal{}
	b, _ = qty.UnmarshalBinaryData(b)
	price := decimal.Decimal{}
	b, _ = price.UnmarshalBinaryData(b)
	trigPrice := decimal.Decimal{}
	b, _ = trigPrice.UnmarshalBinaryData(b)

	if len(b) != 3 {
		return errors.New("decompose failed: invalid bytes provided")
	}

	ord := Order{
		ID:        id,
		Class:     ClassType(b[0]),
		Side:      SideType(b[1]),
		Qty:       qty,
		Price:     price,
		TrigPrice: trigPrice,
		Flag:      FlagType(b[2]),
	}
	*o = ord

	return nil
}
