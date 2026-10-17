package allocator

import (
	"math"
	"sort"
)

// Distribute is the exported form of the largest remainder split, for callers
// that divide whole units (minutes, points) by weight.
func Distribute(total int, weights []float64) []int { return distribute(total, weights) }

// distribute splits total (in half-hour units) among the weights with the
// largest remainder method: floor every share, then hand the units left over
// to the largest fractional parts. The result always sums to exactly total.
// Ties go to the earlier index, so the output is deterministic.
func distribute(total int, weights []float64) []int {
	out := make([]int, len(weights))
	if total <= 0 || len(weights) == 0 {
		return out
	}

	var sum float64
	for _, w := range weights {
		sum += w
	}

	fractions := make([]float64, len(weights))
	assigned := 0
	for i, w := range weights {
		exact := float64(total) * w / sum
		out[i] = int(math.Floor(exact))
		fractions[i] = exact - float64(out[i])
		assigned += out[i]
	}

	order := make([]int, len(weights))
	for i := range order {
		order[i] = i
	}

	remaining := total - assigned
	if remaining >= 0 {
		sort.SliceStable(order, func(a, b int) bool { return fractions[order[a]] > fractions[order[b]] })
		for i := 0; i < remaining; i++ {
			out[order[i%len(order)]]++
		}
		return out
	}

	// Float error made the floors overshoot: take units back from the smallest fractions.
	sort.SliceStable(order, func(a, b int) bool { return fractions[order[a]] < fractions[order[b]] })
	for i := 0; remaining < 0; i++ {
		if idx := order[i%len(order)]; out[idx] > 0 {
			out[idx]--
			remaining++
		}
	}
	return out
}
