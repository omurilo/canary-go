// Package otbm parses Remere/OTBM binary map files into the game map. Item ids
// in OTBM are client ids (Canary 13.x dropped items.otb), so they index the
// appearance catalog directly. Ground vs stacked classification uses the
// catalog's ground flag, not stream position.
package otbm

import (
	"fmt"
	"os"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/items"
)

// Node type bytes.
const (
	nodeRootV1   = 1
	nodeMapData  = 2
	nodeTileArea = 4
	nodeTile     = 5
	nodeItem     = 6
	nodeTownsGrp = 12
	nodeTown     = 13
	nodeHouseTile = 14
	nodeWaypoints = 15
	nodeWaypoint  = 16
	nodeTileZone  = 19
)

// Attribute tag bytes (map + item level share values).
const (
	attrDescription = 1
	attrExtFile     = 2
	attrTileFlags   = 3
	attrActionID    = 4
	attrUniqueID    = 5
	attrText        = 6
	attrDesc        = 7
	attrTeleDest    = 8
	attrItem        = 9
	attrDepotID     = 10
	attrExtSpawnMon = 11
	attrRuneCharges = 12
	attrExtHouse    = 13
	attrHouseDoorID = 14
	attrCount       = 15
	attrDuration    = 16
	attrDecayState  = 17
	attrWrittenDate = 18
	attrWrittenBy   = 19
	attrSleeperGUID = 20
	attrSleepStart  = 21
	attrCharges     = 22
	attrExtSpawnNPC = 23
	attrName        = 24
	attrArticle     = 25
	attrPluralName  = 26
	attrWeight      = 27
	attrAttack      = 28
	attrDefense     = 29
	attrExtraDefense = 30
	attrArmor       = 31
	attrHitChance   = 32
	attrShootRange  = 33
	attrSpecial     = 34
	attrTier        = 40
)

const escapeByte = 0xFD

// Town is a town/temple entry from the map.
type Town struct {
	ID   uint32
	Name string
	Pos  game.Position
}

// Result summarizes a parsed map.
type Result struct {
	Width, Height uint16
	MajorVersion  uint32
	MinorVersion  uint32
	Description   string
	Towns         []Town
	TileCount     int
	ItemCount     int
}

type reader struct {
	data  []byte
	pos   int
	depth int
}

func (r *reader) eof() bool { return r.pos >= len(r.data) }

// u8 reads one escaped byte.
func (r *reader) u8() byte {
	b := r.data[r.pos]
	r.pos++
	if r.depth > 0 && b == escapeByte {
		b = r.data[r.pos]
		r.pos++
	}
	return b
}

func (r *reader) u16() uint16 { return uint16(r.u8()) | uint16(r.u8())<<8 }
func (r *reader) u32() uint32 {
	return uint32(r.u8()) | uint32(r.u8())<<8 | uint32(r.u8())<<16 | uint32(r.u8())<<24
}
func (r *reader) u64() uint64 {
	var v uint64
	for i := 0; i < 8; i++ {
		v |= uint64(r.u8()) << (8 * i)
	}
	return v
}

// str reads a u16-length string. The length prefix is the logical character
// count; the body is escape-encoded like the rest of a node's properties, so it
// MUST be read char-by-char through u8() (which un-escapes). Reading the body
// raw leaves the cursor short by the number of escape bytes, desyncing every
// subsequent attribute/node in the stream — that corruption is what sent some
// teleports to a (0,0,0) "limbo" destination.
func (r *reader) str() string {
	n := int(r.u16())
	b := make([]byte, 0, n)
	for i := 0; i < n && !r.eof(); i++ {
		b = append(b, r.u8())
	}
	return string(b)
}

// peekRaw returns the next raw byte without consuming it.
func (r *reader) peekRaw() byte { return r.data[r.pos] }

// Load parses an OTBM file into m, classifying items via cat.
func Load(path string, cat *items.Catalog, m *game.Map) (*Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("otbm: read %s: %w", path, err)
	}
	if len(data) < 5 {
		return nil, fmt.Errorf("otbm: file too small")
	}
	r := &reader{data: data, pos: 4} // skip 4-byte identifier

	if r.peekRaw() != 0xFE {
		return nil, fmt.Errorf("otbm: expected root node start 0xFE, got 0x%02X", r.peekRaw())
	}
	r.pos++
	r.depth++
	_ = r.u8() // root node type (0)

	res := &Result{}
	res.MajorVersion = 0
	version := r.u32()
	res.Width = r.u16()
	res.Height = r.u16()
	res.MajorVersion = r.u32()
	res.MinorVersion = r.u32()
	if version > 5 {
		return nil, fmt.Errorf("otbm: unsupported version %d", version)
	}

	p := &parser{r: r, cat: cat, m: m, res: res}
	if err := p.parseRootChildren(); err != nil {
		return nil, err
	}
	return res, nil
}

