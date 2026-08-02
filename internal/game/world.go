package game

import (
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/omurilo/canary-go/internal/bosstiary"
	"github.com/omurilo/canary-go/internal/charms"
	"github.com/omurilo/canary-go/internal/creatures"
	"github.com/omurilo/canary-go/internal/game/imbuements"
	"github.com/omurilo/canary-go/internal/items"
)

// World is the authoritative in-memory game state: the map plus all online
// players. It is safe for concurrent use.
type World struct {
	mu                  sync.RWMutex
	Map                 *Map
	Towns               map[string]Position
	TownsByID           map[uint16]Position // town id -> temple position (from the OTBM)
	TownNames           map[uint16]string   // town id -> name (from the OTBM)
	DefaultSpawn        Position
	WorldType           uint8 // 1 = WORLD_TYPE_NO_PVP, 2 = WORLD_TYPE_PVP, 3 = WORLD_TYPE_PVP_ENFORCED
	AutoBank            bool
	MaxContainer        uint32 // MAX_CONTAINER; 0 means unconfigured, i.e. no limit
	OnModalWindowAnswer func(p *Player, id uint32, button uint8, choice uint8)
	ChatManager         *ChatManager
	WaitingList         *WaitingList
	BoostedCreature     string
	BoostedBoss         string
	players             map[uint32]*Player
	byName              map[string]*Player
	creatures           map[uint32]Creature
	nextCreatureID      atomic.Uint32
	guilds              map[uint32]*Guild

	Items               *items.Catalog
	Monsters            *creatures.TypeRegistry
	Charms              *charms.Registry
	Imbuements          *imbuements.Registry
	Achievements        *AchievementRegistry
	ItemClassifications map[uint8]*ItemClassification
	Dispatcher          *WDRRDispatcher
	Houses              map[uint32]*House
	Market              *Market
	Decay               *DecayManager
	BrowseFields        map[Position]*Item

	// oldStackPos maps a spectating player's creature id to the client stack index
	// the moving/removed creature occupied in THAT player's view, captured while it
	// was still on the tile. -1, or a missing key, means the spectator could not see
	// it and must receive no packet at all. C++ builds the same per-spectator vector
	// before the removal (Map::moveCreature, src/map/map.cpp:739-747); reconstructing
	// a single index afterwards is wrong for every spectator whose view differs, and
	// races with concurrent edits to the tile.
	OnCreatureMove   func(c Creature, oldPos Position, newPos Position, oldStackPos map[uint32]int)
	OnCreatureAppear func(c Creature)
	OnCreatureRemove func(c Creature, oldStackPos map[uint32]int)
	// OnCreatureOutfitChange fires when a creature's outfit is changed by a script
	// (creature:setOutfit → internalCreatureChangeOutfit in C++). The hook
	// broadcasts the new appearance to spectators.
	OnCreatureOutfitChange func(c Creature)
	OnGhostModeChange      func(p *Player)

	// CaptureStackPositions is populated by the protocol layer. It is always called
	// with w.mu already held, so its implementation must not take the lock again.
	CaptureStackPositions func(pos Position, c Creature) map[uint32]int

	// OnUseWeapon is populated by the Lua engine. It runs a datapack weapon's
	// onUseWeapon callback for the wielded item and reports whether one existed.
	// Weapon::internalUseWeapon branches on exactly this: with a script attached it
	// runs the script INSTEAD of the built-in damage (weapons.cpp), which is what the
	// arrow/star scripts rely on to apply their own conditions and effects.
	OnUseWeapon func(p *Player, weaponItemID uint16, target Creature) bool

	// Combat hooks, populated by the protocol layer so the combat engine can
	// push updates to clients without importing the protocol package.
	OnCreatureHealthChange func(c Creature)
	OnCombatHit            func(attacker, victim Creature, damage int32, effect uint16)
	OnItemAppear           func(pos Position, item *Item)
	OnTileUpdate           func(pos Position)
	OnContainerAddItem     func(p *Player, container *Item, item *Item)
	OnTargetLost           func(p *Player)
	// OnPlayerStatsChange pushes a refreshed stats packet (0xA0) after
	// experience/level changes, e.g. on a monster kill.
	OnPlayerStatsChange func(p *Player)

	// OnTextMessage pushes a text message / animated text packet (0xB4) to a player.
	OnTextMessage func(p *Player, class uint8, value uint64, text string)

	// OnPlayerDeath is fired when a player's health reaches 0. The protocol
	// layer applies the death penalty, teleports the player to their temple,
	// and refreshes the client (the model-side penalty is applied before this
	// callback runs).
	OnPlayerDeath    func(p *Player, killer Creature)
	OnCreatureDied   func(c Creature) // monster/NPC death, fires before RemoveCreature
	OnGainExperience func(p *Player, source Creature, exp uint64, rawExp uint64) uint64

	// OnHouseOwnerChange persists a house ownership change, the UPDATE `houses`
	// that House::setOwner runs inline (src/map/house/house.cpp:95-101). Populated
	// by the persistence layer. Without it `/owner` only ever changed memory.
	OnHouseOwnerChange func(h *House, ownerID uint32)

	// LookupPlayerAccount resolves a player guid to their name and account id —
	// the `SELECT name, account_id FROM players WHERE id = guid` in setOwner
	// (house.cpp:138). ok is false when no such player exists, which makes the
	// ownership assignment abort rather than record an unknown id.
	LookupPlayerAccount func(guid uint32) (name string, accountID uint32, ok bool)

	// OnShieldUpdate asks the protocol layer to send `viewer` a party-shield
	// packet (0x91) for `target`, using viewer.PartyShield(target).
	OnShieldUpdate func(viewer *Player, target *Player)

	// OnItemDecay is triggered when an item transforms due to decay.
	OnItemDecay func(pos Position, stackPos uint8, oldItem, newItem *Item)

	// OnMagicEffect shows a graphical effect on a tile (spell area/impact with no
	// damage text). OnDistanceEffect shows a shoot animation from->to.
	OnMagicEffect    func(pos Position, effect uint16)
	OnDistanceEffect func(from, to Position, effect uint16)
	OnCreatureSay    func(speaker Creature, talkType byte, text string)
	// OnSoundEffect is Game::sendSingleSoundEffect.
	OnSoundEffect func(pos Position, sound uint16)
	// The Npc:: hooks: the script callbacks the datapack attaches, plus the shop
	// window close that Npc::closeAllShopWindows drives.
	OnNpcCreatureSay  func(n *Npc, speaker Creature, talkType byte, text string)
	OnNpcCloseChannel func(n *Npc, p *Player)
	OnCloseShopWindow func(p *Player)
	// OnHouseItemsToDepot hands the items cleared out of a house to the
	// persistence layer, which is where the owner.s depot lives.
	OnHouseItemsToDepot func(h *House, owner *Player, items []*Item)
	OnNpcBuyItem        func(n *Npc, p *Player, itemID uint16, subType uint8, amount uint16, ignore, inBackpacks bool, totalCost uint64)
	OnNpcSellItem       func(n *Npc, p *Player, itemID uint16, subType uint8, amount uint32, ignore bool, totalPrice uint64)
	OnNpcCheckItem      func(n *Npc, p *Player, itemID uint16, subType uint8)
	// The Monster:: script callbacks the datapack attaches through
	// monsterType:eventType. Each is the send half of one Monster::on* handler.
	OnMonsterCreatureSay      func(m *Monster, speaker Creature, talkType byte, text string)
	OnMonsterAttackedByPlayer func(m *Monster, attacker *Player)
	OnMonsterSpawn            func(m *Monster, pos Position)
	OnMonsterCastSpell        func(m *Monster, target Creature, block creatures.MonsterAttack)
	// OnCreatureTurn notifies spectators that a creature changed facing, the
	// send half of Game::internalCreatureTurn. Monster::updateLookDirection
	// fires it on every think, so a monster visibly faces its target.
	OnCreatureTurn   func(c Creature)
	OnCastSpell      func(name string, caster Creature, target Creature) bool
	OnTargetTile     func(funcName string, caster Creature, pos Position)
	OnTargetCreature func(funcName string, caster Creature, target Creature)
	OnChangeSpeed    func(c Creature)
	OnIconsUpdate    func(p *Player)
	// OnBosstiaryEntryChanged fires when a player's boss reaches a new unlock
	// level (protocol layer sends the cyclopedia entry-changed update).
	OnBosstiaryEntryChanged func(p *Player, bossRaceID uint16)
	// OnBestiaryEntryChanged fires when a player's bestiary monster reaches a new
	// unlock stage.
	OnBestiaryEntryChanged func(p *Player, raceID uint16)
	// OnAreaCombat fires when area combat resolves, before damage is applied.
	OnAreaCombat func(creature Creature, tile *Tile, isAggressive bool)
	// OnMonsterDropLoot fires when a monster drops loot (before postDropLoot).
	OnMonsterDropLoot func(monster *Monster, corpse *Item)
	// OnMonsterPostDropLoot fires after loot has been placed in the corpse.
	OnMonsterPostDropLoot func(monster *Monster, corpse *Item)

	// Combat is the world's combat engine, used by the spell system to resolve
	// spell damage/heal through the same hit/death path as melee.
	Combat *CombatEngine

	// Zones is the registry of map/script zones (src/game/zones/). The OTBM's
	// per-tile zone ids and `<map>-zones.xml` both feed into it.
	Zones *ZoneRegistry

	TypeRegistry *creatures.TypeRegistry
	Fiendish     *FiendishManager
	Raids        *Raids
	HirelingMgr  *HirelingManager
}

