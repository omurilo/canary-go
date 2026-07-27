package db

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"

	"github.com/opentibiabr/canary-go/internal/game"
)

// houseEntry is one <house> element in the houses XML.
type houseEntry struct {
	XMLName xml.Name `xml:"house"`
	ID      int      `xml:"houseid,attr"`
	Name    string   `xml:"name,attr"`
	EntryX  int      `xml:"entryx,attr"`
	EntryY  int      `xml:"entryy,attr"`
	EntryZ  int      `xml:"entryz,attr"`
	Rent    int      `xml:"rent,attr"`
	TownID  int      `xml:"townid,attr"`
	Size    int      `xml:"size,attr"`
	Beds    int      `xml:"beds,attr"`
	Guild   bool     `xml:"guildhall,attr"`
	ClientID int      `xml:"clientid,attr"`
}

// ParseHouseFile parses an OTBR-style houses XML and returns a slice of houses.
func (d *DB) ParseHouseFile(path string) ([]game.House, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read house file %q: %w", path, err)
	}

	var doc struct {
		XMLName xml.Name     `xml:"houses"`
		Houses  []houseEntry `xml:"house"`
	}
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse house file %q: %w", path, err)
	}

	var houses []game.House
	for _, hx := range doc.Houses {
		h := game.House{
			ID:         uint32(hx.ID),
			Name:       hx.Name,
			Rent:       uint32(hx.Rent),
			Size:       uint32(hx.Size),
			Beds:       uint8(hx.Beds),
			TownID:     uint16(hx.TownID),
			ClientID:   uint32(hx.ClientID),
			RentPeriod: "monthly",
			Position:   game.Position{X: uint16(hx.EntryX), Y: uint16(hx.EntryY), Z: uint8(hx.EntryZ)},
		}
		if hx.Guild {
			h.OwnerID = 0 // guild halls have no individual owner
		}
		houses = append(houses, h)
	}
	return houses, nil
}

// SaveHouse inserts or updates a house record in the database.
func (d *DB) SaveHouse(ctx context.Context, h *game.House) error {
	const q = `INSERT INTO houses (id, name, owner, rent, size, beds, town_id, client_id)
	           VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	           ON DUPLICATE KEY UPDATE name=?, rent=?, size=?, beds=?, town_id=?, client_id=?`
	_, err := d.SQL.ExecContext(ctx, q,
		h.ID, h.Name, h.OwnerID, h.Rent, h.Size, h.Beds, h.TownID, h.ClientID,
		h.Name, h.Rent, h.Size, h.Beds, h.TownID, h.ClientID)
	return err
}
