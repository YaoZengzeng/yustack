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

var (
	ErrBadLinkEndpoint		=	&Error{"bad link layer endpoint"}
	ErrDuplicateNicId		=	&Error{"duplicate nic id"}
	ErrUnknownNicId			=	&Error{"unknown nic id"}
)
