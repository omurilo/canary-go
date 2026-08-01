// Package protocol implements the Tibia login (7171) and game (7172) protocols
// on top of the network/transport layers.
package protocol

import (
	"log/slog"

	"github.com/omurilo/canary-go/internal/config"
	"github.com/omurilo/canary-go/internal/db"
	"github.com/omurilo/canary-go/internal/events"
	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/items"
	"github.com/omurilo/canary-go/internal/luaengine"
	"github.com/omurilo/canary-go/internal/tibcrypto"
)

// ClientVersion is the protocol version the server speaks (Tibia 13.x).
const ClientVersion = 1525

// SpeedFormula constants used to encode player speed for the client.
const ServerBeat = 50

// Deps bundles the shared services both protocols need.
type Deps struct {
	Cfg   *config.Config
	DB    *db.DB
	RSA   *tibcrypto.RSA
	World *game.World
	Items  *items.Catalog
	Lua    *luaengine.Engine
	Events *events.Engine
	Log    *slog.Logger
}
