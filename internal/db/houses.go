package db

import (
	"context"
	"database/sql"
	"time"

	"github.com/opentibiabr/canary-go/internal/game"
)

// LoadHouses loads all houses from the houses table into the world.
func (d *DB) LoadHouses(ctx context.Context, w *game.World) error {
	const q = `SELECT id, name, owner, rent, size, beds, town_id, client_id,
		COALESCE(bidder_name,''), COALESCE(highest_bid,0), COALESCE(internal_bid,0),
		COALESCE(bid_holder_limit,0), COALESCE(bid_end_date,0), COALESCE(bidder,0),
		COALESCE(transfer_to_name,''), COALESCE(transfer_price,0), COALESCE(transfer_accept,0)
		FROM houses`
	rows, err := d.SQL.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var h game.House
		h.RentPeriod = "monthly"
		var ownerID sql.NullInt64
		if err := rows.Scan(&h.ID, &h.Name, &ownerID, &h.Rent, &h.Size, &h.Beds, &h.TownID, &h.ClientID,
			&h.BidderName, &h.HighestBid, &h.InternalBid, &h.BidHolderLimit, &h.BidEndDate, &h.Bidder,
			&h.TransferToName, &h.TransferPrice, &h.TransferAccept); err != nil {
			continue
		}
		if ownerID.Valid {
			h.OwnerID = uint32(ownerID.Int64)
		}
		w.RegisterHouse(&h)
	}
	return nil
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

// SaveHouseOwner persists the owner of a house.
func (d *DB) SaveHouseOwner(ctx context.Context, houseID uint32, ownerID uint32) error {
	_, err := d.SQL.ExecContext(ctx, `UPDATE houses SET owner = ? WHERE id = ?`, ownerID, houseID)
	return err
}

// SaveHouseBid persists the bid information for a house.
func (d *DB) SaveHouseBid(ctx context.Context, h *game.House) error {
	const q = `UPDATE houses SET bidder_name=?, highest_bid=?, internal_bid=?,
		bid_holder_limit=?, bid_end_date=?, bidder=? WHERE id=?`
	_, err := d.SQL.ExecContext(ctx, q,
		h.BidderName, h.HighestBid, h.InternalBid,
		h.BidHolderLimit, h.BidEndDate, h.Bidder, h.ID)
	return err
}

// ClearHouseBid clears the bid info for a house (e.g. after auction ends).
func (d *DB) ClearHouseBid(ctx context.Context, houseID uint32) error {
	const q = `UPDATE houses SET bidder_name='', highest_bid=0, internal_bid=0,
		bid_holder_limit=0, bid_end_date=0, bidder=0 WHERE id=?`
	_, err := d.SQL.ExecContext(ctx, q, houseID)
	return err
}

// SaveHouseTransfer persists the transfer info for a house.
func (d *DB) SaveHouseTransfer(ctx context.Context, h *game.House) error {
	const q = `UPDATE houses SET transfer_to_name=?, transfer_price=?, transfer_accept=? WHERE id=?`
	_, err := d.SQL.ExecContext(ctx, q,
		h.TransferToName, h.TransferPrice, h.TransferAccept, h.ID)
	return err
}

// ClearHouseTransfer clears the transfer info for a house.
func (d *DB) ClearHouseTransfer(ctx context.Context, houseID uint32) error {
	const q = `UPDATE houses SET transfer_to_name='', transfer_price=0, transfer_accept=0 WHERE id=?`
	_, err := d.SQL.ExecContext(ctx, q, houseID)
	return err
}

// SaveHouseAccessList persists the access list for a house.
func (d *DB) SaveHouseAccessList(ctx context.Context, houseID uint32, access game.AccessList) error {
	tx, err := d.SQL.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM house_lists WHERE house_id = ?`, houseID); err != nil {
		return err
	}
	for _, player := range access.Players {
		if _, err := tx.ExecContext(ctx, `INSERT INTO house_lists (house_id, type, value) VALUES (?, 'player', ?)`, houseID, player); err != nil {
			return err
		}
	}
	for _, guild := range access.Guilds {
		if _, err := tx.ExecContext(ctx, `INSERT INTO house_lists (house_id, type, value) VALUES (?, 'guild', ?)`, houseID, guild); err != nil {
			return err
		}
	}
	return tx.Commit()
}


// HouseBidExpiryDuration is how long an auction lasts after the first bid.
const HouseBidExpiryDuration = 7 * 24 * time.Hour // 7 days
