// Package ipv4 contains the implementation of the ipv4 network protocol. To use
// it in the networking stack, this package must be added to the project, and
// activated on the stack by passing ipv4.ProtocolName (or "ipv4") as one of the
// network protocols when calling stack.New(). The endpoins can be created by passing
// ipv4.ProtocolNumber as the network protocol number when calling protocol.NewEndpoint().
package ipv4

import (
	"log"

	"github.com/YaoZengzeng/yustack/header"
	"github.com/YaoZengzeng/yustack/stack"
	"github.com/YaoZengzeng/yustack/types"
	"github.com/YaoZengzeng/yustack/buffer"
)

const (
	// ProtocolName is the string representation of the ipv4 protocol name.
	ProtocolName = "ipv4"

	// ProtocolNumber is the ipv4 protocol number.
	ProtocolNumber = header.IPv4ProtocolNumber
)

type address [header.IPv4AddressSize]byte

type endpoint struct {
	nicid 			types.NicId
	id 				types.NetworkEndpointId
	address 		address
	linkEp 			types.LinkEndpoint
	dispatcher 		types.TransportDispatcher
	echoRequests	chan echoRequest
}

func newEndpoint(nicid types.NicId, addr types.Address, dispatcher types.TransportDispatcher, linkEp types.LinkEndpoint) *endpoint {
	e := &endpoint{
		nicid:			nicid,
		linkEp:			linkEp,
		dispatcher:		dispatcher,
		echoRequests:	make(chan echoRequest, 10),
	}
	copy(e.address[:], addr)
	e.id = types.NetworkEndpointId{types.Address(e.address[:])}

	return e
}

// Id returns the ipv4 endpoint Id
func (e *endpoint) Id() *types.NetworkEndpointId {
	return &e.id
}

// HandlePacket is called by the link layer when new ipv4 packets arrive for
// this endpoint
func (e *endpoint) HandlePacket(r *types.Route, vv *buffer.VectorisedView) {
	log.Printf("HandlePacket has not implemented\n")
}

type protocol struct{}

// NewProtocol creates a new ipv4 protocol descriptor. This is exported only for tests
// that short-circuit the stack. Regular use of the protocol is done via the stack, which
// gets a protocol descriptor from the init() function below.
func NewProtocol() types.NetworkProtocol {
	return &protocol{}
}

// Number returns the ipv4 protocol number
func (p *protocol) Number() types.NetworkProtocolNumber {
	return ProtocolNumber
}

// MinimumPacketSize returns the minimum valid ipv4 packet size
func (p *protocol) MinimumPacketSize() int {
	return header.IPv4MinimumSize
}

// ParseAddresses implements NetworkProtocol.ParseAddresses
func (p *protocol) ParseAddresses(v buffer.View) (src, dst types.Address) {
	h := header.IPv4(v)
	return h.SourceAddress(), h.DestinationAddress()
}

// NewEndpoint creates a new ipv4 endpoint
func (p *protocol) NewEndpoint(nicid types.NicId, addr types.Address, dispatcher types.TransportDispatcher, linkEp types.LinkEndpoint) (types.NetworkEndpoint, error) {
	return newEndpoint(nicid, addr, dispatcher, linkEp), nil
}

func init() {
	stack.RegisterNetworkProtocolFactory(ProtocolName, func() types.NetworkProtocol {
		return &protocol{}
	})
}
