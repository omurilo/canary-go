package protocol

import (
	"context"
	"time"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// parseHighscores handles the highscore request (opcode 0xB9).
// Packet format:
//
//	u8: type (0=get entries, 1=get categories)
//	u8: categoryID
//	u32: vocationID (0=all, 1-4=specific)
//	str: worldName (empty = current)
//	u16: page
//	u8: entriesPerPage
func (g *GameProtocol) parseHighscores(r *netmsg.Reader) {
	reqType := r.GetByte()
	categoryID := r.GetByte()
	vocationID := r.GetU32()
	worldName := r.GetString()
	page := r.GetU16()
	entriesPerPage := r.GetByte()

	_ = worldName

	if game.HighscoreType(reqType) == game.HighscoreGetCategories {
		g.sendHighscoreCategories()
		return
	}

	// Load from DB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	entries, totalPages, err := g.deps.DB.LoadHighscore(ctx, categoryID, vocationID, page, entriesPerPage)
	if err != nil || len(entries) == 0 {
		g.sendHighscoresNoData()
		return
	}

	g.sendHighscores(entries, categoryID, vocationID, page, uint16(totalPages), 60)
}

// sendHighscoreCategories sends the list of categories (0xB9 response).
func (g *GameProtocol) sendHighscoreCategories() {
	w := netmsg.NewWriter()
	w.AddByte(0xB9)
	w.AddByte(0x00) // no data flag
	w.AddByte(byte(len(game.DefaultHighscoreCategories)))
	for _, cat := range game.DefaultHighscoreCategories {
		w.AddString(cat.Name)
		w.AddByte(cat.ID)
	}
	g.SendToClient(w)
}

// sendHighscores sends highscore entries to the client.
func (g *GameProtocol) sendHighscores(entries []game.HighscoreEntry, categoryID uint8, vocationID uint32, page, totalPages uint16, updateTimer uint32) {
	w := netmsg.NewWriter()
	w.AddByte(0xB9)
	w.AddByte(0x01) // data flag
	w.AddByte(categoryID)
	w.AddU32(vocationID)
	w.AddU16(page)
	w.AddU16(totalPages)
	w.AddU32(updateTimer)
	w.AddU16(uint16(len(entries)))
	for _, e := range entries {
		w.AddU16(e.Rank)
		w.AddString(e.Name)
		w.AddU16(e.Level)
		w.AddByte(e.Vocation)
		w.AddU32(e.Value)
		w.AddU32(e.TownID)
	}
	g.SendToClient(w)
}

// sendHighscoresNoData sends an empty highscore response.
func (g *GameProtocol) sendHighscoresNoData() {
	w := netmsg.NewWriter()
	w.AddByte(0xB9)
	w.AddByte(0x00) // no data
	w.AddByte(0x00) // zero categories
	g.SendToClient(w)
}
