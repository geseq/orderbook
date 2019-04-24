package orderbook

import (
	"encoding/json"
	"testing"

	decimal "github.com/geseq/udecimal"
	"github.com/stretchr/testify/assert"
)

func TestNewOrder(t *testing.T) {
	t.Log(NewOrder(1, Limit, Sell, decimal.New(100, 0), decimal.New(100, 0), decimal.Zero, None))
	t.Log(NewOrder(2, Market, Sell, decimal.New(100, 0), decimal.New(100, 0), decimal.Zero, None))
}

func TestOrderCompose(t *testing.T) {
	data := []*Order{
		NewOrder(24324234, Limit, Buy, decimal.New(11, -1), decimal.New(11, 1), decimal.Zero, None),
		NewOrder(3634345, Limit, Buy, decimal.New(11, -1), decimal.New(11, 1), decimal.New(22, 1), None),
		NewOrder(4123412, Limit, Buy, decimal.New(22, -1), decimal.New(22, 1), decimal.Zero, AoN),
		NewOrder(830459304501, Limit, Sell, decimal.New(33, -1), decimal.New(33, 1), decimal.Zero, FoK),
		NewOrder(237823742802, Limit, Sell, decimal.New(44, -1), decimal.New(44, 1), decimal.Zero, IoC),
	}

	var result = [][]byte{}
	for _, order := range data {
		result = append(result, order.Compose())
	}

	t.Log(result)

	resultDec := []*Order{}

	for _, b := range result {
		var ord Order
		err := ord.Decompose(b)
		if err != nil {
			t.Log(string(b))
			t.Fatal(err)
		}
		resultDec = append(resultDec, &ord)
	}

	for i := 0; i < len(data); i++ {
		db, err := json.Marshal(data[i])
		assert.NoError(t, err)

		rdb, err := json.Marshal(resultDec[i])
		assert.NoError(t, err)

		assert.Equal(t, db, rdb)
	}
}
