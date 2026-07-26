package game

import "sync"

// WheelOfDestiny ports the Tibia 13.x Wheel of Destiny. The slot layout, the
// per-slot / per-vocation stat table, the point budget, the save validation
// (per-slot cap + total budget + adjacency tree) and the bonus recalculation
// mirror the C++ implementation in:
//   - src/creatures/players/components/wheel/player_wheel.cpp (points/validation)
//   - src/io/io_wheel.cpp (slot -> bonus mapping)
//
// Not yet modeled: gems/vessels, revelation grade modifiers, promotion scrolls,
// the monk quest bonus, and the wheel spells/instants (the instant unlock flags
// are computed and stored but are inert until the spell system lands).

// CIP client vocation ids (creatures_definitions.hpp). The wheel keys all its
// per-vocation stats off these, not the OT vocation id.
const (
	cipNone     uint8 = 0
	cipKnight   uint8 = 1
	cipPaladin  uint8 = 2
	cipSorcerer uint8 = 3
	cipDruid    uint8 = 4
	cipMonk     uint8 = 5
)

// CIPVocation maps an OT vocation id (1=Sorc,2=Druid,3=Paladin,4=Knight and the
// promoted 5..8, plus 9/10 monk) to its CIP client vocation id.
func CIPVocation(vocation uint16) uint8 {
	switch vocation {
	case 1, 5:
		return cipSorcerer
	case 2, 6:
		return cipDruid
	case 3, 7:
		return cipPaladin
	case 4, 8:
		return cipKnight
	case 9, 10:
		return cipMonk
	default:
		return cipNone
	}
}

// WheelSlotCount is the number of wheel slots (WheelSlots_t, 1..36).
const WheelSlotCount = 36

// Slot ids (WheelSlots_t). Kept for readability in the tables below.
const (
	slotGreen200        = 1
	slotGreenTop150     = 2
	slotGreenTop100     = 3
	slotRedTop100       = 4
	slotRedTop150       = 5
	slotRed200          = 6
	slotGreenBottom150  = 7
	slotGreenMiddle100  = 8
	slotGreenTop75      = 9
	slotRedTop75        = 10
	slotRedMiddle100    = 11
	slotRedBottom150    = 12
	slotGreenBottom100  = 13
	slotGreenBottom75   = 14
	slotGreen50         = 15
	slotRed50           = 16
	slotRedBottom75     = 17
	slotRedBottom100    = 18
	slotBlueTop100      = 19
	slotBlueTop75       = 20
	slotBlue50          = 21
	slotPurple50        = 22
	slotPurpleTop75     = 23
	slotPurpleTop100    = 24
	slotBlueTop150      = 25
	slotBlueMiddle100   = 26
	slotBlueBottom75    = 27
	slotPurpleBottom75  = 28
	slotPurpleMiddle100 = 29
	slotPurpleTop150    = 30
	slotBlue200         = 31
	slotBlueBottom150   = 32
	slotBlueBottom100   = 33
	slotPurpleBottom100 = 34
	slotPurpleBottom150 = 35
	slotPurple200       = 36
)

// Per-slot stat increase constants (io_wheel.cpp #defines).
const (
	wheelMitigationIncrease  = 0.03
	wheelManaLeechIncrease   = 0.25
	wheelHealthLeechIncrease = 0.75
)

// wheelMaxPointsPerSlot returns the point cap for a slot (from its tier suffix),
// mirroring PlayerWheel::getMaxPointsPerSlot.
func wheelMaxPointsPerSlot(slot uint8) uint16 {
	switch slot {
	case slotGreen50, slotRed50, slotBlue50, slotPurple50:
		return 50
	case slotGreenTop75, slotGreenBottom75, slotRedTop75, slotRedBottom75,
		slotPurpleTop75, slotPurpleBottom75, slotBlueTop75, slotBlueBottom75:
		return 75
	case slotGreenTop100, slotGreenMiddle100, slotGreenBottom100, slotRedTop100,
		slotRedMiddle100, slotRedBottom100, slotPurpleTop100, slotPurpleMiddle100,
		slotPurpleBottom100, slotBlueTop100, slotBlueMiddle100, slotBlueBottom100:
		return 100
	case slotGreenTop150, slotGreenBottom150, slotRedTop150, slotRedBottom150,
		slotPurpleTop150, slotPurpleBottom150, slotBlueTop150, slotBlueBottom150:
		return 150
	case slotGreen200, slotRed200, slotPurple200, slotBlue200:
		return 200
	}
	return 0
}