type parser struct {
	r   *reader
	cat *items.Catalog
	m   *game.Map
	res *Result
}

// parseRootChildren walks the children of the root node (MAP_DATA, etc).
func (p *parser) parseRootChildren() error {
	r := p.r
	for !r.eof() {
		c := r.peekRaw()
		if c == 0xFF { // end of root
			r.pos++
			r.depth--
			return nil
		}
		if c != 0xFE {
			return fmt.Errorf("otbm: unexpected byte 0x%02X at root child pos %d", c, r.pos)
		}
		r.pos++
		r.depth++
		nodeType := r.u8()
		switch nodeType {
		case nodeMapData:
			if err := p.parseMapData(); err != nil {
				return err
			}
		default:
			p.skipNode()
		}
	}
	return nil
}

// parseMapData reads map attributes then its tile-area / towns children.
func (p *parser) parseMapData() error {
	r := p.r
	// Inline map attributes.
	for {
		tag := r.u8()
		switch tag {
		case attrDescription:
			p.res.Description = r.str()
		case attrExtFile, attrExtSpawnMon, attrExtSpawnNPC, attrExtHouse, 24: // 24 = EXT_ZONE_FILE at map level
			_ = r.str()
		default:
			r.pos-- // not an attribute; a control byte / child
			goto children
		}
	}
children:
	for {
		c := r.peekRaw()
		if c == 0xFF {
			r.pos++
			r.depth--
			return nil
		}
		if c != 0xFE {
			return fmt.Errorf("otbm: unexpected byte 0x%02X in map data pos %d", c, r.pos)
		}
		r.pos++
		r.depth++
		nodeType := r.u8()
		switch nodeType {
		case nodeTileArea:
			p.parseTileArea()
		case nodeTownsGrp:
			p.parseTowns()
		case nodeWaypoints:
			p.skipNode()
		default:
			p.skipNode()
		}
	}
}

func (p *parser) parseTileArea() {
	r := p.r
	baseX := r.u16()
	baseY := r.u16()
	baseZ := r.u8()
	for {
		c := r.peekRaw()
		if c == 0xFF {
			r.pos++
			r.depth--
			return
		}
		if c != 0xFE {
			return
		}
		r.pos++
		r.depth++
		nodeType := r.u8()
		switch nodeType {
		case nodeTile:
			p.parseTile(baseX, baseY, baseZ, false)
		case nodeHouseTile:
			p.parseTile(baseX, baseY, baseZ, true)
		default:
			p.skipNode()
		}
	}
}

func (p *parser) parseTile(baseX, baseY uint16, baseZ uint8, house bool) {
	r := p.r
	xOff := r.u8()
	yOff := r.u8()
	pos := game.Position{X: baseX + uint16(xOff), Y: baseY + uint16(yOff), Z: baseZ}
	tile := &game.Tile{}

	if house {
		_ = r.u32() // house id
	}

	// Inline tile attributes.
	for {
		tag := r.u8()
		if tag == attrTileFlags {
			tile.Flags = r.u32()
			continue
		}
		if tag == attrItem {
			id := r.u16()
			p.addItemToTile(tile, &game.Item{ID: id})
			continue
		}
		r.pos-- // control byte / child node
		break
	}

	// Child nodes (items, zones).
	for {
		c := r.peekRaw()
		if c == 0xFF {
			r.pos++
			r.depth--
			break
		}
		if c != 0xFE {
			break
		}
		r.pos++
		r.depth++
		nodeType := r.u8()
		switch nodeType {
		case nodeItem:
			p.parseItemNode(tile)
		case nodeTileZone:
			cnt := int(r.u16())
			for i := 0; i < cnt; i++ {
				_ = r.u16()
			}
			p.expectEndNode()
		default:
			p.skipNode()
		}
	}

	if tile.Ground != nil || len(tile.Items) > 0 || tile.Flags != 0 {
		p.m.SetTile(pos, tile)
		p.res.TileCount++
	}
}