// PlayerByName returns an online player by (case-insensitive) name, or nil.
func (w *World) PlayerByName(name string) *Player {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.byName[strings.ToLower(strings.TrimSpace(name))]
}

// CreatureByName returns an online creature (player, monster, npc) by (case-insensitive) name, or nil.
func (w *World) CreatureByName(name string) Creature {
	w.mu.RLock()
	defer w.mu.RUnlock()
	target := strings.ToLower(strings.TrimSpace(name))
	if p, ok := w.byName[target]; ok {
		return p
	}
	for _, c := range w.creatures {
		if c != nil && strings.EqualFold(strings.TrimSpace(c.GetName()), target) {
			return c
		}
	}
	return nil
}

// NewWorld creates an empty world with a fresh map.
func NewWorld() *World {
	w := &World{
		Map:                 NewMap(),
		Towns:               make(map[string]Position),
		TownsByID:           make(map[uint16]Position),
		TownNames:           make(map[uint16]string),
		players:             make(map[uint32]*Player),
		byName:              make(map[string]*Player),
		creatures:           make(map[uint32]Creature),
		guilds:              make(map[uint32]*Guild),
		TypeRegistry:        creatures.NewTypeRegistry(),
		Charms:              charms.NewRegistry(),
		Imbuements:          imbuements.NewRegistry(),
		ItemClassifications: make(map[uint8]*ItemClassification),
		Market:              NewMarket(),
		Fiendish:            NewFiendishManager(3),
		HirelingMgr:         NewHirelingManager(),
	}
	w.Zones = NewZoneRegistry(w)
	w.Combat = NewCombatEngine(w)
	w.Decay = NewDecayManager(w)
	w.nextCreatureID.Store(0x10000000) // player creature ids start high, like TFS
	return w
}

