package orderbook

//go:generate gotemplate "github.com/geseq/orderbook/pool" orderPool(Order)
//go:generate gotemplate "github.com/geseq/orderbook/pool" orderQueuePool(orderQueue)
//go:generate gotemplate "github.com/geseq/orderbook/pool" nodeTreePool(nodeTree)
//go:generate gotemplate "github.com/geseq/orderbook/pool" orderTreeNodePool(nodeOrderTree)
