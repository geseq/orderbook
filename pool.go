package orderbook

//go:generate gotemplate "github.com/geseq/orderbook/pool" OrderPool(Order)
//go:generate gotemplate "github.com/geseq/orderbook/pool" orderQueuePool(orderQueue)
//go:generate gotemplate "github.com/geseq/orderbook/pool" nodeTreePool(nodeTree)