// TempleByTownID returns the temple position for a town id (from the OTBM),
// used to respawn a player at the temple of the town they belong to.
func (w *World) TempleByTownID(id uint16) (Position, bool) {
	p, ok := w.TownsByID[id]
	return p, ok
}

// TownNameByID returns the town's name, or "" when unknown.
func (w *World) TownNameByID(id uint16) string { return w.TownNames[id] }

// TownTemple returns a town's temple position by (case-insensitive) name.
func (w *World) TownTemple(name string) (Position, bool) {
	p, ok := w.Towns[strings.ToLower(strings.TrimSpace(name))]
	return p, ok
}

// TownIDByName returns the town id for a (case-insensitive) town name.
func (w *World) TownIDByName(name string) (uint16, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	target := strings.ToLower(strings.TrimSpace(name))
	for id, tName := range w.TownNames {
		if strings.ToLower(strings.TrimSpace(tName)) == target {
			return id, true
		}
	}
	return 0, false
}

// SetPosition moves a player to an absolute position under the world lock,
// correctly updating the tile creature tracking.
func (w *World) SetPosition(p *Player, pos Position) {
	w.mu.Lock()
	p.IsTraining = false
	w.removeCreatureFromTile(p)
	p.Pos = pos
	w.addCreatureToTile(p)
	w.mu.Unlock()
}

// AddItem appends an item to the map and triggers decay if applicable.
func (w *World) AddItem(pos Position, it *Item) bool {
	if !w.Map.AddItem(pos, it) {
		return false
	}
	// Refresh open browse field for this tile
	if bf := w.BrowseFieldGet(pos); bf != nil {
		bf.Contents = append([]*Item{it}, bf.Contents...)
	}
	// Notify nearby players about the new item
	if w.OnItemAppear != nil {
		w.OnItemAppear(pos, it)
	}
	if w.Items != nil && w.Decay != nil {
		if itemType := w.Items.Get(it.ID); itemType != nil && itemType.Duration > 0 && itemType.DecayTo > 0 {
			w.Decay.StartDecaying(pos, it, itemType.Duration, itemType.DecayTo)
		}
	}
	return true
}

// BrowseFieldGet returns the browse field container for the given tile position, or nil.
func (w *World) BrowseFieldGet(pos Position) *Item {
	if w.BrowseFields == nil {
		return nil
	}
	return w.BrowseFields[pos]
}

// BrowseFieldSet registers a browse field container for the given tile position.
func (w *World) BrowseFieldSet(pos Position, c *Item) {
	if w.BrowseFields == nil {
		w.BrowseFields = make(map[Position]*Item)
	}
	w.BrowseFields[pos] = c
}

// BrowseFieldRemove unregisters the browse field for the given tile position.
func (w *World) BrowseFieldRemove(pos Position) {
	delete(w.BrowseFields, pos)
}

// InternalRemoveItem removes count from an item on a tile and broadcasts the update.
// Mirrors C++ Game::internalRemoveItem for tile items.
func (w *World) InternalRemoveItem(pos Position, item *Item, count uint16) {
	if int(item.Count) > int(count) {
		item.Count -= count
	} else {
		w.RemoveMapItem(pos, item)
		// Remove from open browse field
		if bf := w.BrowseFieldGet(pos); bf != nil {
			for i, cit := range bf.Contents {
				if cit == item {
					bf.Contents = append(bf.Contents[:i], bf.Contents[i+1:]...)
					break
				}
			}
		}
	}
	if w.OnTileUpdate != nil {
		w.OnTileUpdate(pos)
	}
}