// wheelSlotMinPlayerPoints is the minimum lifetime wheel points a player must
// have earned before a slot of that tier may hold any points, mirroring the
// `playerPoints < X` gates in canPlayerSelectPointOnSlot.
func wheelSlotMinPlayerPoints(slot uint8) uint16 {
	switch wheelMaxPointsPerSlot(slot) {
	case 200:
		return 375
	case 150:
		return 225
	case 100:
		return 125
	case 75:
		return 50
	default: // 50 (rim) has no gate
		return 0
	}
}

// wheelSlotNeighbors is the adjacency graph transcribed from
// canPlayerSelectPointOnSlot: a non-rim slot becomes selectable only once one of
// its listed neighbors is fully filled. Rim (_50) slots have no neighbors and
// are always selectable.
var wheelSlotNeighbors = map[uint8][]uint8{
	slotGreen200:       {slotGreenTop150, slotGreenBottom150},
	slotGreenTop150:    {slotGreenTop100, slotGreenMiddle100, slotGreenBottom150},
	slotGreenBottom150: {slotGreenMiddle100, slotGreenBottom100, slotGreenTop150},
	slotGreenTop100:    {slotRedTop100, slotGreenTop75, slotGreenMiddle100},
	slotGreenMiddle100: {slotGreenTop100, slotGreenTop75, slotGreenBottom75, slotGreenBottom100},
	slotGreenBottom100: {slotGreenMiddle100, slotGreenBottom75, slotBlueTop100, slotGreenBottom150},
	slotGreenTop75:     {slotGreen50, slotRedTop75, slotGreenBottom75, slotGreenMiddle100, slotGreenTop100},
	slotGreenBottom75:  {slotGreen50, slotBlueTop75, slotGreenMiddle100, slotGreenBottom100, slotGreenTop75},
	slotGreen50:        nil,

	slotRed200:       {slotRedTop150, slotRedBottom150},
	slotRedTop150:    {slotRedTop100, slotRedMiddle100, slotRedBottom150},
	slotRedBottom150: {slotRedMiddle100, slotRedBottom100, slotRedTop150},
	slotRedTop100:    {slotRedTop75, slotGreenTop100, slotRedMiddle100, slotRedTop150},
	slotRedMiddle100: {slotRedTop75, slotRedBottom75, slotRedBottom100, slotRedTop100},
	slotRedBottom100: {slotRedBottom75, slotPurpleTop100, slotRedMiddle100, slotRedBottom150},
	slotRedTop75:     {slotRed50, slotGreenTop75, slotRedTop100, slotRedMiddle100, slotRedBottom75},
	slotRedBottom75:  {slotRed50, slotPurpleTop75, slotRedBottom100, slotRedMiddle100, slotRedTop75},
	slotRed50:        nil,

	slotPurple200:       {slotPurpleTop150, slotPurpleBottom150},
	slotPurpleTop150:    {slotPurpleMiddle100, slotPurpleTop100, slotPurpleBottom150},
	slotPurpleBottom150: {slotPurpleMiddle100, slotPurpleBottom100, slotPurpleTop150},
	slotPurpleTop100:    {slotPurpleTop75, slotRedBottom100, slotPurpleMiddle100, slotPurpleTop150},
	slotPurpleMiddle100: {slotPurpleTop75, slotPurpleBottom75, slotPurpleBottom100, slotPurpleTop100},
	slotPurpleBottom100: {slotPurpleBottom75, slotBlueBottom100, slotPurpleMiddle100, slotPurpleBottom150},
	slotPurpleTop75:     {slotPurple50, slotRedBottom75, slotPurpleBottom75, slotPurpleMiddle100, slotPurpleTop100},
	slotPurpleBottom75:  {slotPurple50, slotBlueBottom75, slotPurpleTop75, slotPurpleMiddle100, slotPurpleBottom100},
	slotPurple50:        nil,

	slotBlue200:       {slotBlueTop150, slotBlueBottom150},
	slotBlueTop150:    {slotBlueMiddle100, slotBlueTop100, slotBlueBottom150},
	slotBlueBottom150: {slotBlueMiddle100, slotBlueBottom100, slotBlueTop150},
	slotBlueTop100:    {slotBlueTop75, slotGreenBottom100, slotBlueMiddle100, slotBlueTop150},
	slotBlueMiddle100: {slotBlueTop75, slotBlueBottom75, slotBlueBottom100, slotBlueTop100},
	slotBlueBottom100: {slotBlueBottom75, slotPurpleBottom100, slotBlueMiddle100, slotBlueBottom150},
	slotBlueTop75:     {slotBlue50, slotGreenBottom75, slotBlueBottom75, slotBlueMiddle100, slotBlueTop100},
	slotBlueBottom75:  {slotBlue50, slotPurpleBottom75, slotBlueTop75, slotBlueMiddle100, slotBlueBottom100},
	slotBlue50:        nil,
}

