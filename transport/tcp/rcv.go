package tcp

import (
	"log"

	"github.com/YaoZengzeng/yustack/seqnum"
)

// receiver holds the state necessary to receive TCP segments and turn them
// into a stream of bytes
type receiver struct {
	ep 	*endpoint

	rcvNxt	seqnum.Value

	// rcvAcc is the one beyond the last acceptable sequence number. That is,
	// the "largest" sequence value that the receiver has announced to the
	// its peer that it's willing to accept. This may be different than
	// rcvNxt + rcvWnd if the receive window is reduced; in that case we have
	// to reduce the window as we receive more data instead of shrinking it
	rcvAcc 	seqnum.Value

	rcvWndScale	uint8

	closed bool

	pendingRcvdSegments	segmentHeap
	pendingBufUsed		seqnum.Size
	pendingBufSize		seqnum.Size
}

func newReceiver(ep *endpoint, irs seqnum.Value, rcvWnd seqnum.Size, rcvWndScale uint8) *receiver {
	return &receiver{
		ep:				ep,
		rcvNxt:			irs + 1,
		rcvAcc:			irs.Add(rcvWnd + 1),
		rcvWndScale:	rcvWndScale,
	}
}

// acceptable checks if the segment sequence number range is acceptable
// according to the table on page 26 of RFC 793
func (r *receiver) acceptable(segSeq seqnum.Value, segLen seqnum.Size) bool {
	rcvWnd := r.rcvNxt.Size(r.rcvAcc)
	if rcvWnd == 0 {
		return segLen == 0 && segSeq == r.rcvNxt
	}

	return segSeq.InWindow(r.rcvNxt, rcvWnd) ||
		seqnum.Overlap(r.rcvNxt, rcvWnd, segSeq, segLen)
}

// consumeSegment attempts to consume a segment that was received by r. The
// segment may have just been received or may have been received earlier but
// wasn't ready to be consumed then
//
// Returns true if the segment was consumed, false if it cannot be consumed
// yet because of a missing segment
func (r *receiver) consumeSegment(s *segment, segSeq seqnum.Value, segLen seqnum.Size) bool {
	if segLen > 0 {
		// If the segment doesn't include the seqnum we're expecting to
		// consume now, we're missing a segment. We cannot proceed until
		// we receive that segment though
		if !r.rcvNxt.InWindow(segSeq, segLen) {
			return false
		}

		// Trim segment to eliminate already acknowledged data
		if segSeq.LessThan(r.rcvNxt) {
			diff := segSeq.Size(r.rcvNxt)
			segLen -= diff
			segSeq.UpdateForward(diff)
			s.sequenceNumber.UpdateForward(diff)
			s.data.TrimFront(int(diff))
		}
		// Move segment to ready-to-deliver list. Wakeup any waiters
		r.ep.readyToRead(s)
	} else if segSeq != r.rcvNxt {
		return false
	}

	return  true
}

// getSendParams returns the parameters needed by the sender when building
// segments to send
func (r *receiver) getSendParams() (rcvNxt seqnum.Value, rcvWnd seqnum.Size) {
	// Calculaten the window size based on the current buffer size
	n := r.ep.receiveBufferAvailable()
	acc := r.rcvNxt.Add(seqnum.Size(n))
	if r.rcvAcc.LessThan(acc) {
		r.rcvAcc = acc
	}

	return r.rcvNxt, r.rcvNxt.Size(r.rcvAcc) >> r.rcvWndScale
}

// handleRcvdSegment handles TCP segments directed at the connection managed by
// r as they arrive. It is called by the protocol main loop
func (r *receiver) handleRcvdSegment(s *segment) {
	// We don't care about receiving processing anymore if the receive side
	// is closed
	if r.closed {
		return
	}

	segLen := seqnum.Size(s.data.Size())
	segSeq := s.sequenceNumber

	// If the sequence number range is outside the acceptance range, just
	// send an ACK. This is according to RFC 793, page 37
	if !r.acceptable(segSeq, segLen) {
		log.Printf("receiver.handleRcvdSegment: segment is not acceptable\n")
		return
	}

	// Defer segment processing if it can't be consumed now
	if !r.consumeSegment(s, segSeq, segLen) {
		log.Printf("receiver.handleRcvdSegment: segment can not be consumed\n")
		return
	}
}
