package game

import "math/rand"

// NewShuffledTiles builds a full 136-tile set with unique IDs and shuffles it.
func NewShuffledTiles(rng *rand.Rand) []Tile {
	tiles := make([]Tile, 0, 136)
	id := 0
	for _, k := range AllKinds {
		for c := 0; c < 4; c++ {
			tiles = append(tiles, Tile{ID: id, Kind: k})
			id++
		}
	}
	rng.Shuffle(len(tiles), func(i, j int) {
		tiles[i], tiles[j] = tiles[j], tiles[i]
	})
	return tiles
}

// DealSplit splits 136 shuffled tiles into two private walls (34 each) and a side wall (68).
func DealSplit(tiles []Tile) (wall0, wall1, side []Tile) {
	if len(tiles) != 136 {
		panic("deal requires 136 tiles")
	}
	wall0 = append([]Tile(nil), tiles[0:34]...)
	wall1 = append([]Tile(nil), tiles[34:68]...)
	side = append([]Tile(nil), tiles[68:136]...)
	return wall0, wall1, side
}

// PairSideWall groups 68 side-wall tiles into 34 stacks of [top, bottom] for omote/ura.
func PairSideWall(side []Tile) [][2]Tile {
	if len(side) != 68 {
		panic("side wall requires 68 tiles")
	}
	out := make([][2]Tile, 0, 34)
	for i := 0; i < 68; i += 2 {
		out = append(out, [2]Tile{side[i], side[i+1]})
	}
	return out
}
// PickTilesByID extracts tiles by unique IDs from wall, preserving selection order.
// Returns ErrInvalidPayload-equivalent via ok=false on duplicates or missing ids.
func PickTilesByID(wall []Tile, ids []int) (picked, remain []Tile, ok bool) {
	seen := map[int]bool{}
	picked = make([]Tile, 0, len(ids))
	remain = append([]Tile(nil), wall...)
	for _, id := range ids {
		if seen[id] {
			return nil, wall, false
		}
		seen[id] = true
		var t Tile
		var found bool
		remain, t, found = RemoveTileID(remain, id)
		if !found {
			return nil, wall, false
		}
		picked = append(picked, t)
	}
	return picked, remain, true
}

// RemoveTileID removes one tile by ID from a slice. Returns ok=false if missing.
func RemoveTileID(tiles []Tile, id int) ([]Tile, Tile, bool) {
	for i, t := range tiles {
		if t.ID == id {
			out := append(append([]Tile(nil), tiles[:i]...), tiles[i+1:]...)
			return out, t, true
		}
	}
	return tiles, Tile{}, false
}

// ContainsID reports whether id is in tiles.
func ContainsID(tiles []Tile, id int) bool {
	for _, t := range tiles {
		if t.ID == id {
			return true
		}
	}
	return false
}

// KindsOf returns kinds for tiles.
func KindsOf(tiles []Tile) []Kind {
	out := make([]Kind, len(tiles))
	for i, t := range tiles {
		out[i] = t.Kind
	}
	return out
}

// CountKinds returns multiplicity of each kind.
func CountKinds(kinds []Kind) map[Kind]int {
	m := make(map[Kind]int, len(kinds))
	for _, k := range kinds {
		m[k]++
	}
	return m
}
