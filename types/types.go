package types

import (
	"fmt"

	"github.com/YaoZengzeng/yustack/buffer"
	"github.com/YaoZengzeng/yustack/waiter"
)

// Address is a byte slice cast as a string that represents the address of a
// network node. Or, when we support the case of unix endpoints, it may represent a path.
type Address string

// String implements the fmt.Stringer interface
func (a Address) String() string {
	switch len(a) {
	case 4:
		return fmt.Sprintf("%d.%d.%d.%d", int(a[0]), int(a[1]), int(a[2]), int(a[3]))
	default:
		return fmt.Sprintf("%x", []byte(a))
	}
}

// NicId is a number that uniquely identifies a Nic
type NicId int32

// NetworkDispatcher contains the methods used by the network stack to deliver
// packets to the appropriate network endpoint after it has been handled by the
// data link layer
type NetworkDispatcher interface {
	// DeliverNetworkPacket finds the appropriate network protocol
	// endpoint and hands the packet for further processing
	DeliverNetworkPacket(linkEp LinkEndpoint, remoteLinkAddr LinkAddress, protocol NetworkProtocolNumber, vv *buffer.VectorisedView)
}

// TransportDispatcher contains the methods used by the network stack to deliver
// packets to the appropriate transport endpoint after it has been handled by the
// network layer
type TransportDispatcher interface {
	// DeliverTransportPacket delivers the packets to the appropriate
	// transport protocol endpoint
	DeliverTransportPacket(r *Route, protocol TransportProtocolNumber, vv *buffer.VectorisedView)
}

// ErrorOption is used in GetSockOpt to specify that the last error reported by
// the endpoint should be cleared and returned
type ErrorOption struct{}

// Endpoint is the interface implemented by transport protocols (e.g., tcp, udp)
// that exposes functionality link read, write, connect, etc to uses of the networking
// stack
type Endpoint interface {
	// Bind binds the endpoint to a specific local address and port
	// Specifying a Nic is optional
	Bind(address FullAddress) error

	// Read reads data from the endpoint and optionally returns the sender
	// This method does not block if there is no data pending
	// It will also either return an error or data, never both
	Read(*FullAddress) (buffer.View, error)

	// Write writes data to the endpoint's peer, or the provided address if
	// one is specified. This method does not block if the data cannot be written
	//
	// Note that unlike io.Write.Write, it is not an error for Write to perform a
	// partial write
	Write(buffer.View, *FullAddress) (uintptr, error)

	// Listen puts the endpoint in "listen" mode, which allows it to connect
	// newn connections
	Listen(backlog int) error

	// Accept returns a new endpoint if a peer has established a connection
	// to and endpoint previously set to listen mode. This method does not
	// block if no new connections are available.
	//
	// The returned Queue is the wait queue for the newly created endpoint
	Accept() (Endpoint, *waiter.Queue, error)

	// Connect connects the endpoint to its peer. Specifying a Nic is
	// optional.
	//
	// There are three classes of return values:
	//	nil -- the attemp to connect succeeded
	//	ErrConnectStarted -- the connect attempt started but hasn't
	//		completed yet. In this case, the actual result will
	//		become available via GetSockOpt(ErrorOption) when
	//		the endpoint becomes writable. (This mimics the connect(2)
	//		syscall behaviour.)
	//	Anything else -- the attemp to connect failed
	Connect(address FullAddress) error

	// Close puts the endpoint in a closed state and frees all resources
	// associated with it
	Close()

	// Shutdown closes the read and/or write end of the endpoint connection
	// to its peer
	Shutdown(flags ShutdownFlags) error

	// GetSockOpt gets a socket option. opt should be a pointer to one of the
	// *Option types
	GetSockOpt(opt interface{}) error
}

// FullAddress represents a full transport node address, as required by the
// Connect() and Bind() methods
type FullAddress struct {
	// Nic is the Id of the Nic this address refers to
	// This may not be used by all endpoint types
	Nic NicId

	// Address is the network address
	Address Address

	// Port is the transport port
	//
	// This may not be used by all endpoint types
	Port uint16
}

// ShutdownFlags represents flags that can be passed to the Shutdown() method
// of the Endpoint interface
type ShutdownFlags int

// Values of the flags that can be passed to the Shutdown() method. They can
// be OR'ed together
const (
	ShutdownRead	ShutdownFlags = 1 << iota
	ShutdownWrite
)
