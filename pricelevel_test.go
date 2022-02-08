package orderbook

import (
	"testing"

	decimal "github.com/geseq/udecimal"
)

func TestPriceLevel(t *testing.T) {
	ot := newPriceLevel(BidPrice)

	o1 := NewOrder(
		1,
		Limit,
		Buy,
		decimal.New(10, 0),
		decimal.New(10, 0),
		decimal.Zero,
		None,
	)

	o2 := NewOrder(
		2,
		Limit,
		Buy,
		decimal.New(10, 0),
		decimal.New(20, 0),
		decimal.Zero,
		None,
	)

	if ot.MinPriceQueue() != nil || ot.MaxPriceQueue() != nil {
		t.Fatal("invalid price levels")
	}

	el1 := ot.Append(o1)

	if ot.MinPriceQueue() != ot.MaxPriceQueue() {
		t.Fatal("invalid price levels")
	}

	el2 := ot.Append(o2)

	if ot.Depth() != 2 {
		t.Fatal("invalid depth")
	}

	if ot.Len() != 2 {
		t.Fatal("invalid orders count")
	}

	t.Log(ot)

	if ot.MinPriceQueue().head != el1 || ot.MinPriceQueue().tail != el1 ||
		ot.MaxPriceQueue().head != el2 || ot.MaxPriceQueue().tail != el2 {
		t.Fatal("invalid price levels")
	}

	if o := ot.Remove(el1); o != o1 {
		t.Fatal("invalid order")
	}

	if ot.MinPriceQueue() != ot.MaxPriceQueue() {
		t.Fatal("invalid price levels")
	}

	t.Log(ot)
}

func TestPriceFinding(t *testing.T) {
	os := newPriceLevel(AskPrice)

	os.Append(NewOrder(1, Limit, Sell, decimal.New(5, 0), decimal.New(130, 0), decimal.Zero, None))
	os.Append(NewOrder(2, Limit, Sell, decimal.New(5, 0), decimal.New(170, 0), decimal.Zero, None))
	os.Append(NewOrder(3, Limit, Sell, decimal.New(5, 0), decimal.New(100, 0), decimal.Zero, None))
	os.Append(NewOrder(4, Limit, Sell, decimal.New(5, 0), decimal.New(160, 0), decimal.Zero, None))
	os.Append(NewOrder(5, Limit, Sell, decimal.New(5, 0), decimal.New(140, 0), decimal.Zero, None))
	os.Append(NewOrder(6, Limit, Sell, decimal.New(5, 0), decimal.New(120, 0), decimal.Zero, None))
	os.Append(NewOrder(7, Limit, Sell, decimal.New(5, 0), decimal.New(150, 0), decimal.Zero, None))
	os.Append(NewOrder(8, Limit, Sell, decimal.New(5, 0), decimal.New(110, 0), decimal.Zero, None))

	if !os.Volume().Equal(decimal.New(40, 0)) {
		t.Fatal("invalid volume")
	}

	if !os.LargestLessThan(decimal.New(101, 0)).Price().Equal(decimal.New(100, 0)) ||
		!os.LargestLessThan(decimal.New(150, 0)).Price().Equal(decimal.New(140, 0)) ||
		os.LargestLessThan(decimal.New(100, 0)) != nil {
		t.Fatal("LessThan return invalid price")
	}

	if !os.SmallestGreaterThan(decimal.New(169, 0)).Price().Equal(decimal.New(170, 0)) ||
		!os.SmallestGreaterThan(decimal.New(150, 0)).Price().Equal(decimal.New(160, 0)) ||
		os.SmallestGreaterThan(decimal.New(170, 0)) != nil {
		t.Fatal("GreaterThan return invalid price")
	}

	t.Log(os.LargestLessThan(decimal.New(101, 0)))
	t.Log(os.SmallestGreaterThan(decimal.New(169, 0)))
}

func TestStopQueuePriceFinding(t *testing.T) {
	os := newPriceLevel(StopPrice)

	os.Append(NewOrder(1, Limit, Sell, decimal.New(5, 0), decimal.New(10, 0), decimal.New(130, 0), None))
	os.Append(NewOrder(2, Limit, Sell, decimal.New(5, 0), decimal.New(20, 0), decimal.New(170, 0), None))
	os.Append(NewOrder(3, Limit, Sell, decimal.New(5, 0), decimal.New(30, 0), decimal.New(100, 0), None))
	os.Append(NewOrder(4, Limit, Sell, decimal.New(5, 0), decimal.New(40, 0), decimal.New(160, 0), None))
	os.Append(NewOrder(5, Limit, Sell, decimal.New(5, 0), decimal.New(50, 0), decimal.New(140, 0), None))
	os.Append(NewOrder(6, Limit, Sell, decimal.New(5, 0), decimal.New(60, 0), decimal.New(120, 0), None))
	os.Append(NewOrder(7, Limit, Sell, decimal.New(5, 0), decimal.New(70, 0), decimal.New(150, 0), None))
	os.Append(NewOrder(8, Limit, Sell, decimal.New(5, 0), decimal.New(80, 0), decimal.New(110, 0), None))

	if !os.Volume().Equal(decimal.New(40, 0)) {
		t.Fatal("invalid volume")
	}

	if !os.LargestLessThan(decimal.New(101, 0)).Price().Equal(decimal.New(100, 0)) ||
		!os.LargestLessThan(decimal.New(150, 0)).Price().Equal(decimal.New(140, 0)) ||
		os.LargestLessThan(decimal.New(100, 0)) != nil {
		t.Fatal("LessThan return invalid price")
	}

	if !os.SmallestGreaterThan(decimal.New(169, 0)).Price().Equal(decimal.New(170, 0)) ||
		!os.SmallestGreaterThan(decimal.New(150, 0)).Price().Equal(decimal.New(160, 0)) ||
		os.SmallestGreaterThan(decimal.New(170, 0)) != nil {
		t.Fatal("GreaterThan return invalid price")
	}

	t.Log(os.LargestLessThan(decimal.New(101, 0)))
	t.Log(os.SmallestGreaterThan(decimal.New(169, 0)))
}
