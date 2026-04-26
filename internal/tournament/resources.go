package tournament

import (
	"sort"
	"strconv"
	"strings"
)

// ResourceUsage summarizes the concurrent CPU and memory footprint a
// tournament demands. Values default to 1 thread and 0 MB hash per
// engine that does not declare those UCI options.
type ResourceUsage struct {
	// Threads is the worst-case sum of every engine thread running at the
	// same time (white + black across the most expensive Concurrency
	// pairings).
	Threads int

	// HashMB is the worst-case sum of UCI Hash sizes (in megabytes) held
	// resident across the most expensive Concurrency pairings.
	HashMB int
}

// EstimateUsage returns the worst-case resource footprint for running
// pairings with the given concurrency. It picks the C pairings with the
// highest combined per-pairing usage (threads, then hash as a tiebreak)
// and sums them, since at any instant up to Concurrency games run side
// by side. Defaults: missing Threads = 1, missing Hash = 0.
func EstimateUsage(pairings []Pairing, concurrency int) ResourceUsage {
	if len(pairings) == 0 {
		return ResourceUsage{}
	}
	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > len(pairings) {
		concurrency = len(pairings)
	}

	type cost struct {
		threads int
		hashMB  int
	}
	costs := make([]cost, len(pairings))
	for i, p := range pairings {
		wt, wh := specCost(p.White)
		bt, bh := specCost(p.Black)
		costs[i] = cost{threads: wt + bt, hashMB: wh + bh}
	}

	sort.Slice(costs, func(i, j int) bool {
		if costs[i].threads != costs[j].threads {
			return costs[i].threads > costs[j].threads
		}
		return costs[i].hashMB > costs[j].hashMB
	})

	var u ResourceUsage
	for i := range concurrency {
		u.Threads += costs[i].threads
		u.HashMB += costs[i].hashMB
	}
	return u
}

// specCost returns one engine's declared (threads, hashMB) from its UCI
// option overrides. Missing or unparseable values default to 1 thread
// and 0 MB.
func specCost(spec EngineSpec) (threads, hashMB int) {
	threads = 1
	if v, ok := lookupOption(spec.Options, "Threads"); ok {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			threads = n
		}
	}
	if v, ok := lookupOption(spec.Options, "Hash"); ok {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			hashMB = n
		}
	}
	return
}

// lookupOption is case-insensitive: UCI option names are
// case-insensitive in the protocol, and engines/users disagree on
// casing ("Threads" vs "threads").
func lookupOption(opts map[string]string, name string) (string, bool) {
	if v, ok := opts[name]; ok {
		return v, true
	}
	for k, v := range opts {
		if strings.EqualFold(k, name) {
			return v, true
		}
	}
	return "", false
}