// StartDecayingMap is deliberately gone. It walked the entire map at boot and
// started decay on every item whose TYPE declared a duration, which upstream
// never does.
//
// C++ calls Item::startDecaying from a short list of places — house items
// restored from the DB (iomapserialize.cpp:175), a player's own items
// (iologindata_load_player.cpp), and an item the game moves (game.cpp:2595) —
// and never from the OTBM loader. Decay::startDecay then reads the DURATION
// attribute off the ITEM, not the default off its type, so a map item that was
// never explicitly started has no duration and is never scheduled.
//
// The old scan decayed the map out from under the startup scripts. Every slain
// skeleton (5972, duration 10, decayTo 4024) had already become a 4024 by the
// time the map-attribute loaders ran ~18 seconds after load, which is what
// produced 49 of these:
//
//	[loadLuaMapAction] - Wrong item id 5972 found
//
// and, worse, silently rewrote the map itself — grounds included.

// Players returns a snapshot of all online players.
func (w *World) Players() []*Player {
	w.mu.RLock()
	defer w.mu.RUnlock()
	out := make([]*Player, 0, len(w.players))
	for _, p := range w.players {
		out = append(out, p)
	}
	return out
}

// ChangeSpeed updates a creature's speed and broadcasts the change to spectators.
func (w *World) ChangeSpeed(c Creature, speedDelta int32) {
	c.ChangeSpeed(speedDelta)
	if w.OnChangeSpeed != nil {
		w.OnChangeSpeed(c)
	}
}

// Creatures returns a snapshot of all non-player creatures.
func (w *World) Creatures() []Creature {
	w.mu.RLock()
	defer w.mu.RUnlock()
	out := make([]Creature, 0, len(w.creatures))
	for _, c := range w.creatures {
		out = append(out, c)
	}
	return out
}

// RLock acquires the world's read lock.
func (w *World) RLock() {
	w.mu.RLock()
}

// RUnlock releases the world's read lock.
func (w *World) RUnlock() {
	w.mu.RUnlock()
}

// AddPlayer registers a player, assigns a creature id, applies defaults and
// places it on the map. Returns false if a character with the same name is
// GenerateCreatureID allocates a unique creature ID.
func (w *World) GenerateCreatureID() uint32 {
	return w.nextCreatureID.Add(1)
}

// already online.
func (w *World) AddPlayer(p *Player, sess Session) bool {
	w.mu.Lock()
	key := strings.ToLower(p.Name)
	if _, online := w.byName[key]; online {
		w.mu.Unlock()
		return false
	}
	p.ID = w.nextCreatureID.Add(1)
	p.World = w
	p.Session = sess
	p.ensureDefaults()
	w.players[p.ID] = p
	w.byName[key] = p
	w.addCreatureToTile(p)
	w.mu.Unlock()
	if w.OnCreatureAppear != nil {
		w.OnCreatureAppear(p)
	}
	return true
}

// RemovePlayer unregisters a player by creature id.
func (w *World) RemovePlayer(id uint32) {
	w.mu.Lock()
	if p, ok := w.players[id]; ok {
		delete(w.players, id)
		delete(w.byName, strings.ToLower(p.Name))
		oldStackPos := w.captureStackPositions(p.Pos, p)
		w.removeCreatureFromTile(p)
		w.mu.Unlock()
		if w.OnCreatureRemove != nil {
			w.OnCreatureRemove(p, oldStackPos)
		}
		return
	}
	w.mu.Unlock()
}

// PlayerByID returns an online player or nil (by creature ID).
func (w *World) PlayerByID(id uint32) *Player {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.players[id]
}

// PlayerByDBID returns an online player or nil (by DB ID).
func (w *World) PlayerByDBID(dbID uint32) *Player {
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, p := range w.players {
		if p != nil && p.DBID == dbID {
			return p
		}
	}
	return nil
}

// CreatureByID returns a creature or nil.
func (w *World) CreatureByID(id uint32) Creature {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if c, ok := w.creatures[id]; ok {
		return c
	}
	if p, ok := w.players[id]; ok {
		return p
	}
	return nil
}

// AddCreature adds a non-player creature to the world.
func (w *World) AddCreature(c Creature) {
	w.addCreature(c, false)
}

// AddCreatureAtStartup is Game::internalPlaceCreature with sendToSpectators
// false, the branch SpawnMonster::spawnMonster takes when startup is set
// (spawn_monster.cpp:216): "no need to send out events to the surrounding since
// there is no one out there to listen".
//
// The startup flag was threaded as far as ScheduleSpawn and then dropped, so
// booting the world broadcast an appear for all 1655 monsters to nobody.
func (w *World) AddCreatureAtStartup(c Creature) {
	w.addCreature(c, true)
}

