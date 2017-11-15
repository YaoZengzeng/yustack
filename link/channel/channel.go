package channel

import (
	"github.com/YaoZengzeng/yustack/buffer"
	"github.com/YaoZengzeng/yustack/types"
	"github.com/YaoZengzeng/yustack/stack"
)

// PacketInfo holds all the information about an outbound packet
type PacketInfo struct {
	Header		buffer.View
	Payload		buffer.View
	Protocol	types.NetworkProtocolNumber
}

// Endpoint is link layer endpoint that stores outbound packets in a channel
// and allows injection of inbound packets
type Endpoint struct {
	dispatcher	types.NetworkDispatcher
	mtu			uint32

	C chan PacketInfo
}

// New creates a new channel endpoint
func New(size int, mtu uint32) (types.LinkEndpointID, *Endpoint) {
	e := &Endpoint{
		C:		make(chan PacketInfo, size),
		mtu:	mtu,
	}

	return stack.RegisterLinkEndpoint(e), e
}

// Inject injects an inbound packet
func (e *Endpoint) Inject(protocol types.NetworkProtocolNumber, vv *buffer.VectorisedView) {
	uu := vv.Clone(nil)
	e.dispatcher.DeliverNetworkPacket(e, "", protocol, &uu)
}

// Attach saves the stack network layer dispatcher for use later when packets
// are injected
func (e *Endpoint) Attach(dispatcher types.NetworkDispatcher) {
	e.dispatcher = dispatcher
}

// MTU implements types.LinkEndpoint.MTU. It returns the value initialized
// during construction
func (e *Endpoint) MTU() uint32 {
	return e.mtu
}

// MaxHeaderLength returns the maximum size of the link layer header. Given it
// doesn't have a header, it just returns 0
func (e *Endpoint) MaxHeaderLength() uint16 {
	return 0
}

// LinkAddress returns the link address of this endpoint
func (e *Endpoint) LinkAddress() types.LinkAddress {
	return ""
}

// WritePacket stores outbound packets into the channel
func (e *Endpoint) WritePacket(_ *types.Route, hdr *buffer.Prependable, payload buffer.View, protocol types.NetworkProtocolNumber) error {
	p := PacketInfo{
		Header:		hdr.View(),
		Protocol:	protocol,
	}

	if payload != nil {
		p.Payload = make(buffer.View, len(payload))
		copy(p.Payload, payload)
	}

	e.C <-p

	return nil
}