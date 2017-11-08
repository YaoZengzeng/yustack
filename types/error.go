package types

// Error represents an error in the yustack error space. Using a special type
// ensures that errors outside of this space are not accidentally introduced
type Error struct {
	string
}

// String implements fmt.Stringer.String
func (e *Error) Error() string {
	return e.string
}

// Errors that can be returned by the network stack
var (
	ErrUnknownProtocol       = &Error{"unknown protocol"}
	ErrUnknownNicId          = &Error{"unknown nic id"}
	ErrUnknownProtocolOption = &Error{"unknown option for protocol"}
	ErrDuplicateNicId        = &Error{"duplicate nic id"}
	ErrDuplicateAddress      = &Error{"duplicate address"}
	ErrNoRoute               = &Error{"no route"}
	ErrBadLinkEndpoint       = &Error{"bad link layer endpoint"}
	ErrAlreadyBound          = &Error{"endpoint already bound"}
	ErrInvalidEndpointState  = &Error{"endpoint is in invalid state"}
	ErrAlreadyConnecting     = &Error{"endpoint is already connecting"}
	ErrAlreadyConnected      = &Error{"endpoint is already connected"}
	ErrNoPortAvailable       = &Error{"no ports are available"}
	ErrPortInUse             = &Error{"port is in use"}
	ErrBadLocalAddress       = &Error{"bad local address"}
	ErrClosedForSend         = &Error{"endpoint is closed for send"}
	ErrClosedForReceive      = &Error{"endpoint is closed for receive"}
	ErrWouldBlock            = &Error{"operation would block"}
	ErrConnectionRefused     = &Error{"connection was refused"}
	ErrTimeout               = &Error{"operation timed out"}
	ErrAborted               = &Error{"operation aborted"}
	ErrConnectStarted        = &Error{"connection attempt started"}
	ErrDestinationRequired   = &Error{"destination address is required"}
	ErrNotSupported          = &Error{"operation not supported"}
	ErrQueueSizeNotSupported = &Error{"queue size querying not supported"}
	ErrNotConnected          = &Error{"endpoint not connected"}
	ErrConnectionReset       = &Error{"connection reset by peer"}
	ErrConnectionAborted     = &Error{"connection aborted"}
	ErrNoSuchFile            = &Error{"no such file"}
	ErrInvalidOptionValue    = &Error{"invalid option value specified"}
)
