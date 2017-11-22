package tmutex

import (
	"sync/atomic"
)

// Mutex is a mutual exclusion primitive that implements TryLock in addition
// to Lock and Unlock
type Mutex struct {
	v 	int32
	ch	chan struct{}
}

// Init initializes the mutex
func (m *Mutex) Init() {
	m.v	= 1
	m.ch = make(chan struct{}, 1)
}

// Lock acquires the mutex. If it is currently held by another goroutine, Lock
// will wait until it has a chance to require it
func (m *Mutex) Lock() {
	for {
		if atomic.CompareAndSwapInt32(&m.v, 1, 0) {
			return
		}
		<-m.ch
	}
}

func (m *Mutex) TryLock() bool {
	return atomic.CompareAndSwapInt32(&m.v, 1, 0)
}

// Unlock releases the mutex
func (m *Mutex) Unlock() {
	atomic.SwapInt32(&m.v, 1)

	// Wake some waiter up
	select {
	case m.ch <- struct{}{}:
	default:
	}
}
