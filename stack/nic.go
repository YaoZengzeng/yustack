package stack

import (
	"log"
	"sync"

	"github.com/YaoZengzeng/yustack/types"
	"github.com/YaoZengzeng/yustack/buffer"
)

// Nic represents a "network interface card" to which the
// networking stack is attached
type Nic struct {
	stack 		*Stack
	id			types.NicId
	linkEp		types.LinkEndpoint

	demux		*transportDemuxer

	mu			sync.RWMutex
	endpoints 	map[types.NetworkEndpointId]*referencedNetworkEndpoint
}

func newNic(stack *Stack, id types.NicId, ep types.LinkEndpoint) *Nic {
	return &Nic{
		stack:		stack,
		id:			id,
		linkEp:		ep,
		demux:		newTransportDemuxer(stack),
		endpoints:	make(map[types.NetworkEndpointId]*referencedNetworkEndpoint),
	}
}

// attachLinkEndpoint attaches the Nic to the endpoint, which will enable it
// to start delivering packets
func (n *Nic) attachLinkEndpoint() {
	n.linkEp.Attach(n)
}

// AddAddress adds a new address to n, so that it starts to accepting packets
// targeted at the given address (and network protocol)
func (n *Nic) AddAddress(protocol types.NetworkProtocolNumber, address types.Address) error {
	// Add the endpoint
	n.mu.Lock()
	defer n.mu.Unlock()
	_, err := n.addAddressLocked(protocol, address, false)

	return err
}

func (n *Nic) addAddressLocked(protocol types.NetworkProtocolNumber, addr types.Address, replace bool) (*referencedNetworkEndpoint, error) {
	netProtocol, ok := n.stack.networkProtocols[protocol]
	if !ok {
		log.Printf("addAddressLocked: network protocol %x not exist\n", protocol)
		return nil, types.ErrUnknownProtocol
	}

	// Create the new network endpoint
	ep, err := netProtocol.NewEndpoint(n.id, addr, n, n.linkEp)
	if err != nil {
		log.Printf("addAddressLocked: create network endpoint failed\n")
		return nil, err
	}

	id := *ep.Id()
	ref := newReferencedNetworkEndpoint(ep, protocol, n)

	n.endpoints[id] = ref

	return ref, nil
}

type referencedNetworkEndpoint struct {
	ep 			types.NetworkEndpoint
	nic 		*Nic
	protocol 	types.NetworkProtocolNumber
}

func newReferencedNetworkEndpoint(ep types.NetworkEndpoint, protocol types.NetworkProtocolNumber, nic *Nic) *referencedNetworkEndpoint {
	return &referencedNetworkEndpoint{
		ep:			ep,
		nic:		nic,
		protocol:	protocol, 	
	}
}

// DeliverNetworkPacket finds the appropriate network protocol endpoint and
// hands the packet over for further processing. This function is called when
// the Nic receives a packet from the physical interface
// Note that the ownership of the slice backing vv is retained by the caller
// This rule applies only to the slice itself, not to the items of the slice
// the ownership of the items is not retained by the caller
func (n *Nic) DeliverNetworkPacket(linkEp types.LinkEndpoint, remoteLinkAddr types.LinkAddress, protocol types.NetworkProtocolNumber, vv *buffer.VectorisedView) {
	netProtocol, ok := n.stack.networkProtocols[protocol]
	if !ok {
		log.Printf("DeliverNetworkPacket: protocol %x not exist\n", protocol)
		return
	}

	if len(vv.First()) < netProtocol.MinimumPacketSize() {
		log.Printf("DeliverNetworkPacket: packet is not big enough\n")
		return
	}

	src, dst := netProtocol.ParseAddresses(vv.First())
	id := types.NetworkEndpointId{types.Address(dst)}

	// Lock here
	ref, ok := n.endpoints[id]
	if !ok {
		log.Printf("DeliverNetworkPacket: network protocol endpoint not exist\n")
		return
	}

	r := types.MakeRoute(protocol, dst, src, ref.ep)
	r.LocalLinkAddress = linkEp.LinkAddress()
	r.RemoteLinkAddress = remoteLinkAddr

	// Corresponding network endpoint handling the packet
	ref.ep.HandlePacket(r, vv)
}

// DeliverTransportPacket delivers the packets to the appropriate transport
// protocol endpoint
func (n *Nic) DeliverTransportPacket(r *types.Route, protocol types.TransportProtocolNumber, vv *buffer.VectorisedView) {
	state, ok := n.stack.transportProtocols[protocol]
	if !ok {
		log.Printf("DeliverTransportPacket: protocol not found, drop\n")
		return
	}

	transProtocol := state.Protocol
	if len(vv.First())	 < transProtocol.MinimumPacketSize() {
		log.Printf("DeliverTransportPacket: packet is not big enough, drop\n")
		return
	}

	srcPort, dstPort, err := transProtocol.ParsePorts(vv.First())
	if err != nil {
		log.Printf("DeliverTransportPacket: parse ports failed, drop\n")
		return
	}

	id := types.TransportEndpointId{dstPort, r.LocalAddress, srcPort, r.RemoteAddress}
	if n.demux.deliverPacket(r, protocol, vv, id) {
		return
	}
	if n.stack.demux.deliverPacket(r, protocol, vv, id) {
		return
	}

	log.Printf("DeliverTransportPacket: deliver packet failed, drop\n")
}

// primaryEndpoint returns the primary endpoint of nic
func (n *Nic) primaryEndpoint() *referencedNetworkEndpoint {
	for _, r := range n.endpoints {
		return r
	}

	return nil
}
