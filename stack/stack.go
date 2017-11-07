// Package stack provides the glue between networking protocols and the
// consumers of the networking stack.

package stack

import (
	"sync"

	"github.com/YaoZengzeng/yustack/types"
)

// Stack is a networking stack, with all supported protocols, NICs, and route table.
type Stack struct {
	networkProtocols map[types.NetworkProtocolNumber]types.NetworkProtocol

	mu				sync.RWMutex
	nics 			map[types.NicId]*Nic

	// route is the route table passed in by the user via SetRouteTable(),
	// it is used by FindRoute() to build a route for a specific destination
	routeTable 		[]types.Route
}

// New allocates a new networking stack with only the requested networking and
// transport protocols configured with default options.
func New(network []string, transport []string) *Stack {
	s := &Stack{
		networkProtocols: make(map[types.NetworkProtocolNumber]types.NetworkProtocol),
		nics:			  make(map[types.NicId]*Nic),
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
	defer s.mu.Lock()

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
func (s *Stack) SetRouteTable(table []types.Route) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.routeTable = table
}
