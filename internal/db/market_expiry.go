package db

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/omurilo/canary-go/internal/config"
	"github.com/omurilo/canary-go/internal/game"
)

// Market offer expiry, the port of IOMarket::checkExpiredOffers and
// processExpiredOffers (src/io/iomarket.cpp:155,234). Offers never expired here, so
// gold escrowed for a buy offer and items handed over for a sell offer stayed locked
// away forever.
//
// Expiring must give the value back. An expiry that only deletes the row is
// confiscation, which is why this refunds before it reports success.

// Config defaults from config.lua.dist.
const (
	defaultMarketOfferDays        = 30
	defaultExpiryCheckIntervalMin = 60
)

// MarketOfferDuration is how long an offer stays on the market.
func MarketOfferDuration() time.Duration {
	days := config.Number("marketOfferDuration", defaultMarketOfferDays)
	if days <= 0 {
		days = defaultMarketOfferDays
	}
	return time.Duration(days) * 24 * time.Hour
}

// MarketExpiryInterval is how often the sweep runs; 0 disables it, as upstream's
// non-positive check does.
func MarketExpiryInterval() time.Duration {
	minutes := config.Number("checkExpiredMarketOffersEachMinutes", defaultExpiryCheckIntervalMin)
	if minutes <= 0 {
		return 0
	}
	return time.Duration(minutes) * time.Minute
}

// ExpireMarketOffers moves every offer past its duration into history and returns
// what it was holding: the escrowed gold for a buy offer, the items for a sell one.
// Returns how many were expired.
func (d *DB) ExpireMarketOffers(ctx context.Context, world *game.World) (int, error) {
	cutoff := time.Now().Add(-MarketOfferDuration()).Unix()
	rows, err := d.SQL.QueryContext(ctx,
		"SELECT `id`, `amount`, `price`, `itemtype`, `player_id`, `sale`, `tier` FROM `market_offers` WHERE `created` <= ?",
		cutoff)
	if err != nil {
		return 0, err
	}

	type expired struct {
		id       uint32
		amount   uint16
		price    uint64
		itemType uint16
		playerID uint32
		sale     uint8
		tier     uint8
	}
	var batch []expired
	for rows.Next() {
		var e expired
		if err := rows.Scan(&e.id, &e.amount, &e.price, &e.itemType, &e.playerID, &e.sale, &e.tier); err != nil {
			rows.Close()
			return 0, err
		}
		batch = append(batch, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	count := 0
	for _, e := range batch {
		// Move FIRST and only refund if it moved. C++ has the same ordering with a
		// `continue`, and it is what stops a second sweep from refunding an offer
		// twice: once the row is gone it cannot be selected again.
		moved, err := d.MoveOfferToHistory(ctx, e.id, OfferStateExpired)
		if err != nil {
			slog.Default().Warn("expire market offer", "offer", e.id, "err", err)
			continue
		}
		if !moved {
			continue
		}
		count++

		if e.sale == 1 {
			if err := d.returnExpiredItems(ctx, world, e.playerID, e.itemType, e.amount, e.tier); err != nil {
				slog.Default().Error("expired sell offer items not returned",
					"player", e.playerID, "item", e.itemType, "amount", e.amount, "err", err)
			}
			continue
		}
		total := e.price * uint64(e.amount)
		if err := d.refundBankBalance(ctx, world, e.playerID, total); err != nil {
			slog.Default().Error("expired buy offer gold not refunded",
				"player", e.playerID, "amount", total, "err", err)
		}
	}
	// The market keeps its own in-memory copy; drop what the database no longer has.
	if world != nil && world.Market != nil {
		for _, e := range batch {
			world.Market.RemoveOffer(e.id)
		}
	}
	return count, nil
}

// refundBankBalance credits an expired buy offer's escrow, to the live player when
// they are online and straight to the row when they are not
// (IOLoginData::increaseBankBalance).
func (d *DB) refundBankBalance(ctx context.Context, world *game.World, playerID uint32, amount uint64) error {
	if amount == 0 {
		return nil
	}
	if world != nil {
		for _, p := range world.Players() {
			if p.DBID == playerID {
				p.BankBalance += amount
				return nil
			}
		}
	}
	_, err := d.SQL.ExecContext(ctx, "UPDATE `players` SET `balance` = `balance` + ? WHERE `id` = ?", amount, playerID)
	return err
}

// returnExpiredItems puts a sell offer's goods back in the owner's inbox. Stackables
// are split into stacks of 100 and non-stackables are created one by one, matching
// processExpiredOffers.
func (d *DB) returnExpiredItems(ctx context.Context, world *game.World, playerID uint32,
	itemType uint16, amount uint16, tier uint8,
) error {
	if amount == 0 {
		return nil
	}
	var cat = world.Items
	t := cat.Get(itemType)
	if t == nil {
		return fmt.Errorf("item type %d is not in the catalog", itemType)
	}

	// Build the stacks first so online and offline delivery share the split.
	type stack struct {
		count uint16
	}
	var stacks []stack
	if t.Stackable {
		for left := amount; left > 0; {
			n := uint16(100)
			if left < n {
				n = left
			}
			stacks = append(stacks, stack{count: n})
			left -= n
		}
	} else {
		// A non-stackable carries its charges as the subtype, or nothing.
		sub := uint16(0)
		if t.Charges != 0 {
			sub = uint16(t.Charges)
		}
		for i := uint16(0); i < amount; i++ {
			stacks = append(stacks, stack{count: sub})
		}
	}

	// Online: hand them to the live inbox so the player sees them without relogging.
	if world != nil {
		for _, p := range world.Players() {
			if p.DBID != playerID {
				continue
			}
			if p.Inbox == nil {
				p.Inbox = &game.Item{ID: game.ItemInbox, Container: game.NewContainer(20)}
			}
			for _, s := range stacks {
				it := &game.Item{ID: itemType, Count: s.count}
				if tier != 0 {
					it.SetTier(tier)
				}
				if p.Inbox.Container != nil {
					p.Inbox.Container.Contents = append(p.Inbox.Container.Contents, it)
					// We only set the parent if 'it' is also a container!
					if it.Container != nil {
						it.Container.Parent = p.Inbox
					}
				}
			}
			return nil
		}
	}

	// Offline: write straight into player_inboxitems. sid is allocated from the
	// current maximum — the mail path reuses the item id as the sid, which collides
	// between two items of the same type and then overwrites one via ON DUPLICATE KEY.
	tx, err := d.SQL.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var maxSID int32
	if err := tx.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(`sid`), 0) FROM `player_inboxitems` WHERE `player_id` = ?", playerID).Scan(&maxSID); err != nil {
		return err
	}
	for _, s := range stacks {
		maxSID++
		attrs := []byte{}
		if tier != 0 {
			it := &game.Item{ID: itemType, Count: s.count}
			it.SetTier(tier)
			if it.Attr != nil {
				attrs = it.Attr.Encode(s.count)
			}
		}
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO `player_inboxitems` (`player_id`, `sid`, `pid`, `itemtype`, `count`, `attributes`) VALUES (?, ?, 0, ?, ?, ?)",
			playerID, maxSID, itemType, int32(s.count), attrs); err != nil {
			return err
		}
	}
	return tx.Commit()
}
