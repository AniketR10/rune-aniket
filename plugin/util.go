package plugin

// MergeResourceMap merges m1 with mn.
// If permissions are overlapping, the last of passed prevails.
func MergeResourceMap(
	m1 map[Permission]ResourceServer, mn ...map[Permission]ResourceServer,
) map[Permission]ResourceServer {
	ret := make(map[Permission]ResourceServer)
	for k, v := range m1 {
		ret[k] = v
	}
	for _, m := range mn {
		for k, v := range m {
			ret[k] = v
		}
	}
	return ret
}
