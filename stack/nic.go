package stack

import (
	"sync"

	"github.com/YaoZengzeng/yustack/types"
)

// Nic represents a "network interface card" to which the
// networking stack is attached
type Nic struct {
	stack 	*Stack
	id		types.NicId
	linkEp	types.LinkEndpoint

	mu		sync.RWMutex
}

func newNic(stack *Stack, id types.NicId, ep types.LinkEndpoint) *Nic {
	return &Nic{
		stack:		stack,
		id:			id,
		linkEp:		ep,
	}
}

// attachLinkEndpoint attaches the Nic to the endpoint, which will enable it
// to start delivering packets
func (n *Nic) attachLinkEndpoint() {
	// n.linkEp.Attach(n)
}

// AddAddress adds a new address to n, so that it starts to accepting packets
// targeted at the given address (and network protocol)
func (n *Nic) AddAddress(protocol types.NetworkProtocolNumber, address types.Address) error {
	// Add the endpoint
	n.mu.Lock()
	defer n.mu.Unlock()
	// _, err := n.addAddressLocked(protocol, address, false)

	return nil
}