// WheelBonusData holds the aggregated stat bonuses from all invested slots,
// mirroring PlayerWheelMethodsBonusData (the modeled subset).
type WheelBonusData struct {
	Health   uint32
	Mana     uint32
	Capacity uint32 // raw (applied as ×100 to the player, matching C++)
	Damage   uint32
	Healing  uint32

	Mitigation float64 // percent
	LifeLeech  float64 // percent
	ManaLeech  float64 // percent

	Fist     int
	Melee    int
	Distance int
	Magic    int

	Instants map[string]bool // unlocked wheel instants (inert until spells land)
}

// WheelGem tracks a single gem in a wheel slot.
type WheelGem struct {
	Slot        uint16 // 1..36
	Domain      uint8  // 0=combat, 1=defense, 2=healing, 3=support
	Grade       uint8  // 0..6
	Locked      bool
	Revealed    bool
}

// WheelOfDestiny models the character progression tree.
type WheelOfDestiny struct {
	mu           sync.RWMutex
	BonusPoints  uint16            // extra points from quests/scrolls (gated ≥ level 51 in C++)
	ActivePreset uint8             // current preset (0-2)
	SlotPoints   map[uint16]uint16 // slot id (1..36) -> allocated points

	Gems         []WheelGem  // gems installed in slots
	RevealedGems int         // total revealed gem count

	// Fragment resources for gem enhance.
	LesserFragments   uint16
	RegularFragments  uint16
	GreaterFragments  uint16

	cip   uint8          // cached CIP vocation, drives the per-vocation bonuses
	bonus WheelBonusData // cached, recomputed lazily
	dirty bool
}

// NewWheelOfDestiny returns an empty wheel.
func NewWheelOfDestiny() *WheelOfDestiny {
	return &WheelOfDestiny{SlotPoints: make(map[uint16]uint16), dirty: true}
}

// SetVocation sets the CIP vocation used to compute per-vocation bonuses.
func (w *WheelOfDestiny) SetVocation(cip uint8) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cip != cip {
		w.cip = cip
		w.dirty = true
	}
}

// GetTotalPoints returns the lifetime wheel points available at a level: one per
// level above 50 (m_pointsPerLevel default 1) plus extra points. Mirrors
// PlayerWheel::getWheelPoints.
func (w *WheelOfDestiny) GetTotalPoints(level uint16) uint16 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.totalPointsLocked(level)
}

func (w *WheelOfDestiny) totalPointsLocked(level uint16) uint16 {
	var levelPoints uint16
	if level > 50 {
		levelPoints = level - 50
	}
	return levelPoints + w.BonusPoints
}

func (w *WheelOfDestiny) getSpentPointsLocked() uint16 {
	var spent uint16
	for _, pts := range w.SlotPoints {
		spent += pts
	}
	return spent
}

// GetSpentPoints returns the total points invested across all slots.
func (w *WheelOfDestiny) GetSpentPoints() uint16 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.getSpentPointsLocked()
}

