// Package waiter provides the implementation of a wait queue, where waiters can
// be enqueued to be notified when an event of interest happens

package waiter

import (
	"sync"

	"github.com/YaoZengzeng/yustack/ilist"
)

// EventMask represents io events as used in the poll() syscall
type EventMask uint16

// Events that waiters can wait on. The meaning is the same as those in the
// poll() syscall
const (
	EventIn		EventMask = 0x01	// syscall.EPOLLIN
	EventPri	EventMask = 0x02	// syscall.EPOLLPRI
	EventOut	EventMask = 0x04	// syscall.EPOLLOUT
	EventErr 	EventMask = 0x08	// syscall.EPOLLERR
	EventHup 	EventMask = 0x10 	// syscall.EPOLLHUP
	EventNVal	EventMask = 0x20 	// Not defined in syscall
)

// EntryCallback provides a notify callback
type EntryCallback interface {
	// Callback is the function to be called when the waiter entry is
	// notified. It is responsible for doing whatever is needed to wake up
	// the waiter
	//
	// The callback is supposed to perform minimal work, and cannot call
	// any method on the queue itself because it will be locked while the
	// callback is running
	Callback(e *Entry)
}

// Entry represents a waiter that can be added to the wait queue. It can
// only be in one queue at a time, and is added "intrusively" to the queue with
// no extra memory allocations
type Entry struct {
	// Context stores any state the waiter may wish to store in the entry
	// itself, which may be used at wake up time
	//
	// Note that use of this field is optional and state may alternatively be
	// stored in the callback itself
	Context interface{}

	Callback EntryCallback

	// The following fields are protected by the queue lock
	mask EventMask
	ilist.Entry
}

type channelCallback struct{}

func (*channelCallback) Callback(e *Entry) {
	ch := e.Context.(chan struct{})
	ch <-struct{}{}
}

// NewChannelEntry initializes a new Entry that does a non-blocking write to a
// struct{} channel when the callback is called. It returns the new Entry
// instance and the channel being used
//
// If a channel isn't specified (i.e., if "c" is nil), then NewChannelEntry
// allocates a new channel
func NewChannelEntry(c chan struct{}) (Entry, chan struct{}) {
	if c == nil {
		c = make(chan struct{}, 1)
	}

	return Entry{Context: c, Callback: &channelCallback{}}, c
}

// Queue represents the wait queue where waiters can be and notifiers
// can notify them when events happen
//
// The zero value for waiter.Queue is an empty queue ready for use
type Queue struct {
	list	ilist.List
	mu 		sync.RWMutex
}

// EventRegister adds a waiter to the wait queue; the waiter will be notified
// when at least one of the events specified in mask happens
func (q *Queue) EventRegister(e *Entry, mask EventMask) {
	q.mu.Lock()
	defer q.mu.Unlock()
	e.mask = mask
	q.list.PushBack(e)
}

// EventUnregister removes the given waiter entry from the wait queue
func (q *Queue) EventUnregister(e *Entry) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.list.Remove(e)
}

// Notify notifies all waiters in the queue whose masks have at least one bit
// in common with the notification mask
func (q *Queue) Notify(mask EventMask) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	for it := q.list.Front(); it != nil; it = it.Next() {
		e := it.(*Entry)
		if (mask & e.mask) != 0 {
			e.Callback.Callback(e)
		}
	}
}
