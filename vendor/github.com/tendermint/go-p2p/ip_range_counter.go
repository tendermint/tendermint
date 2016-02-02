package p2p

import (
	"strings"
)

// TODO Test
func AddToIPRangeCounts(counts map[string]int, ip string) map[string]int {
	changes := make(map[string]int)
	ipParts := strings.Split(ip, ":")
	for i := 1; i < len(ipParts); i++ {
		prefix := strings.Join(ipParts[:i], ":")
		counts[prefix] += 1
		changes[prefix] = counts[prefix]
	}
	return changes
}

// TODO Test
func CheckIPRangeCounts(counts map[string]int, limits []int) bool {
	for prefix, count := range counts {
		ipParts := strings.Split(prefix, ":")
		numParts := len(ipParts)
		if limits[numParts] < count {
			return false
		}
	}
	return true
}
