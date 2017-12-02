package tcp

import (
	"time"

	"github.com/YaoZengzeng/yustack/seqnum"
	"github.com/YaoZengzeng/yustack/sleep"
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
