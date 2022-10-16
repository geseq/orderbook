package orderbook

//go:generate gotemplate "github.com/geseq/fastchan" orderChan(*Order)
//go:generate gotemplate "github.com/geseq/fastchan" orderQueueChan(*orderQueue)
//go:generate gotemplate "github.com/geseq/fastchan" nodeTreeChan(*nodeTree)

// A simple order pool.
type OrderPool struct {
	ch *orderChan
}

func NewOrderPool(maxSize uint64) *OrderPool {
	ch := newOrderChan(maxSize)
	p := &OrderPool{
		ch: ch,
	}

	for !ch.IsFull() {
		ch.Put(&Order{})
	}

	return p
}

func (p *OrderPool) Get() *Order {
	if p.ch.IsEmpty() {
		return &Order{}
	}

	return p.ch.Read()
}

func (p *OrderPool) Put(o *Order) {
	if o == nil {
		return
	}

	if p.ch.IsFull() {
		// Leave it to GC
		return
	}

	p.ch.Put(o)
}

// A simple order queue pool.
type OrderQueuePool struct {
	ch *orderQueueChan
}

func NewOrderQueuePool(maxSize uint64) *OrderQueuePool {
	ch := newOrderQueueChan(maxSize)
	p := &OrderQueuePool{
		ch: ch,
	}

	for !ch.IsFull() {
		ch.Put(&orderQueue{})
	}

	return p
}

func (p *OrderQueuePool) Get() *orderQueue {
	if p.ch.IsEmpty() {
		return &orderQueue{}
	}

	return p.ch.Read()
}

func (p *OrderQueuePool) Put(o *orderQueue) {
	if o == nil {
		return
	}

	if p.ch.IsFull() {
		// Leave it to GC
		return
	}

	p.ch.Put(o)
}

// A simple rb tree node pool.
type nodeTreePool struct {
	ch *nodeTreeChan
}

func NewNodeTreePool(maxSize uint64) *nodeTreePool {
	ch := newNodeTreeChan(maxSize)
	p := &nodeTreePool{
		ch: ch,
	}

	for !ch.IsFull() {
		ch.Put(&nodeTree{})
	}

	return p
}

func (p *nodeTreePool) Get() *nodeTree {
	if p.ch.IsEmpty() {
		return &nodeTree{}
	}

	return p.ch.Read()
}

func (p *nodeTreePool) Put(o *nodeTree) {
	if o == nil {
		return
	}

	if p.ch.IsFull() {
		// Leave it to GC
		return
	}

	p.ch.Put(o)
}
