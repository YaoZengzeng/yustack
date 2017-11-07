package types

// Route is a row in the routing table. It specifies through which Nic (and
// gateway) sets of packets should be routed. A row is considered viable if the
// masked target address matches the destination address in the row
type Route struct {
	// Destination is the address that must be matched against the masked
	// target address to check if this row is viable
	Destination 	Address

	// Mask specifies which bits of the Destination and the target address
	// must match for this row to be viable
	Mask 			Address

	// Gateway is the gateway to be used if this row is viable
	Gateway 		Address

	// Nic is the id of the nic to be used if this row is viable
	Nic 			NicId
}
