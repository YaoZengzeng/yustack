package types

// Address is a byte slice cast as a string that represents the address of a
// network node. Or, when we support the case of unix endpoints, it may represent a path.
type Address string

// NicId is a number that uniquely identifies a Nic
type NicId int32

// NetworkDispatcher contains the methods used by the network stack to deliver
// packets to the appropriate network endpoint after it has been handled by the
// data link layer
type NetworkDispatcher interface {
	// DeliverNetworkPacket finds the appropriate network protocol
	// endpoint and hands the packet for further processing
	DeliverNetworkPacket(linkEp LinkEndpoint, remoteLinkAddr LinkAddress, protocol NetworkProtocolNumber)
}