// SaveSlotPoints stores an allocation verbatim WITHOUT validation. Use only for
// trusted sources (DB load). The client path must use ValidateAndSave.
func (w *WheelOfDestiny) SaveSlotPoints(points map[uint16]uint16) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.SlotPoints = make(map[uint16]uint16)
	for slot, pts := range points {
		if pts > 0 {
			w.SlotPoints[slot] = pts
		}
	}
	w.dirty = true
}

// ValidateAndSave applies a client-submitted allocation, enforcing the three C++
// checks: (1) per-slot cap, (2) total point budget, (3) the adjacency tree
// (a non-rim slot needs a full neighbor). It mirrors
// PlayerWheel::saveSlotPointsOnPressSaveButton: slots are processed rim-outward
// with a retry loop, and only the slots that pass are applied. Returns false if
// the raw packet was invalid (any slot over its cap), in which case nothing is
// applied.
func (w *WheelOfDestiny) ValidateAndSave(points map[uint16]uint16, playerPoints uint16) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	// (1) Any slot exceeding its cap fails the whole save (as in C++).
	for slot, pts := range points {
		if slot < 1 || slot > WheelSlotCount {
			return false
		}
		if pts > wheelMaxPointsPerSlot(uint8(slot)) {
			return false
		}
	}

	// Build the work list ordered rim-outward (50, 75, 100, 150, 200) so a
	// slot's neighbor is already applied before it is evaluated.
	type entry struct {
		slot   uint8
		points uint16
	}
	var pending []entry
	for slot, pts := range points {
		if pts > 0 {
			pending = append(pending, entry{uint8(slot), pts})
		}
	}
	orderOf := func(slot uint8) int {
		switch wheelMaxPointsPerSlot(slot) {
		case 50:
			return 0
		case 75:
			return 1
		case 100:
			return 2
		case 150:
			return 3
		default:
			return 4
		}
	}
	// Stable insertion sort by order (small n).
	for i := 1; i < len(pending); i++ {
		for j := i; j > 0 && orderOf(pending[j].slot) < orderOf(pending[j-1].slot); j-- {
			pending[j], pending[j-1] = pending[j-1], pending[j]
		}
	}

	applied := make(map[uint16]uint16)
	spent := uint16(0)
	budget := playerPoints

	tryApply := func(e entry) bool {
		if e.points > 0 && !w.canSelectSlotLocked(uint8(e.slot), applied, playerPoints) {
			return false
		}
		if spent+e.points > budget {
			return false
		}
		applied[uint16(e.slot)] = e.points
		spent += e.points
		return true
	}

	// Greedy passes with retry (C++ loops up to 5 extra times).
	remaining := pending
	for loop := 0; loop < 6 && len(remaining) > 0; loop++ {
		var next []entry
		progressed := false
		for _, e := range remaining {
			if tryApply(e) {
				progressed = true
			} else {
				next = append(next, e)
			}
		}
		remaining = next
		if !progressed {
			break
		}
	}

	w.SlotPoints = applied
	w.dirty = true
	return true
}

// canSelectSlotLocked reports whether a slot may receive points given the slots
// already applied this save. Mirrors canPlayerSelectPointOnSlot +
// canSelectSlotFullOrPartial.
func (w *WheelOfDestiny) canSelectSlotLocked(slot uint8, applied map[uint16]uint16, playerPoints uint16) bool {
	if playerPoints < wheelSlotMinPlayerPoints(slot) {
		return false
	}
	neighbors := wheelSlotNeighbors[slot]
	if len(neighbors) == 0 {
		return true // rim slot: always selectable
	}
	for _, n := range neighbors {
		if applied[uint16(n)] == wheelMaxPointsPerSlot(n) && wheelMaxPointsPerSlot(n) > 0 {
			return true
		}
	}
	return false
}

// GetSlotPointsCopy returns a snapshot of the slot allocation.
func (w *WheelOfDestiny) GetSlotPointsCopy() map[uint16]uint16 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	out := make(map[uint16]uint16, len(w.SlotPoints))
	for k, v := range w.SlotPoints {
		out[k] = v
	}
	return out
}

