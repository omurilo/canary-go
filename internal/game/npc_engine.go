package game

import (
	"math/rand"
	"time"
)

// npcThinkInterval is how often the NPC loop runs. C++ drives Npc::onThink from
// Game::checkCreatures at EVENT_CREATURE_THINK_INTERVAL; the tick accumulators
// below are fed this value, so walkInterval and voices.interval keep their
// datapack meaning in milliseconds regardless of the cadence chosen here.
const npcThinkInterval = 500 * time.Millisecond

// NpcEngine drives Npc::onThink for every NPC in the world: it dispatches the
// Lua onThink callback and then handles idle walking and voices.
//
// Before this existed the NpcType onThink callback was stored but never called,
// which meant npcHandler:onThink never ran — so the FocusModule lifecycle
// (greeting timeout, farewell when the player walks away) was dead, NPCs never
// walked, and voices never fired.
type NpcEngine struct {
	world *World

	// OnNpcThink dispatches the Lua onThink(npc, interval) callback. Set by the
	// Lua engine, following the same indirection as World.OnCreatureSay so the
	// game package does not import luaengine.
	OnNpcThink func(npc *Npc, interval uint32)

	// Say emits creature speech; wired to World.OnCreatureSay.
	Say func(npc *Npc, talkType byte, text string)
}

// Talk types used by onThinkYell (TALKTYPE_SAY / TALKTYPE_YELL).
const (
	talkTypeSay  byte = 1
	talkTypeYell byte = 3
)

// NewNpcEngine builds the engine.
func NewNpcEngine(w *World) *NpcEngine {
	return &NpcEngine{world: w}
}

// Start schedules the loop on the dispatcher.
func (e *NpcEngine) Start() {
	GlobalDispatcher.AddEvent(npcThinkInterval, e.tick)
}

func (e *NpcEngine) tick() {
	// Re-arm first so a panic inside the body cannot silently kill the loop; the
	// dispatcher's recover() would otherwise swallow it after this point.
	defer GlobalDispatcher.AddEvent(npcThinkInterval, e.tick)

	if e.world == nil {
		return
	}
	const interval = uint32(npcThinkInterval / time.Millisecond)

	for _, npc := range e.world.Npcs() {
		e.thinkNpc(npc, interval)
	}
}

// thinkNpc ports Npc::onThink (npc.cpp:707).
func (e *NpcEngine) thinkNpc(npc *Npc, interval uint32) {
	if npc == nil {
		return
	}

	// The Lua callback runs first and unconditionally, as in C++.
	if e.OnNpcThink != nil {
		e.OnNpcThink(npc, interval)
	}

	// Teleport home when the NPC has drifted out of its spawn range, resetting
	// conversations — Npc::onThink does this via internalTeleport +
	// resetPlayerInteractions.
	if !npc.isInSpawnRange(npc.GetPosition()) {
		npc.SetPosition(npc.MasterPos)
		npc.resetPlayerInteractions()
	}

	// Yell, walk and sound only run while a player can see the NPC
	// (`if (!playerSpectators.empty())`). Skipping it when nobody is watching is
	// both upstream behaviour and what keeps this loop cheap on a full map.
	if !e.hasPlayerSpectator(npc) {
		return
	}

	e.thinkYell(npc, interval)
	e.thinkWalk(npc, interval)
}

// thinkYell ports Npc::onThinkYell (npc.cpp:1046).
func (e *NpcEngine) thinkYell(npc *Npc, interval uint32) {
	if npc.Type == nil || npc.Type.YellInterval == 0 || len(npc.Type.Voices) == 0 {
		return
	}

	npc.yellTicks += interval
	if npc.yellTicks < npc.Type.YellInterval {
		return
	}
	npc.yellTicks = 0

	// yellChance is a percentage rolled against uniform_random(1, 100).
	if npc.Type.YellChance < uint32(rand.Intn(100)+1) {
		return
	}

	voice := npc.Type.Voices[rand.Intn(len(npc.Type.Voices))]
	talkType := talkTypeSay
	if voice.Yell {
		talkType = talkTypeYell
	}
	if e.Say != nil {
		e.Say(npc, talkType, voice.Text)
	}
}

