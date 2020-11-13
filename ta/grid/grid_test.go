package grid

import (
	"math"
	"testing"
)

func TestComputeGrid(t *testing.T) {
	tests := []struct {
		upper, lower float64
		grids        int
		percent      float64
		min          float64
	}{
		{1, 0.995, 2, 0.005, 0},
		{1, 0.990025, 3, 0.009975, 0},
		{1, 0.985, 4, 0.015, 0},
		//{1, 0.98507488, 4, 0.01492512, 0},
		{100, 98, 5, 0.02, 0},
	}
	for _, tt := range tests {
		grids, percent, _ := ComputeGrid(tt.upper, tt.lower)
		if grids != tt.grids {
			t.Errorf("grids expect: %d, got: %d", tt.grids, grids)
		}
		if math.Abs(percent-tt.percent) >= 1e-9 {
			t.Errorf("percent expect: %f, got: %f", tt.percent, percent)
		}
	}
}
