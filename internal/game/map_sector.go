package game

const SectorSize = 16

// SectorFloor holds the base (shared) tiles and the materialized tiles for a 16x16 chunk of a map floor.
type SectorFloor struct {
	Base  [SectorSize * SectorSize]*Tile // shared read-only tiles
	Tiles [SectorSize * SectorSize]*Tile // materialized mutable tiles
}

// Sector holds up to 16 floors for a 16x16 chunk of the map.
type Sector struct {
	Floors [16]*SectorFloor
}