func (w *World) addCreature(c Creature, startup bool) {
	w.mu.Lock()
	w.creatures[c.GetID()] = c
	// Every creature should know its world — the NPC needs it to resolve the
	// player it turns to when an interaction begins. Only Player got this
	// (AddPlayer); monsters and NPCs silently kept a nil World, so anything on
	// the creature that reached back through World was dead code.
	switch cc := c.(type) {
	case *Monster:
		cc.World = w
	case *Npc:
		cc.World = w
	}
	w.addCreatureToTile(c)
	w.mu.Unlock()
	if startup {
		// The creature is still notified about ITSELF — Monster::onCreatureAppear
		// builds its target list from that, and skipping it would leave every
		// boot-placed monster blind until its first sweep.
		switch self := c.(type) {
		case *Monster:
			self.OnCreatureAppear(w, self)
		case *Npc:
			self.OnPlacedCreature(w)
		}
		return
	}
	w.notifyCreatureAppear(c)
	if w.OnCreatureAppear != nil {
		w.OnCreatureAppear(c)
	}
}

func (w *World) addCreatureToTile(c Creature) {
	t := w.Map.GetTile(c.GetPosition())
	if t != nil {
		t.Creatures = append(t.Creatures, c)
	}
}

// removeCreatureFromTile takes c off its tile and reports whether it was found.
// It deliberately does NOT return the slice index: an index cannot be turned back
// into a client stack position afterwards (that needs the creatures which were
// above it, and each spectator's visibility), so callers snapshot the
// per-spectator stack positions with captureStackPositions first.
func (w *World) removeCreatureFromTile(c Creature) bool {
	t := w.Map.GetTile(c.GetPosition())
	if t != nil {
		for i, v := range t.Creatures {
			if v.GetID() == c.GetID() {
				t.Creatures = append(t.Creatures[:i], t.Creatures[i+1:]...)
				return true
			}
		}
	}
	return false
}

// RemoveCreature removes a non-player creature from the world.
func (w *World) RemoveCreature(id uint32) {
	w.mu.Lock()
	c, exists := w.creatures[id]
	var oldStackPos map[uint32]int
	if exists {
		delete(w.creatures, id)
		oldStackPos = w.captureStackPositions(c.GetPosition(), c)
		w.removeCreatureFromTile(c)
	}
	w.mu.Unlock()
	if exists {
		w.notifyCreatureRemove(c)
	}
	if exists && w.OnCreatureRemove != nil {
		w.OnCreatureRemove(c, oldStackPos)
	}
}

// OnlineCount returns the number of connected players.
func (w *World) OnlineCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.players)
}

// Spectators returns players (optionally excluding one id) whose client can see
// pos.
func (w *World) Spectators(pos Position, excludeID uint32) []*Player {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.SpectatorsLocked(pos, excludeID)
}

// SpectatorsLocked is Spectators without the lock, for callers that already hold
// w.mu (the stack-position capture runs inside the move critical section).
func (w *World) SpectatorsLocked(pos Position, excludeID uint32) []*Player {
	var out []*Player
	for id, p := range w.players {
		if id == excludeID {
			continue
		}
		if p.Pos.InRangeOf(pos) {
			out = append(out, p)
		}
	}
	return out
}

// captureStackPositions snapshots the per-spectator client stack index of c on its
// current tile. It must be called with w.mu held and BEFORE c is taken off the
// tile: the index depends on which creatures are above it and on what each
// spectator can see, neither of which is recoverable once it is gone.
func (w *World) captureStackPositions(pos Position, c Creature) map[uint32]int {
	if w.CaptureStackPositions == nil {
		return nil
	}
	return w.CaptureStackPositions(pos, c)
}

// collectSpectators invokes fn for every creature standing on a tile that pos can
// see, without allocating a result slice.
//
// The old implementation walked the whole w.creatures map — O(all creatures) per
// query. On the full OTServBR world that is ~86k entries, and the monster AI runs
// this per monster per think, so the single-threaded dispatcher spent all its
// time in those scans and never reached the NPC think loop (NPCs stayed frozen).
// Walking the viewport's tiles instead makes the cost proportional to the area
// actually visible.
func (w *World) collectSpectators(pos Position, fn func(c Creature)) {
	if fn == nil {
		return
	}
	w.mu.RLock()
	defer w.mu.RUnlock()

	px, py, pz := int(pos.X), int(pos.Y), int(pos.Z)
	z0, z1 := 0, MapInitSurfaceLayer
	if pz > MapInitSurfaceLayer {
		z0 = pz - MapLayerViewLimit
		z1 = pz + MapLayerViewLimit
	}
	for z := z0; z <= z1; z++ {
		if z < 0 || z > 255 {
			continue
		}
		// InRangeOf shifts the x/y window diagonally by the floor delta, because
		// that is how a tile one floor down is drawn. Mirror it here so the
		// rectangle enumerates exactly the positions InRangeOf accepts.
		off := pz - z
		w.Map.RangeRect(px-MapMaxViewPortX+off, py-MapMaxViewPortY+off,
			px+MapMaxViewPortX+off, py+MapMaxViewPortY+off, z, func(t *Tile) {
				for _, c := range t.Creatures {
					if c != nil {
						fn(c)
					}
				}
			})
	}
}

