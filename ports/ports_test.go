package ports

import (
	"testing"

	"github.com/YaoZengzeng/yustack/types"
)

func TestPickEphemeralPort(t *testing.T) {
	pm := NewPortManager()
	customErr := &types.Error{}
	for _, test := range []struct {
		name 		string
		f 			func(port uint16) (bool, error)
		wantErr		error
		wantPort	uint16
	}{
		{
			name: "no-port-available",
			f: func(port uint16) (bool, error) {
				return false, nil
			},
			wantErr: types.ErrNoPortAvailable,
		},
		{
			name: "port-tester-error",
			f: func(port uint16) (bool, error) {
				return false, customErr
			},
			wantErr: customErr,
		},
		{
			name: "only-port-16042-available",
			f: func(port uint16) (bool, error) {
				if port == firstEphemeral + 42 {
					return true, nil
				}
				return false, nil
			},
			wantPort: firstEphemeral + 42,
		},
		{
			name: "only-port-under-16000-available",
			f:	func(port uint16) (bool, error) {
				if port < firstEphemeral {
					return true, nil
				}
				return false, nil
			},
			wantErr: types.ErrNoPortAvailable,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if port, err := pm.PickEphemeralPort(test.f); port != test.wantPort || err != test.wantErr {
				t.Errorf("PickEphemeralPort(...) = (port %d, err %v); want (port %d, err %v)", port, err, test.wantPort, test.wantErr)
			}
		})
	}

}