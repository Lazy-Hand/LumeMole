package engine

// Treemap implements the squarified treemap layout (Bruls, Huizing, van Wijk
// 2000): children are placed to keep each rectangle as close to square as
// possible, which makes sizes easy to compare.

// TMapRect is one laid-out cell, in the same 0..1 normalized space as the input.
type TMapRect struct {
	Node          *TreeNode
	X, Y, W, H    float64 // 0..1 fractions of the given area
	Row           int     // visual row for coloring variation
	Index         int     // index into the sorted children
}

// Layout positions nodes inside the unit square (0,0,1,1).
// Input nodes must have Size set; they are sorted desc internally.
func Layout(nodes []*TreeNode) []TMapRect {
	if len(nodes) == 0 {
		return nil
	}
	total := int64(0)
	for _, n := range nodes {
		total += n.Size
	}
	if total <= 0 {
		return nil
	}
	// Copy & sort desc.
	sorted := make([]*TreeNode, len(nodes))
	copy(sorted, nodes)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Size > sorted[i].Size {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	var rects []TMapRect
	squarify(sorted, 0, 0, 1, 1, total, &rects)
	return rects
}

// squarify is the classic algorithm. x,y,w,h is the current free area in
// unit space; sizes are already proportions of total.
func squarify(nodes []*TreeNode, x, y, w, h float64, total int64, out *[]TMapRect) {
	if len(nodes) == 0 || w <= 0 || h <= 0 {
		return
	}
	// Accumulate a row that stays squarish, then lay it along the shorter side.
	i := 0
	for i < len(nodes) {
		// Try to extend the current row until aspect worsens.
		row := []*TreeNode{nodes[i]}
		rowSum := nodes[i].Size
		j := i + 1
		for j < len(nodes) {
			if !improvesAspect(row, rowSum, nodes[j], w, h, total) {
				break
			}
			row = append(row, nodes[j])
			rowSum += nodes[j].Size
			j++
		}
		// Lay the row.
		frac := float64(rowSum) / float64(total)
		if w >= h {
			// row along left side, full height.
			rw := w * frac
			yy := y
			for _, n := range row {
				nf := float64(n.Size) / float64(rowSum)
				nh := h * nf
				*out = append(*out, TMapRect{Node: n, X: x, Y: yy, W: rw, H: nh, Index: len(*out)})
				yy += nh
			}
			x += rw
			w -= rw
		} else {
			// row along top, full width.
			rh := h * frac
			xx := x
			for _, n := range row {
				nf := float64(n.Size) / float64(rowSum)
				nw := w * nf
				*out = append(*out, TMapRect{Node: n, X: xx, Y: y, W: nw, H: rh, Index: len(*out)})
				xx += nw
			}
			y += rh
			h -= rh
		}
		i = j
	}
}

// improvesAspect reports whether adding n to the row improves the worst
// aspect ratio (closer to 1 is better).
func improvesAspect(row []*TreeNode, rowSum int64, n *TreeNode, w, h float64, total int64) bool {
	if len(row) == 0 {
		return true
	}
	side := w
	if h < w {
		side = h
	}
	rowArea := float64(rowSum) / float64(total) * w * h
	newSum := rowSum + n.Size
	newArea := float64(newSum) / float64(total) * w * h
	worstBefore := worstAspect(row, rowSum, rowArea, side)
	row2 := append(append([]*TreeNode{}, row...), n)
	worstAfter := worstAspect(row2, newSum, newArea, side)
	return worstAfter <= worstBefore
}

// worstAspect computes the worst (furthest from 1) aspect ratio in a row,
// given the row's total area and the length of the side it lies on.
func worstAspect(row []*TreeNode, sum int64, area, side float64) float64 {
	if len(row) == 0 || sum == 0 {
		return 0
	}
	min := float64(row[0].Size)
	max := min
	for _, n := range row {
		f := float64(n.Size) / float64(sum) * area
		if f < min {
			min = f
		}
		if f > max {
			max = f
		}
	}
	s2 := side * side
	a2 := area * area
	if a2 == 0 {
		return 1
	}
	if s2*max > a2/min {
		return s2 * max / a2
	}
	return a2 / (s2 * min)
}
