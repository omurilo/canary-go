package protocol

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/opentibiabr/canary-go/internal/game"
)

// tileHasMailbox checks whether any item on the tile has the "mailbox" type.
func (g *GameProtocol) tileHasMailbox(tile *game.Tile) bool {
	if tile == nil {
		return false
	}
	for _, it := range tile.Items {
		if it != nil {
			if itemType := g.deps.Items.Get(it.ID); itemType != nil && itemType.IsMailbox() {
				return true
			}
		}
	}
	return false
}

// isMailItem returns true when the item is a sendable letter or parcel.
func isMailItem(item *game.Item) bool {
	if item == nil {
		return false
	}
	return item.ID == game.ItemLetter || item.ID == game.ItemParcel
}

// getMailRecipient extracts the recipient name from a letter or parcel.
// For parcels, it looks for a label (ITEM_LABEL) inside the container.
// For letters, it reads the first line of the item's text attribute.
func getMailRecipient(item *game.Item) string {
	if item.ID == game.ItemParcel {
		for _, content := range item.Contents {
			if content != nil && content.ID == game.ItemLabel {
				text := itemText(content)
				if text != "" {
					return firstLine(text)
				}
			}
		}
		return ""
	}
	return firstLine(itemText(item))
}

// itemText returns the Text attribute of an item, or empty string.
func itemText(item *game.Item) string {
	if item.Attr != nil && item.Attr.Text != nil {
		return *item.Attr.Text
	}
	return ""
}

// itemWrittenBy returns the writer name attribute of an item.
func itemWrittenBy(item *game.Item) string {
	if item.Attr != nil && item.Attr.WrittenBy != nil {
		return *item.Attr.WrittenBy
	}
	return ""
}

// itemWrittenDate returns the written date attribute of an item.
func itemWrittenDate(item *game.Item) uint64 {
	if item.Attr != nil && item.Attr.WrittenDate != nil {
		return *item.Attr.WrittenDate
	}
	return 0
}

// firstLine returns the first line of s, trimmed.
func firstLine(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return strings.TrimSpace(s[:idx])
	}
	return strings.TrimSpace(s)
}

// processMailSend attempts to deliver item (a letter/parcel dropped on a
// mailbox) to the player named in the item's text or label. Returns true if
// mail was sent.
func (g *GameProtocol) processMailSend(item *game.Item) bool {
	if !isMailItem(item) {
		return false
	}

	receiverName := getMailRecipient(item)
	if receiverName == "" {
		g.sendCancelMessage("You must write a recipient name first.")
		return false
	}

	// Look up recipient: online first.
	recipient := g.deps.World.PlayerByName(receiverName)
	if recipient != nil {
		if recipient.Inbox == nil {
			recipient.Inbox = &game.Item{ID: game.ItemInbox, Contents: make([]*game.Item, 0), Pagination: true}
		}
		return g.deliverToInbox(item, recipient.Inbox, recipient.Name, true)
	}

	// Offline lookup.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	recipientPlayer, err := g.deps.DB.LoadPlayer(ctx, receiverName)
	if err != nil || recipientPlayer == nil {
		g.sendCancelMessage(fmt.Sprintf("Player '%s' does not exist.", receiverName))
		return false
	}

	return g.deliverToOfflineInbox(item, recipientPlayer.DBID, receiverName)
}

// deliverToInbox moves the item to the recipient's in-memory inbox,
// transforms it to the stamped variant, and notifies the recipient.
func (g *GameProtocol) deliverToInbox(item *game.Item, inbox *game.Item, recipientName string, online bool) bool {
	stamped := g.makeStampedItem(item)
	if inbox.Contents == nil {
		inbox.Contents = make([]*game.Item, 0)
	}
	inbox.Contents = append(inbox.Contents, stamped)

	if online {
		g.deps.Lua.Call("onPlayerReceiveMail", recipientName)
	}
	g.deps.Log.Info("mail delivered", "item", item.ID, "to", recipientName, "online", online)
	g.sendStatusText(fmt.Sprintf("Message sent to %s.", recipientName))
	return true
}

// deliverToOfflineInbox saves the stamped item directly to the database inbox.
func (g *GameProtocol) deliverToOfflineInbox(item *game.Item, recipientID uint32, recipientName string) bool {
	stamped := g.makeStampedItem(item)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sid := int32(stamped.ID)
	_, err := g.deps.DB.SQL.ExecContext(ctx,
		`INSERT INTO player_inboxitems (player_id, sid, pid, itemtype, count, attributes)
		 VALUES (?, ?, 0, ?, ?, '')
		 ON DUPLICATE KEY UPDATE itemtype=VALUES(itemtype), count=VALUES(count)`,
		recipientID, sid, stamped.ID, int32(stamped.Count),
	)
	if err != nil {
		g.deps.Log.Warn("mail: failed to save for offline recipient", "to", recipientName, "err", err)
		g.sendCancelMessage("Could not deliver the mail.")
		return false
	}

	g.deps.Log.Info("mail delivered", "item", item.ID, "to", recipientName, "online", false)
	g.sendStatusText(fmt.Sprintf("Message sent to %s.", recipientName))
	return true
}

// makeStampedItem transforms a letter (3505) or parcel (3503) into its stamped
// variant (3506 / 3504) preserving writer, date and text attributes.
func (g *GameProtocol) makeStampedItem(item *game.Item) *game.Item {
	stamped := &game.Item{
		ID:         item.ID + 1,
		Count:      item.Count,
		Attributes: item.Attributes,
		Contents:   item.Contents,
	}
	if item.Attr != nil {
		stamped.Attr = &game.ItemAttributes{}
		if item.Attr.Text != nil {
			t := *item.Attr.Text
			stamped.Attr.Text = &t
		}
		if item.Attr.WrittenBy != nil {
			w := *item.Attr.WrittenBy
			stamped.Attr.WrittenBy = &w
		}
		if item.Attr.WrittenDate != nil {
			d := *item.Attr.WrittenDate
			stamped.Attr.WrittenDate = &d
		}
	}
	return stamped
}


