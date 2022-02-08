package orderbook

import (
	"testing"

	decimal "github.com/geseq/udecimal"
)

func TestOrderQueue(t *testing.T) {
	price := decimal.New(100, 0)
	oq := newOrderQueue(price)

	o1 := NewOrder(
		1,
		Limit,
		Buy,
		decimal.New(100, 0),
		decimal.New(100, 0),
		decimal.Zero,
		None,
	)

	o2 := NewOrder(
		2,
		Limit,
		Buy,
		decimal.New(100, 0),
		decimal.New(100, 0),
		decimal.Zero,
		None,
	)

	head := oq.Append(o1)
	tail := oq.Append(o2)

	if head == nil || tail == nil {
		t.Fatal("Could not append order to the OrderQueue")
	}

	if !oq.TotalQty().Equal(decimal.New(200, 0)) {
		t.Fatalf("Invalid order volume (have: %s, want: 200", oq.TotalQty())
	}

	if head != o1 || tail != o2 {
		t.Fatal("Invalid element value")
	}

	if oq.head != head || oq.tail != tail {
		t.Fatal("Invalid element position")
	}

	if oq.head.next != oq.tail || oq.tail.prev != head ||
		oq.head.prev != nil || oq.tail.next != nil {
		t.Fatal("Invalid element link")
	}

	if o := oq.Remove(head); o != o1 {
		t.Fatal("Invalid element value")
	}

	if !oq.TotalQty().Equal(decimal.New(100, 0)) {
		t.Fatalf("Invalid order volume (have: %s, want: 100", oq.TotalQty())
	}

	t.Log(oq)
}
