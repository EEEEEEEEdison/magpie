package provider

import (
	"hash/maphash"
	"syscall"
	"unsafe"
)

var environSeed = maphash.MakeSeed()

// Hash the environment block itself instead of os.Environ's slice and one
// converted string per variable. A list tag is asked for on every models
// request; those conversions were most of its unchanged-stamp allocations.
var (
	environmentStrings     = syscall.GetEnvironmentStrings
	freeEnvironmentStrings = syscall.FreeEnvironmentStrings
)

func environmentSummary() (sum uint64, ok bool) {
	envp, err := environmentStrings()
	if err != nil || envp == nil {
		return 0, false
	}
	defer func() {
		if err := freeEnvironmentStrings(envp); err != nil {
			sum, ok = 0, false
		}
	}()

	// GetEnvironmentStrings returns a double-NUL-terminated UTF-16 block.
	// Hash its exact bytes, including the terminator: no intermediate string
	// or slice of entries is needed, and every Setenv/Unsetenv is visible.
	end := unsafe.Pointer(envp)
	for *(*uint16)(end) != 0 || *(*uint16)(unsafe.Add(end, 2)) != 0 {
		end = unsafe.Add(end, 2)
	}
	n := uintptr(end) - uintptr(unsafe.Pointer(envp)) + 2
	var h maphash.Hash
	h.SetSeed(environSeed)
	h.Write(unsafe.Slice((*byte)(unsafe.Pointer(envp)), n))
	return h.Sum64(), true
}
