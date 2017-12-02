package tcp

import (
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