// SpectatorCreatures returns every creature that can see pos — the equivalent of
// Map::getSpectators with onlyPlayers off.
//
// Players and monsters are kept in two separate maps here, and this walked only
// w.creatures, so despite the name it never returned a player. The one caller
// filtered to NPCs and could not tell; the monster target list could, and saw an
// empty world.
func (w *World) SpectatorCreatures(pos Position) []Creature {
	var out []Creature
	w.collectSpectators(pos, func(c Creature) {
		if pos.InRangeOf(c.GetPosition()) {
			out = append(out, c)
		}
	})
	return out
}

// SpectatingNpcs returns NPCs whose AI can see pos.
func (w *World) SpectatingNpcs(pos Position) []*Npc {
	w.mu.RLock()
	defer w.mu.RUnlock()
	var out []*Npc
	for _, c := range w.creatures {
		if npc, ok := c.(*Npc); ok {
			if npc.Pos.InRangeOf(pos) {
				out = append(out, npc)
			}
		}
	}
	return out
}

// CreaturesAt returns every creature standing exactly on pos (players, monsters,
// NPCs). Used by the spell system to resolve area-combat targets.
func (w *World) CreaturesAt(pos Position) []Creature {
	w.mu.RLock()
	defer w.mu.RUnlock()
	var out []Creature
	for _, p := range w.players {
		if p.Pos == pos {
			out = append(out, p)
		}
	}
	for _, c := range w.creatures {
		if c.GetPosition() == pos {
			out = append(out, c)
		}
	}
	return out
}

// CreaturesInView returns all creatures (players, monsters, NPCs) within range of pos.
func (w *World) CreaturesInView(pos Position) []Creature {
	var out []Creature
	w.collectSpectators(pos, func(c Creature) {
		if pos.InRangeOf(c.GetPosition()) {
			out = append(out, c)
		}
	})
	return out
}

// TryMove validates and applies a directional step for a player, returning the new position
// and whether the move succeeded.
func (w *World) TryMove(p *Player, dir Direction) (Position, bool) {
	dest := p.Pos.Offset(dir)
	destTile := w.Map.GetTile(dest)
	if destTile == nil || !destTile.WalkableFor(p, w.Items, w.WorldType) {
		return p.Pos, false
	}
	dest = w.resolveFloorChangeDest(dest, destTile)
	w.mu.Lock()
	p.IsTraining = false
	oldPos := p.Pos
	oldStackPos := w.captureStackPositions(oldPos, p)
	w.removeCreatureFromTile(p)
	p.Pos = dest
	p.Direction = dir
	w.addCreatureToTile(p)
	w.mu.Unlock()

	if w.OnCreatureMove != nil {
		w.OnCreatureMove(p, oldPos, dest, oldStackPos)
	}

	return dest, true
}

// TeleportCreature relocates c to an arbitrary destination and notifies
// spectators via OnCreatureMove (BroadcastCreatureMove handles the far-move
// remove/appear so the client actually sees the jump). Unlike TryMove it does
// not require the destination to be adjacent. Used by scripted travel/teleport.
func (w *World) TeleportCreature(c Creature, dest Position) {
	w.mu.Lock()
	if player, ok := c.(*Player); ok {
		player.IsTraining = false
	}
	oldPos := c.GetPosition()
	oldStackPos := w.captureStackPositions(oldPos, c)
	w.removeCreatureFromTile(c)
	c.SetPosition(dest)
	w.addCreatureToTile(c)
	w.mu.Unlock()

	// TryMove notifies spectators at both ends of the move; a teleport jumped
	// straight to the world hook and skipped it. With the monster AI idle gate
	// relying on spectator events to wake monsters up, a creature that teleports
	// past an idle monster would otherwise never be noticed until it stepped.
	w.notifyCreatureMove(c, oldPos, dest)

	if w.OnCreatureMove != nil {
		w.OnCreatureMove(c, oldPos, dest, oldStackPos)
	}
}