// parseItemNode reads an OTBM_ITEM node (id + attrs + container children).
func (p *parser) parseItemNode(tile *game.Tile) {
	p.addItemToTile(tile, p.readItem())
}

// readItem reads an item node (id, attributes, and any container children) and
// returns the constructed item with its contents populated.
func (p *parser) readItem() *game.Item {
	r := p.r
	id := r.u16()
	count := uint16(0)
	var teleDest *game.Position
	var actionID, uniqueID *uint16

	// Item attributes.
attrLoop:
	for {
		tag := r.u8()
		switch tag {
		case attrCount, attrRuneCharges:
			count = uint16(r.u8())
		case attrCharges:
			count = r.u16()
		case attrActionID:
			v := r.u16()
			actionID = &v
		case attrUniqueID:
			// The unique id keys map-placed movements/actions (e.g. the temple
			// "citizen" set-town tiles register by uid); it must be stored.
			v := r.u16()
			uniqueID = &v
		case attrDepotID:
			_ = r.u16()
		case attrHouseDoorID, attrDecayState, attrShootRange:
			_ = r.u8()
		case attrHitChance:
			_ = r.u8()
		case attrTeleDest:
			tx := r.u16()
			ty := r.u16()
			tz := r.u8()
			teleDest = &game.Position{X: tx, Y: ty, Z: tz}
		case attrText, attrDesc, attrName, attrArticle, attrPluralName, attrSpecial, attrWrittenBy:
			_ = r.str()
		case attrWrittenDate:
			_ = r.u64()
		case attrDuration, attrAttack, attrDefense, attrExtraDefense, attrArmor:
			_ = r.u32()
		case attrWeight:
			_ = r.u32()
		case attrTier:
			_ = r.u8()
		default:
			r.pos-- // control byte / child node
			break attrLoop
		}
	}

	it := &game.Item{ID: id, Count: count}
	if teleDest != nil || actionID != nil || uniqueID != nil {
		it.Attr = &game.ItemAttributes{
			TeleDest: teleDest,
			ActionID: actionID,
			UniqueID: uniqueID,
		}
	}

	// Container children: nested item nodes become this item's contents.
	for {
		c := r.peekRaw()
		if c == 0xFF {
			r.pos++
			r.depth--
			break
		}
		if c != 0xFE {
			break
		}
		r.pos++
		r.depth++
		nodeType := r.u8()
		if nodeType == nodeItem {
			it.Contents = append(it.Contents, p.readItem())
		} else {
			p.skipNode()
		}
	}

	return it
}

// addItemToTile classifies an item as ground or stacked via the catalog.
func (p *parser) addItemToTile(tile *game.Tile, it *game.Item) {
	if it == nil || it.ID == 0 {
		return
	}
	if t := p.cat.Get(it.ID); t != nil && t.IsGround() {
		tile.Ground = it
	} else {
		tile.Items = append(tile.Items, it)
	}
	p.res.ItemCount++
}

func (p *parser) parseTowns() {
	r := p.r
	for {
		c := r.peekRaw()
		if c == 0xFF {
			r.pos++
			r.depth--
			return
		}
		if c != 0xFE {
			return
		}
		r.pos++
		r.depth++
		nodeType := r.u8()
		if nodeType == nodeTown {
			id := r.u32()
			name := r.str()
			x := r.u16()
			y := r.u16()
			z := r.u8()
			p.res.Towns = append(p.res.Towns, Town{ID: id, Name: name, Pos: game.Position{X: x, Y: y, Z: z}})
			p.expectEndNode()
		} else {
			p.skipNode()
		}
	}
}

// skipNode consumes the current node and all its children (already past type).
func (p *parser) skipNode() {
	r := p.r
	for {
		if r.eof() {
			return
		}
		c := r.peekRaw()
		if c == 0xFF {
			r.pos++
			r.depth--
			return
		}
		if c == 0xFE {
			r.pos++
			r.depth++
			_ = r.u8() // child type
			p.skipNode()
			continue
		}
		// property byte
		r.u8()
	}
}

// expectEndNode consumes a trailing 0xFF for leaf nodes.
func (p *parser) expectEndNode() {
	r := p.r
	if !r.eof() && r.peekRaw() == 0xFF {
		r.pos++
		r.depth--
	}
}
