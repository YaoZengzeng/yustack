package stack

import (
	"log"
	"sync"

	"github.com/YaoZengzeng/yustack/types"
	"github.com/YaoZengzeng/yustack/buffer"
)

// transportEndpoints manages all endpoints of a given protocol. It has its own
// mutex so as to reduce interference between protocols
type transportEndpoints struct {
	mu 			sync.RWMutex
	endpoints 	map[types.TransportEndpointId]types.TransportEndpoint
}

type protocolIds struct {
	network 		types.NetworkProtocolNumber
	transport 		types.TransportProtocolNumber
}

// transportDemuxer demultiplexes packets targeted at a transport endpoint
// (i.e., after they've parsed by the network layer). It does tow levels
// of demultiplexing: first based on the network and transport protocols, then
// based on endpoints Ids
type transportDemuxer struct {
	protocol map[protocolIds]*transportEndpoints
}

func newTransportDemuxer(stack *Stack) *transportDemuxer {
	d := &transportDemuxer{
		protocol:	make(map[protocolIds]*transportEndpoints),
	}

	// Add each network and transport pair to the demuxer
	for netProto := range stack.networkProtocols {
		for transProto := range stack.transportProtocols {
			d.protocol[protocolIds{netProto, transProto}] = &transportEndpoints{endpoints: make(map[types.TransportEndpointId]types.TransportEndpoint)}
		}
	}

	return d
}

// registerEndpoint registers the given endpoint  with the dispatcher such that
// packets that match the endpoint Id are delivered to it
func (d *transportDemuxer) registerEndpoint(netProtos []types.NetworkProtocolNumber, protocol types.TransportProtocolNumber, id types.TransportEndpointId, ep types.TransportEndpoint) error {
	for i, n := range netProtos {
		if err := d.singleRegisterEndpoint(n, protocol, id, ep); err != nil {
			d.unregisterEndpoint(netProtos[:i], protocol, id)
			return err
		}
	}

	return nil
}

func (d *transportDemuxer) singleRegisterEndpoint(netProto types.NetworkProtocolNumber, protocol types.TransportProtocolNumber, id types.TransportEndpointId, ep types.TransportEndpoint) error {
	eps, ok := d.protocol[protocolIds{netProto, protocol}]
	if !ok {
		log.Printf("singleRegisterEndpoint: find endpoints failed\n")
		return nil
	}

	eps.mu.Lock()
	defer eps.mu.Unlock()

	if _, ok := eps.endpoints[id]; ok {
		return types.ErrPortInUse
	}

	eps.endpoints[id] =  ep

	return nil
}

// unregisterEndpoint unregisters the endpoint with the given id such that it
// won't receive any more packets
func (d *transportDemuxer) unregisterEndpoint(netProtos []types.NetworkProtocolNumber, protocol types.TransportProtocolNumber, id types.TransportEndpointId) {
	for _, n := range netProtos {
		if eps, ok := d.protocol[protocolIds{n, protocol}]; ok {
			eps.mu.Lock()
			delete(eps.endpoints, id)
			eps.mu.Unlock()
		}
	}
}

// deliverPacket attempts to deliver the given packet. Returns true if it found
// an endpoint, false otherwise
func (d *transportDemuxer) deliverPacket(r *types.Route, protocol types.TransportProtocolNumber, vv *buffer.VectorisedView, id types.TransportEndpointId) bool {
	eps, ok := d.protocol[protocolIds{r.NetProto, protocol}]
	if !ok {
		log.Printf("deliverPacket: found transport endpoints failed\n")
		return false
	}

	eps.mu.RLock()
	defer eps.mu.RUnlock()
	b := d.deliverPacketLocked(r, eps, vv, id)

	return b
}

func (d *transportDemuxer) deliverPacketLocked(r *types.Route, eps *transportEndpoints, vv *buffer.VectorisedView, id types.TransportEndpointId) bool {
	// Now only try to match with the id as provided
	if ep := eps.endpoints[id]; ep != nil {
		ep.HandlePacket(r, id, vv)
		return true
	}

	log.Printf("deliverPacketLocked: found transport endpoint failed\n")
	return false
}