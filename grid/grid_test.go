package grid

import "testing"

func TestGrid_Up(t *testing.T) {
	g := Grid{
		Percentage:      0.01,
		Fund:            100,
		PricePrecision:  5,
		AmountPrecision: 3,
		MaxGrid:         3,
		Base:            1.0,
	}

	price1, amount1 := g.top()
	t.Logf("up: (%f, %f)", price1, amount1)

	price2, amount2 := g.bottom()
	t.Logf("down: (%f, %f)", price2, amount2)
	if !g.up() {
		t.Error("can't up 1st grid")
	}
	if g.Base != 1.01010 {
		t.Errorf("grid base wrong, base = %f", g.Base)
	}
	if !g.up() {
		t.Error("can't up 2nd grid")
	}
	if !g.up() {
		t.Error("can't up 3rd grid")
	}
	if g.up() {
		t.Error("should not up 4th grid")
	}
	if !g.down() {
		t.Error("can't down 1st grid")
	}
}

func TestGrid_Bottom(t *testing.T) {

}