// recomputeLocked rebuilds the cached bonus data from the current allocation and
// vocation, mirroring IOWheel's per-slot bonus functions.
func (w *WheelOfDestiny) recomputeLocked() {
	b := WheelBonusData{Instants: map[string]bool{}}
	for slot := uint8(1); slot <= WheelSlotCount; slot++ {
		pts := w.SlotPoints[uint16(slot)]
		if pts == 0 {
			continue
		}
		maxed := pts == wheelMaxPointsPerSlot(slot)
		applyWheelSlotBonus(slot, pts, w.cip, maxed, &b)
	}
	w.bonus = b
	w.dirty = false
}

func (w *WheelOfDestiny) ensureFresh() {
	w.mu.RLock()
	if !w.dirty {
		w.mu.RUnlock()
		return
	}
	w.mu.RUnlock()
	w.mu.Lock()
	if w.dirty {
		w.recomputeLocked()
	}
	w.mu.Unlock()
}

// Bonus getters. GetBonusCapacity returns the value already scaled to the
// player capacity unit (×100), matching C++ getCapacity.
func (w *WheelOfDestiny) bonus_() WheelBonusData {
	w.ensureFresh()
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.bonus
}

func (w *WheelOfDestiny) GetBonusHealth() uint32      { return w.bonus_().Health }
func (w *WheelOfDestiny) GetBonusMana() uint32        { return w.bonus_().Mana }
func (w *WheelOfDestiny) GetBonusCapacity() uint32    { return w.bonus_().Capacity * 100 }
func (w *WheelOfDestiny) GetBonusMitigation() float64 { return w.bonus_().Mitigation }
func (w *WheelOfDestiny) GetBonusLifeLeech() float64  { return w.bonus_().LifeLeech }
func (w *WheelOfDestiny) GetBonusManaLeech() float64  { return w.bonus_().ManaLeech }

// GetBonusSkill returns the flat wheel bonus for a skill (Fist/Melee/Distance/Magic).
func (w *WheelOfDestiny) GetBonusSkill(skill Skill) int {
	b := w.bonus_()
	switch skill {
	case SkillFist:
		return b.Fist
	case SkillClub, SkillSword, SkillAxe:
		return b.Melee
	case SkillDistance:
		return b.Distance
	default:
		return 0
	}
}

// GetBonusMagic returns the flat wheel magic-level bonus.
func (w *WheelOfDestiny) GetBonusMagic() int { return w.bonus_().Magic }

// --- per-vocation stat helpers (io_wheel.cpp) ---

// hpStd: knight 3, paladin/monk 2, others 1 (× points).
func hpStd(cip uint8, points uint16) uint32 {
	switch cip {
	case cipKnight:
		return 3 * uint32(points)
	case cipPaladin, cipMonk:
		return 2 * uint32(points)
	default:
		return 1 * uint32(points)
	}
}

// manaStd: knight 1, paladin 3, sorc/druid 6, others(monk) 2 (× points).
func manaStd(cip uint8, points uint16) uint32 {
	switch cip {
	case cipKnight:
		return 1 * uint32(points)
	case cipPaladin:
		return 3 * uint32(points)
	case cipSorcerer, cipDruid:
		return 6 * uint32(points)
	default:
		return 2 * uint32(points)
	}
}

// capStd: knight/monk 5, paladin 4, others 2 (× points).
func capStd(cip uint8, points uint16) uint32 {
	switch cip {
	case cipKnight, cipMonk:
		return 5 * uint32(points)
	case cipPaladin:
		return 4 * uint32(points)
	default:
		return 2 * uint32(points)
	}
}

// hpManaBig: the _200-slot health/mana pair. knight (3,1), paladin (2,3),
// sorc/druid (1,6), others(monk) (2,2).
func hpManaBig(cip uint8, points uint16) (uint32, uint32) {
	switch cip {
	case cipKnight:
		return 3 * uint32(points), 1 * uint32(points)
	case cipPaladin:
		return 2 * uint32(points), 3 * uint32(points)
	case cipSorcerer, cipDruid:
		return 1 * uint32(points), 6 * uint32(points)
	default:
		return 2 * uint32(points), 2 * uint32(points)
	}
}

