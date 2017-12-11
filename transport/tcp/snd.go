package tcp

import (
	"log"
	"time"

	"github.com/YaoZengzeng/yustack/seqnum"
	"github.com/YaoZengzeng/yustack/sleep"
	"github.com/YaoZengzeng/yustack/buffer"
)

// sender holds the state necessary to send TCP segments
type sender struct {
	ep *endpoint

	// lastSendTime is the timestamp when the last packet was sent
	lastSendTime time.Time

	// dupAckCount is the number of duplicated acks received. It is used
	// for fast retransmit
	dupAckCount int

	// fr holds state related to fast recovery
	// fr fastRecovery

	// sndCwnd is the congestion window, in packets
	// sndCwnd int

	// sndSsthresh is the threshold between slow start and congestion avoidance
	// sndSsthresh int

	// sndCAAckCount is the number of packets acknowledged during congestion
	// avoidance. When enough packets have been ack'd (typically cwnd packets),
	// the congestion window is incremented by one
	// sndCAAckCount int

	// outstanding is the number of outstanding packets, that is, packets
	// that have been sent but not yet acknowledged
	// outstanding int

	// sndWnd is the send window size
	sndWnd seqnum.Size

	// sndUna is the next unacknowledged sequence number
	sndUna seqnum.Value

	// sndNxt is the sequence number of the next segment to be sent
	sndNxt seqnum.Value

	// sndNxtList is the sequence number of the next segment to be added to
	// the send list
	sndNxtList seqnum.Value

	// rttMeasureSeqNum is the sequence number being used for the latest RTT
	// measurement
	rttMeasureSeqNum seqnum.Value

	// rttMeasureTime is the time when the rttMeasureSeqNum was sent
	rttMeasureTime time.Time

	closed		bool
	writeNext	*segment
	writeList	segmentList
	// resendTimer	timer
	resendWaker	sleep.Waker

	// srtt, rttval & rto are the "smoothed round-trip time", "round-trip
	// time variation" and "retransmit timeout", as defined in section 2 of
	// RFC 6289
	srtt 		time.Duration
	rttvar 		time.Duration
	rto 		time.Duration
	srttInited	bool

	// maxPayloadSize is the maximum size of the payload of a given segment
	// It is initialized on demand
	maxPayloadSize int

	// sndWndScale is the number of bits to shift left when reading the send
	// window size from a segment
	sndWndScale uint8

	// maxSentAck is the maximum acknowledged actually sent
	maxSentAck seqnum.Value
}

func newSender(ep *endpoint, iss, irs seqnum.Value, sndWnd seqnum.Size, mss uint16, sndWndScale int) *sender {
	s := &sender{
		ep:			ep,
		sndWnd:		sndWnd,
		sndUna:		iss + 1,
		sndNxt:		iss + 1,
		sndNxtList:	iss + 1,
		rto:		1 * time.Second,
		rttMeasureSeqNum:	iss + 1,
		lastSendTime:		time.Now(),
		maxPayloadSize:		int(mss),
		maxSentAck:			irs + 1,
	}

	// A negative sndWndScale means that no scaling is in use, otherwise we
	// store the scaling value
	if sndWndScale > 0 {
		s.sndWndScale = uint8(sndWndScale)
	}

	return s
}

// sendAck sends an ACk segment
func (s *sender) sendAck() {
	s.sendSegment(nil, flagAck, s.sndNxt)
}

// sendData sends new data segments. It is called when data becomes available or
// when the send window opens up
func (s *sender) sendData() {
	// TODO: We currently don't merge multiple send buffers
	// into one segment if they happen to fit. We should do that
	// eventually
	var seg *segment
	end := s.sndUna.Add(s.sndWnd)
	for seg = s.writeNext; seg != nil; seg = seg.Next() {
		// We abuse the flags field to determine if we have already
		// assigned a sequence number to this segment
		if seg.flags == 0 {
			seg.sequenceNumber = s.sndNxt
			seg.flags = flagAck
		}

		var segEnd seqnum.Value
		if seg.data.Size() == 0 {
			// We're sending a FIN
			seg.flags = flagAck | flagFin
			segEnd = seg.sequenceNumber.Add(1)
		} else {
			// We're sending a non-FIN segment
			if !seg.sequenceNumber.LessThan(end) {
				log.Printf("sendData: segment's sequenceNumber is less than sndUna + sndWnd\n")
				break
			}

			available := int(seg.sequenceNumber.Size(end))
			if seg.data.Size() > available {
				log.Printf("the length of segment is longer than available window size\n")
				return
			}

			segEnd = seg.sequenceNumber.Add(seqnum.Size(seg.data.Size()))
		}

		s.sendSegment(&seg.data, seg.flags, seg.sequenceNumber)

		// Update sndNxt if we actually sent data (as opposed to
		// retransmitting some previously sent data)
		if s.sndNxt.LessThan(segEnd) {
			s.sndNxt = segEnd
		}
	}

	// Remember the next segment we'll write
	s.writeNext = seg
}

// handleRcvdSegment is called when a segment is received; it is responsible for
// updating the send-related state
func (s *sender) handleRcvdSegment(seg *segment) {
	// Stash away the current window size
	s.sndWnd = seg.window

	// Ignore ack if it doesn't acknowledge any new data
	ack := seg.ackNumber
	if (ack - 1).InRange(s.sndUna, s.sndNxt) {
		// Remove all acknowledged data from the write list
		acked :=s.sndUna.Size(ack)
		s.sndUna = ack

		ackLeft := acked
		for ackLeft > 0 {
			// We use logicalLen here because we can have FIN
			// segments (which are always at the end of list) that
			// have no data, but do consume a segment number
			seg := s.writeList.Front()
			dataLen := seg.logicalLen()

			if dataLen > ackLeft {
				seg.data.TrimFront(int(ackLeft))
				break
			}

			s.writeList.Remove(seg)
			ackLeft -= dataLen
		}
	}
}

// sendSegment sends a new segment containing the given payload, flags and
// sequence number
func (s *sender) sendSegment(data *buffer.VectorisedView, flags byte, seq seqnum.Value) error {
	rcvNxt, rcvWnd := s.ep.rcv.getSendParams()

	if data == nil {
		return s.ep.sendRaw(nil, flags, seq, rcvNxt, rcvWnd)
	}

	if len(data.Views()) > 1 {
		panic("send path does not support views with multiple buffers")
	}

	return s.ep.sendRaw(data.First(), flags, seq, rcvNxt, rcvWnd)
}
