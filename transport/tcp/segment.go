package tcp

import (
	"github.com/YaoZengzeng/yustack/buffer"
	"github.com/YaoZengzeng/yustack/seqnum"
	"github.com/YaoZengzeng/yustack/types"
	"github.com/YaoZengzeng/yustack/header"
)

// Flags that may be set in a TCP segment.
const (
	flagFin = 1 << iota
	flagSyn
	flagRst
	flagPsh
	flagAck
	flagUrg
)

// segment represents a TCP segment. It holds the payload and parsed TCP segment
// information, and can be added to intrusive lists.
type segment struct {
	segmentEntry
	refCnt int32
	id     types.TransportEndpointId
	route  types.Route
	data   buffer.VectorisedView
	// views is used as buffer for data when its length is large
	// enough to store a VectorisedView.
	views [8]buffer.View
	// viewToDeliver keeps track of the next View that should be
	// delivered by the Read endpoint.
	viewToDeliver  int
	sequenceNumber seqnum.Value
	ackNumber      seqnum.Value
	flags          uint8
	window         seqnum.Size

	// parsedOptions stores the parsed values from the options in the segment.
	parsedOptions header.TCPOptions
	options       []byte
}

func newSegment(r *types.Route, id types.TransportEndpointId, vv *buffer.VectorisedView) *segment {
	s := &segment{
		refCnt: 1,
		id:     id,
		route:  r.Clone(),
	}
	s.data = vv.Clone(s.views[:])
	return s
}

func newSegmentFromView(r *types.Route, id types.TransportEndpointId, v buffer.View) *segment {
	s := &segment{
		id:		id,
		route:	r.Clone(),
	}
	s.views[0] = v
	s.data = buffer.NewVectorisedView(s.views[:1], len(v))
	return s
}

// parse populates the sequence & ack numbers, flags, and window fields of the
// segment from the TCP header stored in the data. It then updates the view to
// skip the data. Returns boolean indicating if the parsing was successful.
// parse从tcp header中解析出sequence以及ack numbers，flags，以及window，跳过data，更新view
func (s *segment) parse() bool {
	h := header.TCP(s.data.First())

	// h is the header followed by the payload. We check that the offset to
	// the data respects the following constraints:
	// 1. That it's at least the minimum header size; if we don't do this
	//    then part of the header would be delivered to user.
	// 2. That the header fits within the buffer; if we don't do this, we
	//    would panic when we tried to access data beyond the buffer.
	//
	// N.B. The segment has already been validated as having at least the
	//      minimum TCP size before reaching here, so it's safe to read the
	//      fields.
	offset := int(h.DataOffset())
	if offset < header.TCPMinimumSize || offset > len(h) {
		return false
	}

	s.options = []byte(h[header.TCPMinimumSize : offset])
	s.parsedOptions = header.ParseTCPOptions(s.options)
	s.data.TrimFront(offset)

	s.sequenceNumber = seqnum.Value(h.SequenceNumber())
	s.ackNumber = seqnum.Value(h.AckNumber())
	s.flags = h.Flags()
	s.window = seqnum.Size(h.WindowSize())

	return true
}

func (s *segment) flagIsSet(flag uint8) bool {
	return (s.flags & flag) != 0
}
