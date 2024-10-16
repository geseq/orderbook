package orderbook

import (
	"flag"
	"math/rand"
	"runtime"
	"testing"
	"time"

	"fortio.org/fortio/stats"
	decimal "github.com/geseq/udecimal"
	"github.com/loov/hrtime"
)

var (
	lowerBound = decimal.MustParse("50.0")
	upperBound = decimal.MustParse("100.0")
	minSpread  = decimal.MustParse("0.25")
	duration   = 30 * time.Second
)

func init() {
	testing.Init()
	flag.Parse()
}

func BenchmarkLatency(b *testing.B) {
	b.ResetTimer()
	addHist, cancelHist := runBenchmarkLatency(b, duration, lowerBound, upperBound, minSpread)
	b.StopTimer()

	printResultsWithPercentiles(b, "AddOrder", addHist)
	printResultsWithPercentiles(b, "CancelOrder", cancelHist)
}

func runBenchmarkLatency(b *testing.B, duration time.Duration, lowerBound, upperBound, minSpread decimal.Decimal) (*stats.Histogram, *stats.Histogram) {
	seed := time.Now().UnixNano()
	ob := getOrderBook()
	bid, ask, bidQty, askQty := getInitialVars(lowerBound, upperBound, minSpread)

	var tok, buyID, sellID uint64
	rand := rand.New(rand.NewSource(seed))

	addHist := stats.NewHistogram(10, 1)
	cancelHist := stats.NewHistogram(10, 1)

	b.ReportAllocs()

	iterations := uint64(duration.Seconds()) * 2_000_000 // to avoid making clock calls in a loop, assuming roughly 500 nanos per iteration (4 operations)

	loopStart := hrtime.TSC()
	for i := uint64(0); i < iterations; i++ {
		var r = rand.Intn(10)
		dec := r < 5

		bid, ask = getPrice(bid, ask, minSpread, dec)
		if bid.LessThan(lowerBound) {
			bid, ask = getPrice(bid, ask, minSpread, false)
		} else if bid.GreaterThan(upperBound) {
			bid, ask = getPrice(bid, ask, minSpread, true)
		}

		tok++
		start := hrtime.TSC()
		ob.CancelOrder(tok, buyID)
		cancelHist.Record(float64(hrtime.TSCSince(start).ApproxDuration()))

		tok++
		start = hrtime.TSC()
		ob.CancelOrder(tok, sellID)
		cancelHist.Record(float64(hrtime.TSCSince(start).ApproxDuration()))

		tok++
		buyID = tok
		tok++
		sellID = tok

		start = hrtime.TSC()
		ob.AddOrder(buyID, buyID, Limit, Buy, bidQty, bid, decimal.Zero, None)
		addHist.Record(float64(hrtime.TSCSince(start).ApproxDuration()))

		start = hrtime.TSC()
		ob.AddOrder(sellID, sellID, Limit, Sell, askQty, ask, decimal.Zero, None)
		addHist.Record(float64(hrtime.TSCSince(start).ApproxDuration()))
	}

	elapsed := hrtime.TSCSince(loopStart).ApproxDuration()
	b.ReportMetric(float64(iterations)/elapsed.Seconds(), "ops/sec")

	return addHist, cancelHist
}

func BenchmarkThroughput(b *testing.B) {
	b.ResetTimer()
	throughput, avgLatency := runBenchmarkThroughput(b, duration, lowerBound, upperBound, minSpread)
	b.StopTimer()

	b.ReportMetric(throughput, "ops/sec")
	b.ReportMetric(avgLatency, "ns/op")
}

func runBenchmarkThroughput(b *testing.B, duration time.Duration, lowerBound, upperBound, minSpread decimal.Decimal) (float64, float64) {
	seed := time.Now().UnixNano()
	ob := getOrderBook()
	bid, ask, bidQty, askQty := getInitialVars(lowerBound, upperBound, minSpread)

	var tok, buyID, sellID uint64
	var operations int
	rand := rand.New(rand.NewSource(seed))

	b.ReportAllocs()

	iterations := uint64(duration.Seconds()) * 2_000_000 // to avoid making clock calls in a loop, assuming roughly 500 nanos per iteration (4 operations)

	start := hrtime.TSC()

	for i := uint64(0); i < iterations; i++ {
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

		ob.AddOrder(buyID, buyID, Limit, Buy, bidQty, bid, decimal.Zero, None)
		ob.AddOrder(sellID, sellID, Limit, Sell, askQty, ask, decimal.Zero, None)
		operations += 4
	}

	elapsed := hrtime.TSCSince(start).ApproxDuration()
	throughput := float64(operations) / elapsed.Seconds()
	avgLatency := float64(elapsed.Nanoseconds()) / float64(operations)

	return throughput, avgLatency
}

func printResultsWithPercentiles(b *testing.B, operationName string, hist *stats.Histogram) {
	percentiles := []float64{50, 75, 90, 95, 99, 99.9, 99.99, 99.9999, 100}
	b.Logf("Operation: %s", operationName)
	for _, p := range percentiles {
		value := hist.Export().CalcPercentile(p)
		b.Logf("%v: %f ns", p, value)
	}
}

func getOrderBook() *OrderBook {
	on := &EmptyNotification{}
	ob := NewOrderBook(on)

	runtime.GC()
	runtime.Gosched()
	return ob
}

func getInitialVars(lowerBound, upperBound, minSpread decimal.Decimal) (bid decimal.Decimal, ask decimal.Decimal, bidQty decimal.Decimal, askQty decimal.Decimal) {
	bid = lowerBound.Add(upperBound).Div(decimal.NewI(2, 0))
	ask = bid.Sub(minSpread)
	bidQty = decimal.NewI(10, 0)
	askQty = decimal.NewI(10, 0)

	return
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
