// Copyright 2017 Tendermint. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
