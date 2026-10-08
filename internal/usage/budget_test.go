package usage

import (
	"math"
	"testing"
)

func TestCost(t *testing.T) {
	if got := Cost(1000, 500, 0.0025, 0.01); math.Abs(got-0.0075) > 1e-12 {
		t.Fatalf("cost %v", got)
	}
	if Cost(0, 0, 1, 2) != 0 {
		t.Fatal("zero usage charged")
	}
}
