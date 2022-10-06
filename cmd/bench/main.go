package main

import (
	"flag"
	"fmt"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/geseq/orderbook"
	decimal "github.com/geseq/udecimal"
)

type EmptyNotification struct {
}

func (e *EmptyNotification) PutOrder(o orderbook.OrderNotification) {
}

func (e *EmptyNotification) PutTrade(o orderbook.Trade) {
}

func getPrice(bid, ask, diff decimal.Decimal, dec bool) (decimal.Decimal, decimal.Decimal) {
	if dec {
		bid = bid.Sub(diff)
		ask = ask.Sub(diff)
		return bid, ask
	}

	bid = bid.Add(diff)
	ask = ask.Add(diff)
	return bid, ask
}

func main() {
	seed := flag.Int64("seed", time.Now().UnixNano(), "rand seed")
	duration := flag.Int("duration", 0, "benchmark duration in seconds")
	lb := flag.String("l", "50.0", "lower bound")
	ub := flag.String("u", "100.0", "upper bound")
	ms := flag.String("m", "0.25", "min spread")
	pd := flag.Uint64("p", 10, "print duration in seconds")
	flag.Parse()

	rand := rand.New(rand.NewSource(*seed))

	lowerBound := decimal.MustParse(*lb)
	upperBound := decimal.MustParse(*ub)
	minSpread := decimal.MustParse(*ms)

	bid := lowerBound.Add(upperBound).Div(decimal.NewI(2, 0))
	ask := bid.Sub(minSpread)
	bidQty := decimal.NewI(10, 0)
	askQty := decimal.NewI(10, 0)

	on := &EmptyNotification{}
	ob := orderbook.NewOrderBook(on)

	var tok, buyID, sellID uint64
	var ops uint64

	start := time.Now()
	end := time.Now().Add(time.Duration(*duration) * time.Second)
	for time.Now().Before(end) {
		var r = rand.Intn(10)
		dec := r < 5

		bid, ask = getPrice(bid, ask, minSpread, dec)
		if bid.LessThan(lowerBound) {
			bid, ask = getPrice(bid, ask, minSpread, false)
		} else if bid.GreaterThan(upperBound) {
			bid, ask = getPrice(bid, ask, minSpread, true)
		}

		tok = tok + 1
		ob.CancelOrder(tok, buyID)
		tok = tok + 1
		ob.CancelOrder(tok, sellID)
		tok = tok + 1
		buyID = tok
		tok = tok + 1
		sellID = tok
		ob.ProcessOrder(buyID, buyID, orderbook.Limit, orderbook.Buy, bidQty, bid, decimal.Zero, orderbook.None)
		ob.ProcessOrder(sellID, sellID, orderbook.Limit, orderbook.Sell, askQty, ask, decimal.Zero, orderbook.None)
		atomic.AddUint64(&ops, 4) // 4 cancels and adds

		if time.Now().Sub(start).Seconds() > 10 {
			fmt.Printf("ops/s: %d\n", ops/(*pd))
			ops = 0
			start = time.Now()
		}
	}
}
