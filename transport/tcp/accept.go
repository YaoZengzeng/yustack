package tcp

import (
	"log"

	"github.com/YaoZengzeng/yustack/stack"
	"github.com/YaoZengzeng/yustack/types"
	"github.com/YaoZengzeng/yustack/seqnum"
	"github.com/YaoZengzeng/yustack/sleep"
)

// listenContext is used by a listening endpoint to store state and used while
// listening for connections. This struct is allocated by the listen goroutine
// and must not be accessed or have its methods called concurrently as they
// may mutate the stored objects
type listenContext struct {
	stack 	*stack.Stack
	rcvWnd	seqnum.Size

	netProtocol 	types.NetworkProtocolNumber
}

// newListenContext creates a new listen context
func newListenContext(stack *stack.Stack, rcvWnd seqnum.Size, netProtocol types.NetworkProtocolNumber) *listenContext {
	l := &listenContext{
		stack:			stack,
		rcvWnd:			rcvWnd,
		netProtocol:	netProtocol,	
	}

	return l
}

// handleListenSegment is called when a listening endpoint receives a segment
// and needs to handle it
func (e *endpoint) handleListenSegment(ctx *listenContext, s *segment) {
	switch s.flags {
	case flagSyn:
		log.Printf("handleListenSegment: flagSyn has not implemented yet\n")

	case flagAck:
		log.Printf("handleListenSegment: flagAck has not implemented yet\n")
	}
}

// protocolListenLoop is the main loop of a listening TCP endpoint. It runs in
// its own goroutine and is responsible for handling connection requests
func (e *endpoint) protocolListenLoop(rcvWnd seqnum.Size) error {
	ctx := newListenContext(e.stack, rcvWnd, e.netProtocol)

	var s sleep.Sleeper
	s.AddWaker(&e.notificationWaker, wakerForNotification)
	s.AddWaker(&e.newSegmentWaker, wakerForNewSegment)
	for {
		switch index, _ := s.Fetch(true); index {
		case wakerForNotification:
			log.Printf("protocolListenLoop: branch wakerForNotification has not implemented yet\n")

		case wakerForNewSegment:
			// Process at most maxSegmentsPerWake segments
			mayRequeue := true
			for i := 0; i < maxSegmentsPerWake; i++ {
				s := e.segmentQueue.dequeue()
				if s == nil {
					mayRequeue = false
					break
				}

				e.handleListenSegment(ctx, s)
			}

			// If the queue is not empty, make sure we'll wake up
			// in the next iteration
			if mayRequeue && !e.segmentQueue.empty() {
				e.newSegmentWaker.Assert()
			}
		}
	}
}
