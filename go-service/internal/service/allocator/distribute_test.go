package allocator

import (
	"math/rand"
	"reflect"
	"testing"
)

func sumInts(v []int) int {
	s := 0
	for _, x := range v {
		s += x
	}
	return s
}

func TestDistributeKnownCases(t *testing.T) {
	tests := []struct {
		name    string
		total   int
		weights []float64
		want    []int
	}{
		{"equal weights, remainder to the first", 10, []float64{1, 1, 1}, []int{4, 3, 3}},
		{"exact split", 12, []float64{1, 1, 2}, []int{3, 3, 6}},
		{"largest fraction wins", 10, []float64{0.45, 0.35, 0.2}, []int{5, 3, 2}},
		{"single element", 7, []float64{0.3}, []int{7}},
		{"zero total", 0, []float64{1, 2}, []int{0, 0}},
		{"weights need not add to 1", 6, []float64{5, 5}, []int{3, 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := distribute(tt.total, tt.weights); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("distribute(%d, %v) = %v, want %v", tt.total, tt.weights, got, tt.want)
			}
		})
	}
}

func TestDistributeAlwaysSumsToTotal(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 20000; i++ {
		total := rng.Intn(1001) // up to 500 h in half-hours
		n := 1 + rng.Intn(12)
		weights := make([]float64, n)
		for j := range weights {
			weights[j] = 0.001 + rng.Float64()
		}

		got := distribute(total, weights)

		if sumInts(got) != total {
			t.Fatalf("sum = %d, want %d (weights %v, got %v)", sumInts(got), total, weights, got)
		}
		for _, v := range got {
			if v < 0 {
				t.Fatalf("negative share in %v", got)
			}
		}
	}
}