// addSkillByVoc adds +1 to the vocation's primary skill (melee K, distance P,
// magic S/D, fist monk).
func addSkillByVoc(cip uint8, b *WheelBonusData) {
	switch cip {
	case cipKnight:
		b.Melee++
	case cipPaladin:
		b.Distance++
	case cipSorcerer, cipDruid:
		b.Magic++
	default:
		b.Fist++
	}
}

// applyWheelSlotBonus accumulates one slot's contribution into b. It ports the
// IOWheel::slot* functions. addSpell() and vessel-resonance (gem) effects are
// intentionally omitted; the maxed-slot instant unlocks are recorded as flags.
func applyWheelSlotBonus(slot uint8, points uint16, cip uint8, maxed bool, b *WheelBonusData) {
	switch slot {
	case slotGreen200, slotPurple200, slotBlue200, slotRed200:
		h, m := hpManaBig(cip, points)
		b.Health += h
		b.Mana += m
		if maxed && slot == slotGreen200 {
			b.Instants[greenInstant(cip)] = true
		}
		if maxed && slot == slotPurple200 {
			b.Instants[purpleInstant(cip)] = true
		}

	case slotGreenTop150, slotBlueTop100, slotBlueMiddle100, slotRed50, slotPurpleBottom75:
		b.Mitigation += wheelMitigationIncrease * float64(points)
		if slot == slotGreenTop150 && maxed {
			b.ManaLeech += wheelManaLeechIncrease
		}
		if slot == slotPurpleBottom75 && maxed {
			b.ManaLeech += wheelManaLeechIncrease
		}

	case slotGreenBottom150:
		b.Mitigation += wheelMitigationIncrease * float64(points)

	case slotPurpleTop75:
		b.Mitigation += wheelMitigationIncrease * float64(points)
		if maxed {
			addSkillByVoc(cip, b)
		}

	case slotBlueBottom100:
		b.Mitigation += wheelMitigationIncrease * float64(points)
		if maxed {
			addSkillByVoc(cip, b)
		}

	case slotGreenTop100, slotRedTop150, slotBlueTop75, slotPurple50, slotBlueBottom75,
		slotGreenMiddle100, slotGreenBottom100:
		b.Health += hpStd(cip, points)
		if slot == slotBlueTop75 && maxed {
			b.ManaLeech += wheelManaLeechIncrease
		}

	case slotRedBottom150:
		b.Health += hpStd(cip, points)
		if maxed {
			b.ManaLeech += wheelManaLeechIncrease
		}

	case slotGreenTop75, slotPurpleBottom150:
		b.Mana += manaStd(cip, points)
		if maxed {
			b.LifeLeech += wheelHealthLeechIncrease
		}

	case slotRedBottom100, slotPurpleTop150, slotRedMiddle100, slotBlue50:
		b.Mana += manaStd(cip, points)

	case slotRedTop100, slotGreenBottom75:
		b.Mana += manaStd(cip, points)
		if maxed {
			addSkillByVoc(cip, b)
		}

	case slotRedTop75, slotGreen50, slotBlueBottom150, slotPurpleBottom100,
		slotPurpleTop100, slotPurpleMiddle100:
		b.Capacity += capStd(cip, points)

	case slotRedBottom75, slotBlueTop150:
		b.Capacity += capStd(cip, points)
		if maxed {
			b.LifeLeech += wheelHealthLeechIncrease
		}
	}
}

func greenInstant(cip uint8) string {
	switch cip {
	case cipKnight:
		return "Battle Instinct"
	case cipPaladin:
		return "Positional Tactics"
	case cipSorcerer:
		return "Runic Mastery"
	case cipDruid:
		return "Healing Link"
	default:
		return "Guiding Presence"
	}
}

func purpleInstant(cip uint8) string {
	switch cip {
	case cipKnight:
		return "Battle Healing"
	case cipPaladin:
		return "Ballistic Mastery"
	case cipSorcerer:
		return "Focus Mastery"
	case cipDruid:
		return "Runic Mastery"
	default:
		return "Sanctuary"
	}
}
