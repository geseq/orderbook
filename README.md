# High Performance Order Book for matching engines

[![Go Report Card](https://goreportcard.com/badge/github.com/geseq/orderbook)](https://goreportcard.com/report/github.com/geseq/orderbook) [![GoDoc](https://godoc.org/github.com/geseq/orderbook?status.svg)](https://godoc.org/github.com/geseq/orderbook)  [![Go](https://github.com/geseq/orderbook/actions/workflows/go.yml/badge.svg)](https://github.com/geseq/orderbook/actions/workflows/go.yml)


An experimental low latency, high throughput order book in Go.

## Why?

Becuase it's fun!

## Why Go? What about GC?

This is an experiment to see how far I can take the performance of a full fledged matching engine in Go, and that includes trying to see how much GC will have an affect on latency guarantees, and find ways around them.

## Features

- [x] Simple API
- [x] Standard price-time priority
- [x] Market and limit orders
- [x] Order cancellation. (No in-book updates. Updates will have to be handled with Cancel+Create, and all that entails)
- [x] Stop loss orders
- [x] AoN, IoC, FoK, etc. Probably not trailing stops. They're probably better handled outside the order book.
- [ ] Handle any GC latency shenanigans
- [ ] Extensive tests and benchmarks
- [x] Extremely high throughput. (At the moment 2.7 million Order Add/Cancel per second on 2.5 Ghz Intel Xeon W with 2666 Mhz DDR4)

## Limitations

- 8 decimal places due to decimal library used. This should be fine for most use cases.
- Consider [LMAX Disruptor](https://lmax-exchange.github.io/disruptor/) to maintain the throughput with a matching engine, although this level of thoughput is probably not necessary for most use cases.

## How do I use this?

You probably shouldn't use this as-is. This is an *experimental* project with the sole goal of optimizing latency and throughput at the expense of everything else.

That said, it should be extremely simple to use this. Create an `OrderBook` object as a starting point.

## There's a bug

Please create an issue. Also PRs are welcome!


