package tcp

import (
	"log"
	"time"
	"crypto/rand"

	"github.com/YaoZengzeng/yustack/header"
	"github.com/YaoZengzeng/yustack/seqnum"
	"github.com/YaoZengzeng/yustack/sleep"
	"github.com/YaoZengzeng/yustack/buffer"
	"github.com/YaoZengzeng/yustack/types"
	"github.com/YaoZengzeng/yustack/checksum"
)

// maxSegmentsPerWake is the maximum number of segments to process in the main
// protocol goroutine per wake-up. Yielding [after this number of segments are
// processed] allows other events to be processed as well (e.g., timeouts,
// resets, etc.)
const maxSegmentsPerWake = 100

type handshakeState int

// The following are the possible states of the TCP connection during a 3-way
// handshake. A depiction of the states and transitions can be found in RFC 793
const (
	handshakeSynSent handshakeState = iota
	handshakeSynRcvd
	handshakeCompleted
)

// The following are used to set up sleepers
const (
	wakerForNotification = iota
	wakerForNewSegment
	wakerForResend
)

// handshake holds the state used during a TCP 3-way handshake
type handshake struct {
	ep 		*endpoint
	state 	handshakeState
	active	bool
	flags 	uint8
	ackNum	seqnum.Value

	// iss is the initial send sequence number
	iss 	seqnum.Value

	// rcvWnd is the receive window
	rcvWnd	seqnum.Size

	// sndWnd is the send window
	sndWnd	seqnum.Size

	// mss is the maximum segment size received from the peer
	mss 	uint16

	// sndWndScale is the send window scale. A negative value means no scaling
	// is supported by the peer
	sndWndScale	int

	// rcvWndScale is the receive window scale
	rcvWndScale int
}

func newHandshake(ep *endpoint, rcvWnd seqnum.Size) (handshake, error) {
	h := handshake{
		ep:				ep,
		active:			true,
		rcvWnd:			rcvWnd,
		rcvWndScale:	FindWndScale(rcvWnd),
	}
	if err := h.resetState(); err != nil {
		return handshake{}, err
	}

	return h, nil
}

// FindWndScale determines the window scale to use for the given maximum window
// size
func FindWndScale(wnd seqnum.Size) int {
	if wnd < 0x10000 {
		return 0
	}

	max := seqnum.Size(0xffff)
	s := 0
	for wnd > max && s < header.MaxWndScale {
		s++
		max <<= 1
	}

	return s
}

// resetState resets the state of the handshake object such that it becomes
// ready for a new 3-way handshake
func (h *handshake) resetState() error {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}

	h.state = handshakeSynSent
	h.flags = flagSyn
	h.ackNum = 0
	h.mss = 0
	h.iss = seqnum.Value(uint32(b[0]) | uint32(b[1]) << 8 | uint32(b[2]) << 16 | uint32(b[3]) << 24)

	return nil
}

// resetToSynRcvd resets the state of the handshake object to the SYN-RCVD
// state
func (h *handshake) resetToSynRcvd(iss seqnum.Value, irs seqnum.Value, opts *header.TCPSynOptions) {
	h.active = false
	h.state = handshakeSynRcvd
	h.flags = flagSyn | flagAck
	h.iss = iss
	h.ackNum = irs + 1
	h.mss = opts.MSS
	h.sndWndScale = opts.WS
}

// synRcvdState handles a segment received when the TCP 3-way handshake is in
// the SYN-RCVD state
func (h *handshake) synRcvdState(s *segment) error {
	// We have previously received (and acknowledged) the peer's SYN. If the
	// peer acknowledges our SYN, the handshake is completed
	if s.flagIsSet(flagAck) {
		h.state = handshakeCompleted
		return nil
	}
	return nil
}

// processSegments goes through the segment queue and process up to
// maxSegmentsPerWake (if they're available)
func (h *handshake) processSegments() error {
	for i := 0; i < maxSegmentsPerWake; i++ {
		s := h.ep.segmentQueue.dequeue()
		if s == nil {
			return nil
		}

		var err error
		switch h.state {
		case handshakeSynRcvd:
			err = h.synRcvdState(s)
		case handshakeSynSent:
			log.Printf("processSegments: handshakeSynSent has not implemented yet\n")
		}
		if err != nil {
			log.Printf("processSegments failed: %v\n", err)
			return err
		}

		// We stop processing packets once the handshake is completed,
		// otherwise we may process packets meant to be processed by the
		// main protocol goroutine
		if h.state == handshakeCompleted {
			log.Printf("3-way handshake succeed\n")
			break
		}
	}

	// If the queue is not empty, make sure we'll wake up in the next
	// iteration
	if !h.ep.segmentQueue.empty() {
		h.ep.newSegmentWaker.Assert()
	}

	return nil
}

