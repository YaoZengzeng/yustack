package types

// Address is a byte slice cast as a string that represents the address of a
// network node. Or, when we support the case of unix endpoints, it may represent a path.
type Address string