// TryMoveCreature validates and applies a directional step for any creature, returning the new position
// and whether the move succeeded.
func (w *World) TryMoveCreature(c Creature, dir Direction) (Position, bool) {
	dest := c.GetPosition().Offset(dir)
	destTile := w.Map.GetTile(dest)
	if destTile == nil || !destTile.WalkableFor(c, w.Items, w.WorldType) {
		return c.GetPosition(), false
	}
	// Protection zones keep MONSTERS out, not every non-player. Tile::queryAdd
	// applies the TILESTATE_PROTECTIONZONE rejection inside its monster branch
	// (src/items/tile.cpp:664); Npc::canWalkTo has no PZ check at all. Blocking
	// NPCs here meant town NPCs, which stand in a PZ, could never take a step.
	if c.GetCreatureType() == 1 && destTile.IsProtectionZone() {
		return c.GetPosition(), false
	}
	dest = w.resolveFloorChangeDest(dest, destTile)
	w.mu.Lock()
	if player, ok := c.(*Player); ok {
		player.IsTraining = false
	}
	oldPos := c.GetPosition()
	oldStackPos := w.captureStackPositions(oldPos, c)
	w.removeCreatureFromTile(c)
	c.SetPosition(dest)
	c.SetDirection(dir)
	w.addCreatureToTile(c)
	w.mu.Unlock()

	w.notifyCreatureMove(c, oldPos, dest)
	if w.OnCreatureMove != nil {
		w.OnCreatureMove(c, oldPos, dest, oldStackPos)
	}

	return dest, true
}

func (w *World) resolveFloorChangeDest(dest Position, destTile *Tile) Position {
	if destTile == nil {
		return dest
	}
	floorChange := ""
	if destTile.Ground != nil {
		if ct := w.Items.Get(destTile.Ground.ID); ct != nil && ct.FloorChange != "" {
			floorChange = ct.FloorChange
		}
	}
	if floorChange == "" {
		for _, it := range destTile.Items {
			if ct := w.Items.Get(it.ID); ct != nil && ct.FloorChange != "" {
				floorChange = ct.FloorChange
				break
			}
		}
	}

	if floorChange != "" {
		dx, dy, dz := dest.X, dest.Y, dest.Z
		hasFC := func(t *Tile, expected string) bool {
			if t == nil {
				return false
			}
			if t.Ground != nil {
				if ct := w.Items.Get(t.Ground.ID); ct != nil && ct.FloorChange == expected {
					return true
				}
			}
			for _, it := range t.Items {
				if ct := w.Items.Get(it.ID); ct != nil && ct.FloorChange == expected {
					return true
				}
			}
			return false
		}
		if floorChange == "down" {
			dz++
			if hasFC(w.Map.GetTile(Position{X: dx, Y: dy - 1, Z: dz}), "southalt") {
				dy -= 2
			} else if hasFC(w.Map.GetTile(Position{X: dx - 1, Y: dy, Z: dz}), "eastalt") {
				dx -= 2
			} else if downTile := w.Map.GetTile(Position{X: dx, Y: dy, Z: dz}); downTile != nil {
				if hasFC(downTile, "north") {
					dy++
				}
				if hasFC(downTile, "south") {
					dy--
				}
				if hasFC(downTile, "southalt") {
					dy -= 2
				}
				if hasFC(downTile, "east") {
					dx--
				}
				if hasFC(downTile, "eastalt") {
					dx -= 2
				}
				if hasFC(downTile, "west") {
					dx++
				}
			}
		} else {
			dz--
			if floorChange == "north" {
				dy--
			}
			if floorChange == "south" {
				dy++
			}
			if floorChange == "southalt" {
				dy += 2
			}
			if floorChange == "east" {
				dx++
			}
			if floorChange == "eastalt" {
				dx += 2
			}
			if floorChange == "west" {
				dx--
			}
		}
		return Position{X: dx, Y: dy, Z: dz}
	}
	return dest
}

// TransformItem changes an item's ID and notifies the clients.
func (w *World) TransformItem(pos Position, item *Item, newID uint16) {
	tile := w.Map.GetTile(pos)
	if tile == nil {
		item.ID = newID
		return
	}

	stackPos := uint8(255)
	if tile.Ground == item {
		stackPos = 0
	} else {
		for i, it := range tile.Items {
			if it == item {
				stackPos = uint8(1 + i)
				break
			}
		}
	}

	oldItem := &Item{ID: item.ID}
	item.ID = newID

	if stackPos != 255 && w.OnItemDecay != nil {
		w.OnItemDecay(pos, stackPos, oldItem, item)
	}
}

func (w *World) GetBoostedCreature() string {
	w.mu.RLock()
	bc := w.BoostedCreature
	w.mu.RUnlock()
	if bc != "" && bc != "default" {
		return bc
	}
	if w.Monsters != nil {
		w.mu.Lock()
		defer w.mu.Unlock()
		if w.BoostedCreature != "" && w.BoostedCreature != "default" {
			return w.BoostedCreature
		}
		if m, ok := w.Monsters.Monsters["dragon"]; ok {
			w.BoostedCreature = m.Name
			return w.BoostedCreature
		}
		for _, m := range w.Monsters.Monsters {
			if m.Name != "" {
				w.BoostedCreature = m.Name
				return w.BoostedCreature
			}
		}
	}
	return "Dragon"
}

