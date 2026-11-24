package translate

import "testing"

func TestPanelPosition(t *testing.T) {
	area := Rect{X: 0, Y: 0, Width: 1000, Height: 800}
	tests := []struct {
		name    string
		anchor  Point
		follows bool
		area    Rect
		wantX   int
		wantY   int
	}{
		{
			name:    "right and below cursor",
			anchor:  Point{X: 100, Y: 100},
			follows: true,
			area:    area,
			wantX:   116,
			wantY:   120,
		},
		{
			name:    "flips at right bottom corner",
			anchor:  Point{X: 950, Y: 700},
			follows: true,
			area:    area,
			wantX:   534,
			wantY:   360,
		},
		{
			name:    "clamps to top left margin",
			anchor:  Point{X: 100, Y: 50},
			follows: true,
			area:    Rect{X: 100, Y: 50, Width: 500, Height: 400},
			wantX:   116,
			wantY:   70,
		},
		{
			name:    "centers when not following",
			anchor:  Point{X: 10, Y: 10},
			follows: false,
			area:    area,
			wantX:   300,
			wantY:   240,
		},
		{
			name:    "falls back to anchor without area",
			anchor:  Point{X: 42, Y: 24},
			follows: true,
			area:    Rect{},
			wantX:   42,
			wantY:   24,
		},
		{
			name:    "keeps margin when area is smaller than panel",
			anchor:  Point{X: 200, Y: 200},
			follows: true,
			area:    Rect{X: 100, Y: 100, Width: 300, Height: 200},
			wantX:   108,
			wantY:   108,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x, y := panelPosition(tt.anchor, 400, 320, tt.area, tt.follows)
			if x != tt.wantX || y != tt.wantY {
				t.Fatalf("panelPosition() = (%d, %d), want (%d, %d)", x, y, tt.wantX, tt.wantY)
			}
		})
	}
}

func TestPanelHeightFor(t *testing.T) {
	if got := panelHeightFor(1, 0); got != panelHeightDefault {
		t.Fatalf("default height = %d, want %d", got, panelHeightDefault)
	}
	if got := panelHeightFor(1, 1000); got != panelHeightDefault {
		t.Fatalf("roomy height = %d, want %d", got, panelHeightDefault)
	}
	if got := panelHeightFor(2, 0); got != panelMaxHeight {
		t.Fatalf("scaled height = %d, want %d", got, panelMaxHeight)
	}
	if got := panelHeightFor(1, 500); got != 400 {
		t.Fatalf("limited height = %d, want 400", got)
	}
	if got := panelHeightFor(1, 300); got != panelMinHeight {
		t.Fatalf("tiny area height = %d, want %d", got, panelMinHeight)
	}
}
