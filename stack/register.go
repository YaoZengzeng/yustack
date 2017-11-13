package stack

import (
	"sync"

	"github.com/YaoZengzeng/yustack/types"
)

var (
	networkProtocols = make(map[string]types.NetworkProtocolFactory)

	transportProtocols = make(map[string]types.TransportProtocolFactory)

	linkEpMux		sync.RWMutex

	nextLinkEndpointID	types.LinkEndpointID = 1

	linkEndpoints	= make(map[types.LinkEndpointID]types.LinkEndpoint)
)

// RegisterNetworkProtocolFactory registers a new network protocol factory with
// the stack so that it becomes available to users of the stack. This function
// is intended to be called by init() functions of the protocols.
func RegisterNetworkProtocolFactory(name string, p types.NetworkProtocolFactory) {
	networkProtocols[name] = p
}

// RegisterTransportProtocolFactory registers a new transport protocol factory
// with the stack so that it becomes available to uses of the stack. This function
// is intended to be called by init() functions of the protocols
func RegisterTransportProtocolFactory(name string, p types.TransportProtocolFactory) {
	transportProtocols[name] = p
}

// RegisterLinkEndpoint register a link layer protocol endpoint and returns an
// ID that can be used to refer to it.
func RegisterLinkEndpoint(linkEp types.LinkEndpoint) types.LinkEndpointID {
	linkEpMux.Lock()
	defer linkEpMux.Unlock()

	id := nextLinkEndpointID
	nextLinkEndpointID++

	linkEndpoints[id] = linkEp

	return id
}

// FindLinkEndpoint finds the link endpoint associated with the given id
func FindLinkEndpoint(id types.LinkEndpointID) types.LinkEndpoint {
	linkEpMux.RLock()
	defer linkEpMux.RUnlock()

	return linkEndpoints[id]
}
