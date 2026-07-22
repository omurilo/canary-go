package combat

import "math"

// This file is a faithful port of the AreaCombat / MatrixArea machinery in
// src/creatures/combat/combat.cpp. A combat area is authored in Lua as a matrix
// of 0/1/3 values (3 = the center/caster reference tile). The matrix is rotated
// into the four cardinal (and optionally diagonal) orientations at setup time;
// at cast time getList() projects the orientation that matches the cast
// direction onto absolute map positions around the target.
//
// The map-dependent sight-line and floor-change filtering done by C++
// AreaCombat::getList (src/creatures/combat/combat.cpp:2142) is intentionally
// omitted here: this package has no map access. The game layer that consumes
// GetList is responsible for discarding positions that are off-map/unreachable.

// Matrix orientation operations, mirroring MatrixOperation_t in combat.cpp.
type matrixOperation int

const (
	matrixOpCopy matrixOperation = iota
	matrixOpMirror
	matrixOpFlip
	matrixOpRotate90
	matrixOpRotate180
	matrixOpRotate270
)

// Direction indices match the C++ Direction enum (creatures_definitions.hpp) and
// the game.Direction values: N,E,S,W,SW,SE,NW,NE.
const (
	dirNorth = 0
	dirEast  = 1
	dirSouth = 2
	dirWest  = 3
	dirSW    = 4
	dirSE    = 5
	dirNW    = 6
	dirNE    = 7
	numDirs  = 8
)

// MatrixArea mirrors the C++ MatrixArea (combat.cpp:2652): a boolean grid with a
// designated center cell.
type MatrixArea struct {
	rows, cols       uint32
	centerX, centerY uint32
	data             [][]bool
}

func newMatrixArea(rows, cols uint32) *MatrixArea {
	data := make([][]bool, rows)
	for i := range data {
		data[i] = make([]bool, cols)
	}
	return &MatrixArea{rows: rows, cols: cols, data: data}
}

func (m *MatrixArea) setValue(row, col uint32, v bool) {
	if row < m.rows && col < m.cols {
		m.data[row][col] = v
	}
}

func (m *MatrixArea) getValue(row, col uint32) bool {
	if row < m.rows && col < m.cols {
		return m.data[row][col]
	}
	return false
}

func (m *MatrixArea) setCenter(y, x uint32) { m.centerY, m.centerX = y, x }

// AreaCombat holds the per-direction matrices. Mirrors AreaCombat in combat.hpp.
type AreaCombat struct {
	areas      [numDirs]*MatrixArea
	hasExtArea bool
}

// NewAreaCombat builds an AreaCombat from a row-major matrix of 0/1/3 values and
// the number of rows, mirroring the Lua createCombatArea(area[, extArea]).
func NewAreaCombat(list []uint32, rows uint32) *AreaCombat {
	a := &AreaCombat{}
	a.setupArea(list, rows)
	return a
}

// SetupExtArea adds the diagonal orientations (createCombatArea's 2nd argument).
func (a *AreaCombat) SetupExtArea(list []uint32, rows uint32) {
	if len(list) == 0 {
		return
	}
	a.hasExtArea = true
	nw := createArea(list, rows)
	maxOutput := max32(nw.cols, nw.rows) * 2
	ne := newMatrixArea(maxOutput, maxOutput)
	copyArea(nw, ne, matrixOpMirror)
	sw := newMatrixArea(maxOutput, maxOutput)
	copyArea(nw, sw, matrixOpFlip)
	se := newMatrixArea(maxOutput, maxOutput)
	copyArea(sw, se, matrixOpMirror)
	a.areas[dirNW] = nw
	a.areas[dirSW] = sw
	a.areas[dirNE] = ne
	a.areas[dirSE] = se
}

// createArea mirrors AreaCombat::createArea (combat.cpp:2291): value 1 or 3 sets
// the cell, value 2 or 3 marks the center.
func createArea(list []uint32, rows uint32) *MatrixArea {
	var cols uint32
	if rows != 0 {
		cols = uint32(len(list)) / rows
	}
	area := newMatrixArea(rows, cols)
	var x, y uint32
	for _, value := range list {
		if value == 1 || value == 3 {
			area.setValue(y, x, true)
		}
		if value == 2 || value == 3 {
			area.setCenter(y, x)
		}
		x++
		if cols == x {
			x = 0
			y++
		}
	}
	return area
}

