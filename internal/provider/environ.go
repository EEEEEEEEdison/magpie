package provider

// environSum is a hash of this process's environment: the folders every
// file and account is read from are named by it. Zero means the environment
// couldn't be read, so it is unknown rather than an empty environment.
func environSum() uint64 {
	sum, ok := environmentSummary()
	if !ok {
		return 0
	}
	// Keep zero for an unknown environment. Remapping this one hash value
	// has the same collision probability as any other value of the hash.
	if sum == 0 {
		return 1
	}
	return sum
}

// sameEnvironment compares only environments that were read successfully.
// Two failed reads are both unknown, not evidence that nothing changed.
func sameEnvironment(a, b uint64) bool {
	return a != 0 && a == b
}
