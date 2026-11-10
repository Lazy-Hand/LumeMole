package engine

import "testing"

func TestLayoutEmpty(t *testing.T) {
	if Layout(nil) != nil {
		t.Fatal("expected nil")
	}
}

func TestLayoutSingle(t *testing.T) {
	nodes := []*TreeNode{{Name: "a", Size: 100}}
	rects := Layout(nodes)
	if len(rects) != 1 {
		t.Fatalf("got %d rects", len(rects))
	}
	r := rects[0]
	if r.W <= 0 || r.H <= 0 {
		t.Fatalf("zero-size rect %+v", r)
	}
}

func TestLayoutSplits(t *testing.T) {
	nodes := []*TreeNode{
		{Name: "big", Size: 800},
		{Name: "mid", Size: 150},
		{Name: "small", Size: 50},
	}
	rects := Layout(nodes)
	if len(rects) != 3 {
		t.Fatalf("got %d rects", len(rects))
	}
	// Areas should be proportional to sizes (within tolerance).
	var total float64
	for _, r := range rects {
		total += r.W * r.H
	}
	if total < 0.9 || total > 1.1 {
		t.Fatalf("areas sum to %v, want ~1.0", total)
	}
	// Biggest should get largest area.
	var bigArea float64
	for _, r := range rects {
		if r.Node.Name == "big" {
			bigArea = r.W * r.H
		}
	}
	if bigArea < 0.7 {
		t.Fatalf("big node area %v too small", bigArea)
	}
}

func TestLayoutZeroSize(t *testing.T) {
	nodes := []*TreeNode{{Name: "a", Size: 0}}
	if Layout(nodes) != nil {
		t.Fatal("expected nil for zero total")
	}
}
