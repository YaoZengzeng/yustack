package ipv4

import (
	"log"
	"time"
	"encoding/binary"

	"github.com/YaoZengzeng/yustack/types"
	"github.com/YaoZengzeng/yustack/header"
	"github.com/YaoZengzeng/yustack/buffer"
	"github.com/YaoZengzeng/yustack/stack"
	"github.com/YaoZengzeng/yustack/waiter"
)

// PingProtocolName is a pseudo transport protocol used to handle ping replies
// Use it when constructing a stack that intends to use ipv4.Ping
const PingProtocolName = "icmpv4ping"

// pingProtocolNumber is a fake transport protocol used to
// deliver incoming ICMP echo replies. The ICMP identifier
// number is used as a port number for multiplexing
const pingProtocolNumber types.TransportProtocolNumber = 256 + 11

type echoRequest struct {
	r *types.Route
	v buffer.View
}

func (e *endpoint) echoReplier() {
	for req := range e.echoRequests {
		sendICMPv4(req.r, header.ICMPv4EchoReply, 0, req.v)
	}
}

func sendICMPv4(r *types.Route, typ header.ICMPv4Type, code byte, data buffer.View) error {
	hdr := buffer.NewPrependable(header.ICMPv4MinimumSize + int(r.MaxHeaderLength()))

	icmpv4 := header.ICMPv4(hdr.Prepend(header.ICMPv4MinimumSize))
	icmpv4.SetType(typ)
	icmpv4.SetCode(code)
	icmpv4.SetChecksum(^header.Checksum(icmpv4, header.Checksum(data, 0)))

	return r.WritePacket(&hdr, data, header.ICMPv4ProtocolNumber)
}

func (e *endpoint) handleICMP(r *types.Route, vv *buffer.VectorisedView) {
	v := vv.First()
	if len(v) < header.ICMPv4MinimumSize {
		log.Printf("handleICMP: the packet is not big enough\n")
		return
	}

	h := header.ICMPv4(v)

	switch h.Type() {
	case header.ICMPv4Echo:
		vv.TrimFront(header.ICMPv4MinimumSize)
		e.echoRequests <- echoRequest{r: r, v: vv.ToView()}
	case header.ICMPv4EchoReply:
		e.dispatcher.DeliverTransportPacket(r, pingProtocolNumber, vv)
	}
}

// A Pinger can send echo requests to an address
type Pinger struct {
	Stack			*stack.Stack
	NicId			types.NicId
	Address 		types.Address
	LocalAddress	types.Address // optional
	Wait 			time.Duration // if zero, defaults to 1 second
	Count			uint16		  // if zero, defaults to MaxUint16
}

// Ping sends echo requests to an ICMPv4 endpoint
// Response are streamed to the channel ch
func (p *Pinger) Ping(ch chan<- PingReply) error {
	count := p.Count
	if count == 0 {
		count = 1<<16 - 1
	}
	wait := p.Wait
	if wait == 0 {
		wait = 1 * time.Second
	}

	r, err := p.Stack.FindRoute(p.NicId, p.LocalAddress, p.Address, ProtocolNumber)
	if err != nil {
		return err
	}

	netProtos := []types.NetworkProtocolNumber{ProtocolNumber}
	ep := &pingEndpoint{
		stack:	p.Stack,
		pktCh:	make(chan buffer.View, 1),
	}
	id := types.TransportEndpointId{
		LocalAddress:		r.LocalAddress,
		RemoteAddress:		p.Address,
		// Hardcode local port for simplicity
		LocalPort:			1111,
	}

	err = p.Stack.RegisterTransportEndpoint(p.NicId, netProtos, pingProtocolNumber, id, ep)
	if err != nil {
		return err
	}
	defer p.Stack.UnregisterTransportEndpoint(p.NicId, netProtos, pingProtocolNumber, id)

	v := buffer.NewView(4)
	binary.BigEndian.PutUint16(v[0:], id.LocalPort)

	start := time.Now()

	done := make(chan struct{})
	go func(count int) {
		for ; count > 0; count-- {
			// Maybe block here?
			select {
			case v := <-ep.pktCh:
				seq := binary.BigEndian.Uint16(v[header.ICMPv4MinimumSize + 2 :])
				ch <- PingReply{
					Duration:	time.Since(start) - time.Duration(seq) * wait,
					SeqNumber:	seq,
				}
			}
		}
		close(done)
	}(int(count))
	defer func() { <-done }()

	t := time.NewTicker(wait)
	defer t.Stop()

	for seq := uint16(0); seq < count; seq++ {
		select {
		case <-t.C:
		}

		binary.BigEndian.PutUint16(v[2:], seq)
		sent := time.Now()
		if err := sendICMPv4(r, header.ICMPv4Echo, 0, v); err != nil {
			ch <- PingReply{
				Error:		err,
				Duration:	time.Since(sent),
				SeqNumber:	seq,
			}
		}
	}

	return nil
}

// PingReply summarizes an ICMP echo reply
type PingReply struct {
	Error 		error		// reports any errors sending a ping request
	Duration	time.Duration
	SeqNumber	uint16
}

type pingProtocol struct{}

func (*pingProtocol) NewEndpoint(stack *stack.Stack, netProtocol types.NetworkProtocolNumber, waiterQueue *waiter.Queue) (types.Endpoint, error) {
	return nil, types.ErrNotSupported
}

func (*pingProtocol) Number() types.TransportProtocolNumber {
	return pingProtocolNumber
}

func (*pingProtocol) MinimumPacketSize() int {
	return header.ICMPv4EchoMinimumSize
}

func (*pingProtocol) ParsePorts(v buffer.View) (src, dst uint16, err error) {
	ident := binary.BigEndian.Uint16(v[4:])
	return 0, ident, nil
}

func init() {
	stack.RegisterTransportProtocolFactory(PingProtocolName, func() stack.TransportProtocol {
		return &pingProtocol{}
	})
}

type pingEndpoint struct {
	stack 		*stack.Stack
	pktCh 		chan buffer.View
}

func (e *pingEndpoint) HandlePacket(r *types.Route, id types.TransportEndpointId, vv *buffer.VectorisedView) {
	e.pktCh <- vv.ToView()
}
