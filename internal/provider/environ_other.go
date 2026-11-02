//go:build !windows

package provider

import (
	"hash/maphash"
	"os"
)

var environSeed = maphash.MakeSeed()

func environmentSummary() (uint64, bool) {
	var h maphash.Hash
	h.SetSeed(environSeed)
	for _, kv := range os.Environ() {
		h.WriteString(kv)
		h.WriteByte(0)
	}
	return h.Sum64(), true
}
