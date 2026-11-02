package provider

import (
	"errors"
	"testing"
)

// A Windows environment read that fails is unknown, including a failure to
// free the block after it was hashed. It must never hash as a usable empty
// environment, or stamps around the failure could compare equal.
func TestEnvironSumReadFailureIsUnknown(t *testing.T) {
	get, free := environmentStrings, freeEnvironmentStrings
	t.Cleanup(func() { environmentStrings, freeEnvironmentStrings = get, free })
	failure := errors.New("environment read failed")

	for _, c := range []struct {
		name string
		get  func() (*uint16, error)
		free func(*uint16) error
	}{
		{"read", func() (*uint16, error) { return nil, failure }, free},
		{"nil block", func() (*uint16, error) { return nil, nil }, free},
		{"free", func() (*uint16, error) {
			block := []uint16{'A', '=', 'B', 0, 0}
			return &block[0], nil
		}, func(*uint16) error { return failure }},
	} {
		t.Run(c.name, func(t *testing.T) {
			environmentStrings, freeEnvironmentStrings = c.get, c.free
			if got := environSum(); got != 0 {
				t.Fatalf("failed read hashed as %x, want unknown", got)
			}
		})
	}
}