// copyArea mirrors AreaCombat::copyArea (combat.cpp:2180).
func copyArea(input, output *MatrixArea, op matrixOperation) {
	centerY, centerX := input.centerY, input.centerX
	switch op {
	case matrixOpCopy:
		for y := uint32(0); y < input.rows; y++ {
			for x := uint32(0); x < input.cols; x++ {
				output.setValue(y, x, input.getValue(y, x))
			}
		}
		output.setCenter(centerY, centerX)
	case matrixOpMirror:
		for y := uint32(0); y < input.rows; y++ {
			var rx uint32
			for x := int(input.cols) - 1; x >= 0; x-- {
				output.setValue(y, rx, input.getValue(y, uint32(x)))
				rx++
			}
		}
		output.setCenter(centerY, (input.rows-1)-centerX)
	case matrixOpFlip:
		for x := uint32(0); x < input.cols; x++ {
			var ry uint32
			for y := int(input.rows) - 1; y >= 0; y-- {
				output.setValue(ry, x, input.getValue(uint32(y), x))
				ry++
			}
		}
		output.setCenter((input.cols-1)-centerY, centerX)
	default: // rotations
		rotateCenterX := int(output.cols/2) - 1
		rotateCenterY := int(output.rows/2) - 1
		var angle int
		switch op {
		case matrixOpRotate90:
			angle = 90
		case matrixOpRotate180:
			angle = 180
		case matrixOpRotate270:
			angle = 270
		}
		angleRad := math.Pi * float64(angle) / 180.0
		a := math.Cos(angleRad)
		b := -math.Sin(angleRad)
		c := math.Sin(angleRad)
		d := math.Cos(angleRad)
		for x := uint32(0); x < input.cols; x++ {
			for y := uint32(0); y < input.rows; y++ {
				newX := int(x) - int(centerX)
				newY := int(y) - int(centerY)
				rotatedX := int(math.Round(float64(newX)*a + float64(newY)*b))
				rotatedY := int(math.Round(float64(newX)*c + float64(newY)*d))
				oy := rotatedY + rotateCenterY
				ox := rotatedX + rotateCenterX
				if oy >= 0 && ox >= 0 {
					output.setValue(uint32(oy), uint32(ox), input.getValue(y, x))
				}
			}
		}
		output.setCenter(uint32(rotateCenterY), uint32(rotateCenterX))
	}
}

// setupArea mirrors AreaCombat::setupArea(list, rows) (combat.cpp:2323).
func (a *AreaCombat) setupArea(list []uint32, rows uint32) {
	north := createArea(list, rows)
	maxOutput := max32(north.cols, north.rows) * 2
	south := newMatrixArea(maxOutput, maxOutput)
	copyArea(north, south, matrixOpRotate180)
	east := newMatrixArea(maxOutput, maxOutput)
	copyArea(north, east, matrixOpRotate90)
	west := newMatrixArea(maxOutput, maxOutput)
	copyArea(north, west, matrixOpRotate270)
	a.areas[dirNorth] = north
	a.areas[dirSouth] = south
	a.areas[dirEast] = east
	a.areas[dirWest] = west
}

// getArea picks the matrix for the direction from center to target, mirroring
// AreaCombat::getArea (combat.cpp:2261).
func (a *AreaCombat) getArea(center, target Position) *MatrixArea {
	dx := int(target.X) - int(center.X)
	dy := int(target.Y) - int(center.Y)
	dir := dirSouth
	switch {
	case dx < 0:
		dir = dirWest
	case dx > 0:
		dir = dirEast
	case dy < 0:
		dir = dirNorth
	default:
		dir = dirSouth
	}
	if a.hasExtArea {
		switch {
		case dx < 0 && dy < 0:
			dir = dirNW
		case dx > 0 && dy < 0:
			dir = dirNE
		case dx < 0 && dy > 0:
			dir = dirSW
		case dx > 0 && dy > 0:
			dir = dirSE
		}
	}
	return a.areas[dir]
}

// GetList projects the oriented matrix onto absolute map positions around target,
// mirroring AreaCombat::getList (combat.cpp:2142) minus the sight/floor filters.
func (a *AreaCombat) GetList(center, target Position) []Position {
	area := a.getArea(center, target)
	if area == nil {
		return nil
	}
	rows, cols := area.rows, area.cols
	out := make([]Position, 0, rows*cols)
	baseX := int(target.X) - int(area.centerX)
	baseY := int(target.Y) - int(area.centerY)
	for y := uint32(0); y < rows; y++ {
		for x := uint32(0); x < cols; x++ {
			if area.getValue(y, x) {
				px := baseX + int(x)
				py := baseY + int(y)
				if px >= 0 && py >= 0 {
					out = append(out, Position{X: uint16(px), Y: uint16(py), Z: target.Z})
				}
			}
		}
	}
	return out
}

func max32(a, b uint32) uint32 {
	if a > b {
		return a
	}
	return b
}