// execute executes the TCP 3-way handshake
func (h *handshake) execute() error {
	// Initialize the resend timer
	resendWaker := sleep.Waker{}
	timeOut := time.Duration(time.Second)
	rt := time.AfterFunc(timeOut, func() {
		resendWaker.Assert()
	})
	defer rt.Stop()

	// Set up the wakers
	s := sleep.Sleeper{}
	s.AddWaker(&resendWaker, wakerForResend)
	s.AddWaker(&h.ep.notificationWaker, wakerForNotification)
	s.AddWaker(&h.ep.newSegmentWaker, wakerForNewSegment)
	defer s.Done()

	// Send the initial SYN segment and loop until the handshake is
	// completed
	synOpts := header.TCPSynOptions{
		WS:		h.rcvWndScale,
		// TS:		true,
		// TSVal:	h.ep.timestamp(),
		// TSEcr:	h.ep.recentTS,
	}

	// Execute is also called in a listen context so we want to make sure we
	// only send the TS option when we received the TS in the initial SYN
	// if h.state == handshakeSynRcvd {
	//	synOpts.TS = h.ep.sendTSOk
	// }
	sendSynTCP(&h.ep.route, h.ep.id, h.flags, h.iss, h.ackNum, h.rcvWnd, synOpts)
	for h.state != handshakeCompleted {
		switch index, _ := s.Fetch(true); index {
		case wakerForResend:
			timeOut *= 2
			if timeOut > 60 * time.Second {
				return types.ErrTimeout
			}
			rt.Reset(timeOut)
			sendSynTCP(&h.ep.route, h.ep.id, h.flags, h.iss, h.ackNum, h.rcvWnd, synOpts)

		case wakerForNotification:
			log.Printf("handshake execute: wakerForNotification has not implemented yet\n")

		case wakerForNewSegment:
			if err := h.processSegments(); err != nil {
				return err
			}
		}
	}

	return nil
}

func sendSynTCP(r *types.Route, id types.TransportEndpointId, flags byte, seq, ack seqnum.Value, rcvWnd seqnum.Size, opts header.TCPSynOptions) error {
	// The MSS in opts is ignored as this function is called from many places and
	// we don't want every call point being embeded with the MSS calculation.
	// So we just do it here and ignore the MSS value passed in the opts
	mss := r.MTU() - header.TCPMinimumSize
	options := []byte{
		// Initialize the MSS option
		header.TCPOptionMSS, 4, byte(mss >> 8), byte(mss),
	}

	// NOTE: a WS of zero is a valid value and it indicates a scale of 1
	if opts.WS >= 0 {
		// Initialize the WS option
		options = append(options,
			header.TCPOptionWS, 3, uint8(opts.WS), header.TCPOptionNOP)
	}

	return sendTCPWithOptions(r, id, nil, flags, seq, ack, rcvWnd, options)
}

// sendTCPWithOptions sends a TCP segment with the provided options via the
// provided network endpoint and under the provided identity
func sendTCPWithOptions(r *types.Route, id types.TransportEndpointId, data buffer.View, flags byte, seq, ack seqnum.Value, rcvWnd seqnum.Size, opts []byte) error {
	optLen := len(opts)
	// Allocate a buffer for the TCP header
	hdr := buffer.NewPrependable(header.TCPMinimumSize + int(r.MaxHeaderLength()) + optLen)

	if rcvWnd > 0xffff {
		rcvWnd = 0xffff
	}

	// Initialize the header
	tcp := header.TCP(hdr.Prepend(header.TCPMinimumSize + optLen))
	tcp.Encode(&header.TCPFields{
		SrcPort:		id.LocalPort,
		DstPort:		id.RemotePort,
		SeqNum:			uint32(seq),
		AckNum:			uint32(ack),
		DataOffset:		uint8(header.TCPMinimumSize + optLen),
		Flags:			flags,
		WindowSize:		uint16(rcvWnd),
	})
	copy(tcp[header.TCPMinimumSize:], opts)

	length := uint16(hdr.UsedLength())
	xsum := r.PseudoHeaderChecksum(ProtocolNumber)
	if data != nil {
		length += uint16(len(data))
		xsum = checksum.Checksum(data, xsum)
	}

	tcp.SetChecksum(^tcp.CalculateChecksum(xsum, length))
	
	log.Printf("Send SYN segment\n")

	return r.WritePacket(&hdr, data, ProtocolNumber)
}

func parseSynSegmentOptions(s *segment) header.TCPSynOptions {
	synOpts := header.ParseSynOptions(s.options, s.flagIsSet(flagAck))
	if synOpts.TS {
		s.parsedOptions.TSVal = synOpts.TSVal
		s.parsedOptions.TSEcr = synOpts.TSEcr
	}
	return synOpts
}
