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
//
// The tick uses its own lane rather than AddEvent (LaneGenericParallel). At boot
// the spawn engine floods that lane with ~84k placement tasks, and every one of
// them is queued with an earlier deadline than the tick, so an AddEvent tick
// would not fire until the whole map had been spawned — minutes of frozen NPCs.
// On a dedicated lane the 500ms tick is dispatched each scheduler cycle no
// matter how long the spawn backlog takes.
func (e *NpcEngine) Start() {
	GlobalDispatcher.AddTask(LaneNpcThink, npcThinkInterval, 0, e.tick)
}

func (e *NpcEngine) tick() {
	// Re-arm first so a panic inside the body cannot silently kill the loop; the
	// dispatcher's recover() would otherwise swallow it after this point.
	defer GlobalDispatcher.AddTask(LaneNpcThink, npcThinkInterval, 0, e.tick)

	if e.world == nil {
		return
	}
	const interval = uint32(npcThinkInterval / time.Millisecond)

	for _, npc := range e.world.Npcs() {
		e.thinkNpc(npc, interval)
	}
}

// thinkNpc runs one NPC.s think tick. The script callback is the engine.s to
// make — it is the only part of Npc::onThink that needs the Lua bridge — and
// everything after it is Npc::OnThink.
func (e *NpcEngine) thinkNpc(npc *Npc, interval uint32) {
	if npc == nil {
		return
	}

	// The Lua callback runs first and unconditionally, as in C++.
	if e.OnNpcThink != nil {
		e.OnNpcThink(npc, interval)
	}
	npc.OnThink(e.world, interval)
}

// OnThink is Npc::onThink (npc.cpp:707), minus the script callback the engine
// fires for it.
func (npc *Npc) OnThink(w *World, interval uint32) {
	// Who can see the NPC is recomputed here rather than maintained from
	// spectator events, because the port has no creature-movement hook to drive
	// OnPlayerAppear/OnPlayerDisappear from yet. The resulting set is the same.
	npc.LoadPlayerSpectators(w)

	// Teleport home when the NPC has drifted out of its spawn range. Upstream
	// also closes every open shop window here: a trade window survives the
	// teleport otherwise, and the player keeps buying from across the map.
	if !npc.isInSpawnRange(npc.GetPosition()) {
		npc.SetPosition(npc.MasterPos)
		npc.resetPlayerInteractions()
		npc.CloseAllShopWindows(w)
	}

	// Yell, walk and sound only run while a player can see the NPC
	// (`if (!playerSpectators.empty())`). Skipping it when nobody is watching is
	// both upstream behaviour and what keeps this loop cheap on a full map.
	if npc.IsIdle() {
		return
	}

	npc.OnThinkYell(w, interval)
	npc.OnThinkWalk(w, interval)
	npc.OnThinkSound(w, interval)
}

// OnThinkYell is Npc::onThinkYell (npc.cpp:1046).
func (npc *Npc) OnThinkYell(w *World, interval uint32) {
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
	if w != nil && w.OnCreatureSay != nil {
		w.OnCreatureSay(npc, talkType, voice.Text)
	}
}

// OnThinkWalk is Npc::onThinkWalk (npc.cpp:1068).
func (npc *Npc) OnThinkWalk(w *World, interval uint32) {
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

	if dir, ok := npc.GetRandomStep(w); ok {
		w.TryMoveCreature(npc, dir)
		npc.OnCreatureWalk()
	}
}

// randomStep ports Npc::getRandomStep (npc.cpp:1207) together with the checks in
// Npc::canWalkTo (npc.cpp:1177): shuffle the four cardinal directions and take the
// first the NPC may enter.
func (npc *Npc) GetRandomStep(w *World) (Direction, bool) {
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
		tile := w.Map.GetTile(dest)
		if tile == nil || !tile.WalkableFor(npc, w.Items, w.WorldType) {
			continue
		}
		// !floorChange && (TILESTATE_FLOORCHANGE || teleport) blocks the step, so an
		// NPC without the flag never wanders onto stairs or a teleport.
		if !npc.Type.FloorChange && w.isFloorChangeTile(tile) {
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
func (w *World) isFloorChangeTile(tile *Tile) bool {
	if tile == nil || w.Items == nil {
		return false
	}
	if tile.Ground != nil {
		if ct := w.Items.Get(tile.Ground.ID); ct != nil && ct.FloorChange != "" {
			return true
		}
	}
	for _, it := range tile.Items {
		if ct := w.Items.Get(it.ID); ct != nil && ct.FloorChange != "" {
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

// OnThinkSound is Npc::onThinkSound (npc.cpp:691), the audio counterpart of
// onThinkYell. It shares nothing with the yell timer: an NPC can be silent and
// still make ambient noise, and both counters run independently.
func (npc *Npc) OnThinkSound(w *World, interval uint32) {
	if npc.Type == nil || npc.Type.SoundSpeedTicks == 0 {
		return
	}
	npc.soundTicks += interval
	if npc.soundTicks < npc.Type.SoundSpeedTicks {
		return
	}
	npc.soundTicks = 0

	if len(npc.Type.Sounds) == 0 || npc.Type.SoundChance < uint32(rand.Intn(100)+1) {
		return
	}
	if w != nil && w.OnSoundEffect != nil {
		w.OnSoundEffect(npc.GetPosition(), npc.Type.Sounds[rand.Intn(len(npc.Type.Sounds))])
	}
}

// CanWalkTo is Npc::canWalkTo (npc.cpp:1177), the per-tile test getRandomStep
// runs. Kept as its own method because the shop and script layers ask it too.
func (npc *Npc) CanWalkTo(w *World, from Position, dir Direction) bool {
	if w == nil || npc.Type == nil {
		return false
	}
	dest := from.Offset(dir)
	if !npc.isInSpawnRange(dest) {
		return false
	}
	tile := w.Map.GetTile(dest)
	if tile == nil || !tile.WalkableFor(npc, w.Items, w.WorldType) {
		return false
	}
	return npc.Type.FloorChange || !w.isFloorChangeTile(tile)
}

// GetNextStep is Npc::getNextStep (npc.cpp:1203). An NPC has no target to
// follow, so its only source of movement is the wander step.
func (npc *Npc) GetNextStep(w *World) (Direction, bool) {
	if npc.IsIdle() {
		return DirNorth, false
	}
	return npc.GetRandomStep(w)
}
