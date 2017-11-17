// Package stack provides the glue between networking protocols and the
// consumers of the networking stack.

package stack

import (
	"sync"
	"log"

	"github.com/YaoZengzeng/yustack/types"
	"github.com/YaoZengzeng/yustack/waiter"
	"github.com/YaoZengzeng/yustack/ports"
)

// Stack is a networking stack, with all supported protocols, NICs, and route table.
type Stack struct {
	networkProtocols map[types.NetworkProtocolNumber]types.NetworkProtocol
	transportProtocols map[types.TransportProtocolNumber]*TransportProtocolState

	demux			*transportDemuxer

	mu				sync.RWMutex
	nics 			map[types.NicId]*Nic

	// route is the route table passed in by the user via SetRouteTable(),
	// it is used by FindRoute() to build a route for a specific destination
	routeTable 		[]types.RouteEntry

	*ports.PortManager
}

// New allocates a new networking stack with only the requested networking and
// transport protocols configured with default options.
func New(network []string, transport []string) *Stack {
	s := &Stack{
		networkProtocols: 	make(map[types.NetworkProtocolNumber]types.NetworkProtocol),
		transportProtocols:	make(map[types.TransportProtocolNumber]*TransportProtocolState),
		nics:			  	make(map[types.NicId]*Nic),
		PortManager:		ports.NewPortManager(),
	}

	// Add specified network protocols.
	for _, name := range network {
		netProtocolFactory, ok := networkProtocols[name]
		if !ok {
			continue
		}
		netProtocol := netProtocolFactory()
		s.networkProtocols[netProtocol.Number()] = netProtocol
	}

	// Add specified transport protocols
	for _, name := range transport {
		transProtocolFactory, ok := transportProtocols[name]
		if !ok {
			continue
		}
		transProtocol := transProtocolFactory()
		s.transportProtocols[transProtocol.Number()] = &TransportProtocolState{
			Protocol:	transProtocol,
		}
	}

	// Create the global transport demuxer
	s.demux = newTransportDemuxer(s)

	return s
}

// createNic creates a Nic with the porvided id and link layer endpoint
// and optionally enable it
func (s *Stack) createNic(id types.NicId, linkEpId types.LinkEndpointID, enable bool) error {
	linkEp := FindLinkEndpoint(linkEpId)
	if linkEp == nil {
		return types.ErrBadLinkEndpoint
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Make sure id is unique
	if _, ok := s.nics[id]; ok {
		return types.ErrDuplicateNicId
	}

	nic := newNic(s, id, linkEp)
	s.nics[id] = nic

	if enable {
		nic.attachLinkEndpoint()
	}

	return nil
}

// CraeteNic creates a NIC with the provided id and link layer endpoint
func (s *Stack) CreateNic(id types.NicId, linkEpId types.LinkEndpointID) error {
	return s.createNic(id, linkEpId, true)
}

// AddAddress adds a new network layer address to the specific Nic
func (s *Stack) AddAddress(id types.NicId, protocol types.NetworkProtocolNumber, address types.Address) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nic := s.nics[id]
	if nic == nil {
		return types.ErrUnknownNicId
	}

	return nic.AddAddress(protocol, address)
}

// SetRouteTable assigns the route table to be used by this stack. It
// specifies which Nic and gateway to use for given destination address ranges
func (s *Stack) SetRouteTable(table []types.RouteEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.routeTable = table
}

// RegisterTransportEndpoint registers the given endpoint with the stack
// transport dispatcher. Received packets that match the provided id will be
// delivered to the given endpoint; specifiying a nic is optional, but
// nic-specific Ids have precedence over global ones
func (s *Stack) RegisterTransportEndpoint(nicId types.NicId, netProtos []types.NetworkProtocolNumber, protocol types.TransportProtocolNumber, id types.TransportEndpointId, ep types.TransportEndpoint) error {
	if nicId == 0 {
		return s.demux.registerEndpoint(netProtos, protocol, id, ep)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	nic := s.nics[nicId]
	if nic == nil {
		return types.ErrUnknownNicId
	}

	return nic.demux.registerEndpoint(netProtos, protocol, id, ep)
}

// UnregisterTransportEndpoint removes the endpoint with the given id from the
// stack transport dispatcher
func (s *Stack) UnregisterTransportEndpoint(nicId types.NicId, netProtos []types.NetworkProtocolNumber, protocol types.TransportProtocolNumber, id types.TransportEndpointId) {
	if nicId == 0 {
		s.demux.unregisterEndpoint(netProtos, protocol, id)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	nic := s.nics[nicId]
	if nic != nil {
		nic.demux.unregisterEndpoint(netProtos, protocol, id)
	}
}

// FindRoute creates a route to the given destination address, leaving through
// the given nic and local address (if provided)
func (s *Stack) FindRoute(id types.NicId, localAddress, remoteAddress types.Address, netProto types.NetworkProtocolNumber) (*types.Route, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.routeTable {
		if id != 0 && id != s.routeTable[i].Nic || !s.routeTable[i].Match(remoteAddress) {
			continue
		}

		nic := s.nics[s.routeTable[i].Nic]
		if nic == nil {
			continue
		}

		// Use the first endpoint of given network protocol
		ref := nic.primaryEndpoint()
		if ref == nil {
			log.Printf("FindRoute: can not find network endpoint of given network protocol")
			continue
		}

		r := types.MakeRoute(netProto, ref.ep.Id().LocalAddress, remoteAddress, ref.ep)
		// Ignore remote link address
		r.NextHop = s.routeTable[i].Gateway
		return r, nil
	}

	return &types.Route{}, types.ErrNoRoute
}

// NewEndpoint creates a new transport layer endpoint of the given protocol
func (s *Stack) NewEndpoint(transport types.TransportProtocolNumber, network types.NetworkProtocolNumber, waiterQueue *waiter.Queue) (types.Endpoint, error) {
	t, ok := s.transportProtocols[transport]
	if !ok {
		return nil, types.ErrUnknownProtocol
	}

	return t.Protocol.NewEndpoint(s, network, waiterQueue)
}