// thinkWalk ports Npc::onThinkWalk (npc.cpp:1068).
func (e *NpcEngine) thinkWalk(npc *Npc, interval uint32) {
	if npc.Type == nil || npc.Type.WalkInterval == 0 || npc.Speed == 0 {
		return
	}

	// An NPC mid-conversation does not walk, and its walk timer resets so it does
	// not step the instant the conversation ends.
	if len(npc.interactions) > 0 {
		npc.walkTicks = 0
		return
	}

	npc.walkTicks += interval
	if npc.walkTicks < npc.Type.WalkInterval {
		return
	}
	npc.walkTicks = 0

	if dir, ok := e.randomStep(npc); ok {
		e.world.TryMoveCreature(npc, dir)
	}
}

// randomStep ports Npc::getRandomStep (npc.cpp:1207) together with the checks in
// Npc::canWalkTo (npc.cpp:1177): shuffle the four cardinal directions and take the
// first the NPC may enter.
func (e *NpcEngine) randomStep(npc *Npc) (Direction, bool) {
	// canWalkTo returns false outright when walkRadius is 0 — that means "does not
	// walk", not "walks anywhere".
	if npc.Type == nil || npc.Type.WalkRadius == 0 {
		return DirNorth, false
	}

	dirs := []Direction{DirNorth, DirWest, DirEast, DirSouth}
	rand.Shuffle(len(dirs), func(i, j int) { dirs[i], dirs[j] = dirs[j], dirs[i] })

	from := npc.GetPosition()
	for _, dir := range dirs {
		dest := from.Offset(dir)
		if !npc.isInSpawnRange(dest) {
			continue
		}
		tile := e.world.Map.GetTile(dest)
		if tile == nil || !tile.WalkableFor(npc, e.world.Items, e.world.WorldType) {
			continue
		}
		// !floorChange && (TILESTATE_FLOORCHANGE || teleport) blocks the step, so an
		// NPC without the flag never wanders onto stairs or a teleport.
		if !npc.Type.FloorChange && e.isFloorChangeTile(tile) {
			continue
		}
		return dir, true
	}
	return DirNorth, false
}

// isFloorChangeTile reports whether the tile carries a floor-change or teleport,
// using the same ItemType.FloorChange signal as resolveFloorChangeDest. Go does not
// map TILESTATE_FLOORCHANGE off the raw OTBM tile flags yet, so this reads the
// items instead.
func (e *NpcEngine) isFloorChangeTile(tile *Tile) bool {
	if tile == nil || e.world.Items == nil {
		return false
	}
	if tile.Ground != nil {
		if ct := e.world.Items.Get(tile.Ground.ID); ct != nil && ct.FloorChange != "" {
			return true
		}
	}
	for _, it := range tile.Items {
		if ct := e.world.Items.Get(it.ID); ct != nil && ct.FloorChange != "" {
			return true
		}
	}
	// C++ also rejects a tile holding a teleport item (getTeleportItem()). Go has no
	// accessor for the teleport destination attribute yet, so that half is not
	// covered — an NPC with floorchange disabled can still step onto a teleport.
	return false
}

// Npcs returns every NPC currently in the world.
func (w *World) Npcs() []*Npc {
	var out []*Npc
	for _, c := range w.Creatures() {
		if npc, ok := c.(*Npc); ok {
			out = append(out, npc)
		}
	}
	return out
}

// hasPlayerSpectator reports whether any player can see the NPC, standing in for
// Npc::playerSpectators.
func (e *NpcEngine) hasPlayerSpectator(npc *Npc) bool {
	return len(e.world.Spectators(npc.GetPosition(), npc.GetID())) > 0
}

// isInSpawnRange mirrors Npc::isInSpawnRange (npc.cpp:1120): a radius of 0 means
// unrestricted, as does an unset master position.
func (n *Npc) isInSpawnRange(pos Position) bool {
	if n.Type == nil || n.Type.WalkRadius <= 0 {
		return true
	}
	if n.MasterPos.X == 0 && n.MasterPos.Y == 0 {
		return true
	}
	if pos.Z != n.MasterPos.Z {
		return false
	}
	radius := int(n.Type.WalkRadius)
	return absInt(int(pos.X)-int(n.MasterPos.X)) <= radius &&
		absInt(int(pos.Y)-int(n.MasterPos.Y)) <= radius
}

// resetPlayerInteractions ends every conversation, as Npc::resetPlayerInteractions
// does when the NPC is teleported home.
func (n *Npc) resetPlayerInteractions() {
	n.interactions = nil
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
