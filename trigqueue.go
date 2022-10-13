package orderbook

// triggerQueue stores and manage chain of orders
type triggerQueue struct {
	size uint64
	head *Order
	tail *Order
}

// newTriggerQueue creates and initialize triggerQueue object
func newTriggerQueue() *triggerQueue {
	return &triggerQueue{}
}

// Len returns amount of orders in queue
func (oq *triggerQueue) Len() uint64 {
	return oq.size
}

// Head returns top order in queue
func (oq *triggerQueue) Head() *Order {
	return oq.head
}

// Append pushes order to tail of the queue
func (oq *triggerQueue) Push(o *Order) {
	tail := oq.tail
	oq.tail = o
	if tail != nil {
		tail.next = o
		o.prev = tail
	}
	if oq.head == nil {
		oq.head = o
	}
	oq.size++
}

// Append pops an order from head of the queue and returns it
func (oq *triggerQueue) Pop() *Order {
	o := oq.head
	if o == nil {
		return nil
	}
	oq.head = o.next
	o.next = nil
	o.prev = nil

	if oq.head == nil {
		oq.tail = nil
	}

	oq.size--
	return o
}

// Remove removes order from the queue and link order chain
func (oq *triggerQueue) Remove(o *Order) *Order {
	prev := o.prev
	next := o.next
	if prev != nil {
		prev.next = next
	}
	if next != nil {
		next.prev = prev
	}
	o.next = nil
	o.prev = nil

	oq.size--
	if oq.head == o {
		oq.head = next
	}
	if oq.tail == o {
		oq.tail = prev
	}
	return o
}