func (w *World) GetBoostedBoss() string {
	w.mu.RLock()
	bb := w.BoostedBoss
	w.mu.RUnlock()
	if bb != "" && bb != "default" {
		return bb
	}
	return "None"
}

// EnsureBoostedBoss picks the daily bosstiary boosted boss (a random Archfoe)
// when none is configured, seeded by the calendar day so it is stable within a
// day and rotates daily. Simplified IOBosstiary::loadBoostedBoss (no DB row /
// outfit). Idempotent and safe for concurrent callers.
func (w *World) EnsureBoostedBoss() {
	w.mu.RLock()
	bb, reg := w.BoostedBoss, w.TypeRegistry
	w.mu.RUnlock()
	if (bb != "" && bb != "default") || reg == nil {
		return
	}
	var names []string
	for _, mt := range reg.Monsters {
		if mt.IsBoss() && mt.BosstiaryRace == bosstiary.RarityArchfoe {
			names = append(names, mt.Name)
		}
	}
	if len(names) == 0 {
		return
	}
	sort.Strings(names)
	day := int(time.Now().Unix() / 86400)
	pick := names[day%len(names)]
	w.mu.Lock()
	if w.BoostedBoss == "" || w.BoostedBoss == "default" {
		w.BoostedBoss = pick
	}
	w.mu.Unlock()
}

func (w *World) PlayerAnswerModalWindow(p *Player, id uint32, button uint8, choice uint8) {
}

// Team Finder stubs
func (w *World) PlayerFindTeam(playerID uint32, category uint8, itemID uint16, stackPos uint8, isLoot bool) {
}
func (w *World) RemoveTeamFinder(playerID uint32)                                  {}
func (w *World) UpdateTeamMemberStatus(playerID, memberID uint32, status byte)     {}
func (w *World) SendTeamFinderList(playerID uint32)                                {}
func (w *World) JoinTeamFinder(playerID, leaderID uint32)                          {}
func (w *World) LeaveTeamFinder(playerID, leaderID uint32)                         {}
func (w *World) CreateTeamFinder(playerID uint32, r interface{ ReadU16() uint16 }) {}
func (w *World) PlayerSetVocation(playerID uint32, voc uint8) {
	if p := w.PlayerByID(playerID); p != nil {
		p.Vocation = uint16(voc)
	}
}
func (w *World) PlayerTeleport(playerID uint32, pos Position) {
	if p := w.PlayerByID(playerID); p != nil {
		p.Pos = pos
	}
}
func (w *World) PlayerCloseNpcChannel(playerID uint32)                                        {}
func (w *World) PlayerRotateItem(playerID uint32, pos Position, itemID uint16, stackPos byte) {}
func (w *World) PlayerExivaRestrictions(playerID uint32, action byte, name string)            {}
func (w *World) PlayerBrowseField(playerID uint32, pos Position)                              {}
func (w *World) PlayerSetBossDifficulty(playerID uint32, bossID uint16, difficulty byte)      {}
func (w *World) PlayerCollectReward(playerID uint32)                                          {}
func (w *World) PlayerJoinAggression(playerID, targetID uint32)                               {}
func (w *World) PlayerRequestTrade(playerID, targetID uint32, pos Position, itemID uint16, stackPos byte) {
}
func (w *World) PlayerAcceptTrade(playerID uint32)      {}
func (w *World) PlayerCloseTrade(playerID uint32)       {}
func (w *World) PlayerFollow(playerID, targetID uint32) {}

// RemoveItemFromHolder removes count from an item wherever it actually lives —
// a container, a player's inventory slot, or nowhere at all — and reports whether
// it found a holder.
//
// Item::remove goes through Game::internalRemoveItem, which asks the item for its
// parent cylinder and removes it from THAT. The Lua binding only knew how to
// remove from a tile and, for anything else, set Count to 0 and left the object
// in place: a mystic bag in a backpack stayed there after item:remove(1), fully
// usable, so it handed out a prize on every click.
func (w *World) RemoveItemFromHolder(p *Player, item *Item, count uint16) bool {
	if item == nil {
		return false
	}
	partial := int(item.Count) > int(count) && count > 0

	// A container the item sits in, which the item itself points at.
	if parent := item.Parent; parent != nil {
		for i, c := range parent.Contents {
			if c != item {
				continue
			}
			if partial {
				item.Count -= count
			} else {
				parent.Contents = append(parent.Contents[:i], parent.Contents[i+1:]...)
				item.Parent = nil
			}
			return true
		}
	}

	// An inventory slot holds the item directly, with no parent container.
	if p != nil {
		for slot, inv := range p.Inventory {
			if inv != item {
				continue
			}
			if partial {
				item.Count -= count
			} else {
				p.Inventory[slot] = nil
			}
			return true
		}
	}
	return false
}
