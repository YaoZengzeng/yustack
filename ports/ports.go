// Package ports provides PortManager that manages allocating, reserving and releasing ports

package ports

import (
	"log"
	"math"
	"math/rand"

	"github.com/YaoZengzeng/yustack/types"
)

const (
	// firstEphemeral is the first ephemeral port
	firstEphemeral uint16 = 16000

	anyIPAddress = types.Address("")
)

// PortManager manages allocating, reserving and releasing ports
type PortManager struct {

}

// NewPortManager creates new PortManager
func NewPortManager() *PortManager {
	return &PortManager{

	}
}

// PickEphemeralPort randomly chooses a starting point and iterates over all
// possible ephemeral ports, allowing the caller to decided whether a given port
// is suitable for its needs, and stopping when a port is found or an error occurs
func (s *PortManager) PickEphemeralPort(testPort func(p uint16) (bool, error)) (port uint16, err error) {
	count := uint16(math.MaxUint16 - firstEphemeral + 1)
	offset := uint16(rand.Int31n(int32(count)))

	for i := uint16(0); i < count; i++ {
		port = firstEphemeral + (offset + i) % count
		ok, err := testPort(port)
		if err != nil {
			log.Printf("PickEphemeralPort: testPort failed %v\n", err)
			return 0, err
		}

		if ok {
			return port, nil
		}

		// The port has been used, try next one
	}

	return 0, types.ErrNoPortAvailable
}