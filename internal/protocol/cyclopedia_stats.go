package protocol

import (
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

func (g *GameProtocol) sendCyclopediaCharacterOffenceStats() {
	if g.player == nil {
		return
	}
	p := g.player
	w := netmsg.NewWriter()
	w.AddByte(0xDA)
	w.AddByte(cyclopediaCharacterInfoOffenceStats)
	w.AddByte(0x00)

	baseCrit := p.GetBaseCritical()
	critChance := float64(baseCrit.Chance) / 100.0
	w.AddDouble(critChance, 4)
	w.AddDouble(critChance, 4)
	w.AddDouble(0.0, 4)
	w.AddDouble(0.0, 4)
	w.AddDouble(0.0, 4)
	w.AddDouble(0.0, 4)

	critDmg := float64(baseCrit.Damage) / 100.0
	w.AddDouble(critDmg, 4)
	w.AddDouble(critDmg, 4)
	w.AddDouble(0.0, 4)
	w.AddDouble(0.0, 4)
	w.AddDouble(0.0, 4)
	w.AddDouble(0.0, 4)

	lifeLeech := float64(p.GetLifeLeechAmount()) / 100.0
	w.AddDouble(lifeLeech, 4)
	w.AddDouble(0.0, 4)
	w.AddDouble(0.0, 4)
	w.AddDouble(0.0, 4)
	w.AddDouble(0.0, 4)

	manaLeech := float64(p.GetManaLeechAmount()) / 100.0
	w.AddDouble(manaLeech, 4)
	w.AddDouble(0.0, 4)
	w.AddDouble(0.0, 4)
	w.AddDouble(0.0, 4)
	w.AddDouble(0.0, 4)

	w.AddDouble(p.GetForgeSkillStat(3), 4)
	w.AddDouble(p.GetForgeSkillStat(3), 4)
	w.AddDouble(0.0, 4)
	w.AddDouble(0.0, 4)

	w.AddDouble(float64(p.GetCleavePercent())/100.0, 4)

	for r := uint8(1); r <= 7; r++ {
		w.AddU16(uint16(p.GetPerfectShotDamage(r)))
	}

	flatBonus := p.CalculateFlatDamageHealing()
	w.AddU16(flatBonus)
	w.AddU16(flatBonus)
	w.AddU16(0)

	weapon := p.GetWeapon(g.deps.Items, false)
	writeOffenceWeapon(g, w, p, weapon, flatBonus)

	w.AddDouble(p.WeaponProficiency.GetPowerfulFoeDamage(), 4)

	bestiaries := p.GetActiveBestiariesDamage()
	w.AddU16(uint16(len(bestiaries)))
	for _, b := range bestiaries {
		w.AddString(b.Name)
		w.AddDouble(b.Amount, 4)
	}

	runesCrit := p.GetRunesCritical()
	autoCrit := p.GetAutoAttackCritical()
	w.AddByte(0)
	w.AddDouble(runesCrit.Chance, 4)
	w.AddDouble(autoCrit.Chance, 4)
	w.AddByte(0)
	w.AddDouble(runesCrit.Damage, 4)
	w.AddDouble(autoCrit.Damage, 4)

	w.AddU16(uint16(p.WeaponProficiency.GetStat(game.WpLifeGainOnHit)))
	w.AddU16(uint16(p.WeaponProficiency.GetStat(game.WpManaGainOnHit)))
	w.AddU16(uint16(p.WeaponProficiency.GetStat(game.WpLifeGainOnKill)))
	w.AddU16(uint16(p.WeaponProficiency.GetStat(game.WpManaGainOnKill)))

	// Skill percentage (auto attack, spell damage, spell healing)
	var weaponSkill game.Skill
	if weapon != nil {
		switch weapon.WeaponType(g.deps.Items) {
		case "sword":
			weaponSkill = game.SkillSword
		case "axe":
			weaponSkill = game.SkillAxe
		case "club":
			weaponSkill = game.SkillClub
		case "distance", "ammunition", "ammo", "missile":
			weaponSkill = game.SkillDistance
		}
	}

	skillPct := p.GetSkillPercentage(weaponSkill)
	hasActiveSkill := skillPct.Skill != 0 || (skillPct.AutoAttack+skillPct.SpellDamage+skillPct.SpellHealing) > 0
	var playerSkill float64
	if hasActiveSkill {
		playerSkill = float64(p.GetEffectiveSkill(skillPct.Skill))
	}

	// Auto Attack
	hasAA := hasActiveSkill && skillPct.AutoAttack > 0
	w.AddByte(boolToByte(hasAA))
	if hasAA {
		w.AddByte(game.GetWeaponCipbiaSkill(skillPct.Skill))
		w.AddDouble(skillPct.AutoAttack, 4)
		w.AddDouble(float64(int64(playerSkill*skillPct.AutoAttack+0.5)), 4) // round
	}

	// Spell Damage
	hasSD := hasActiveSkill && skillPct.SpellDamage > 0
	w.AddByte(boolToByte(hasSD))
	if hasSD {
		w.AddByte(game.GetWeaponCipbiaSkill(skillPct.Skill))
		w.AddDouble(skillPct.SpellDamage, 4)
		w.AddDouble(float64(int64(playerSkill*skillPct.SpellDamage+0.5)), 4)
	}

	// Spell Healing
	hasSH := hasActiveSkill && skillPct.SpellHealing > 0
	w.AddByte(boolToByte(hasSH))
	if hasSH {
		w.AddByte(game.GetWeaponCipbiaSkill(skillPct.Skill))
		w.AddDouble(skillPct.SpellHealing, 4)
		w.AddDouble(float64(int64(playerSkill*skillPct.SpellHealing+0.5)), 4)
	}

	w.AddDouble(0.0, 4) // Full hit points extra damage
	w.AddDouble(0.0, 4) // Low hit points extra damage
	w.AddDouble(0.0, 4) // Armor penetration
	w.AddByte(0x00)     // Elemental pierces count

	g.SendToClient(w)
}

func writeOffenceWeapon(g *GameProtocol, w *netmsg.Writer, p *game.Player, weapon *game.Item, flatBonus uint16) {
	cat := g.deps.Items

	if weapon == nil {
		// FIST
		attackValue := uint16(7)
		skillLevel := p.GetEffectiveSkill(game.SkillFist)
		skillID := uint8(game.CipbiaSkillFist)
		attackSkill := p.GetDistanceAttackSkill(int32(skillLevel), int32(attackValue))
		rawTotal := p.AttackRawTotal(flatBonus, attackValue, skillLevel)
		total := p.AttackTotal(flatBonus, attackValue, skillLevel)

		w.AddU16(total)
		w.AddU16(flatBonus)
		w.AddU16(attackValue)
		w.AddByte(skillID)
		w.AddU16(attackSkill)
		w.AddU16(total - rawTotal)
		w.AddByte(game.CipbiaElementPhysical)
		w.AddDouble(0.0, 4)
		w.AddByte(0)
		w.AddByte(0)
		return
	}

	wt := weapon.WeaponType(cat)
	it := cat.Get(weapon.ID)

	if wt == "wand" {
		maxHit := uint16(it.MaxHitChance)
		if maxHit == 0 {
			maxHit = 100
		}
		elementByte := byte(game.CipbiaElementPhysical)
		if it.ElementType != 0 {
			elementByte = byte(game.GetCipbiaElement(int(it.ElementType)))
		}
		w.AddU16(maxHit)
		w.AddU16(0)
		w.AddU16(0)
		w.AddByte(0x00)
		w.AddU16(0)
		w.AddU16(0)
		w.AddByte(elementByte)
		w.AddDouble(0.0, 4)
		w.AddByte(0)
		w.AddByte(0)
		return
	}

	// MELEE or DISTANCE
	physicalAttack := weapon.Attack(cat)
	if physicalAttack < 0 {
		physicalAttack = 0
	}
	physicalAttack += int32(p.WeaponProficiency.GetStat(game.WpAttackDamage))
	if physicalAttack < 0 {
		physicalAttack = 0
	}

	hasElement := false
	elementType := uint8(0)
	elementalAttack := int32(0)
	if it.ElementType != 0 && it.ElementDamage > 0 {
		hasElement = true
		elementType = it.ElementType
		elementalAttack = int32(it.ElementDamage)
	}

	attackValue := physicalAttack + elementalAttack
	if attackValue < 0 {
		attackValue = 0
	}

	isDist := wt == "distance" || wt == "ammunition" || wt == "ammo" || wt == "missile"
	if wt == "ammunition" || wt == "ammo" {
		if launcher := p.GetWeapon(cat, true); launcher != nil {
			if launcherAtk := launcher.Attack(cat); launcherAtk > 0 {
				attackValue += launcherAtk
			}
		}
	}

	skillID := game.GetWeaponSkillId(wt)
	var skillLevel uint16
	switch wt {
	case "sword":
		skillLevel = p.GetEffectiveSkill(game.SkillSword)
	case "axe":
		skillLevel = p.GetEffectiveSkill(game.SkillAxe)
	case "club":
		skillLevel = p.GetEffectiveSkill(game.SkillClub)
	case "distance", "ammunition", "ammo", "missile":
		skillLevel = p.GetEffectiveSkill(game.SkillDistance)
	default:
		skillLevel = p.GetEffectiveSkill(game.SkillFist)
	}

	var attackSkill uint16
	if isDist {
		attackSkill = p.GetDistanceAttackSkill(int32(skillLevel), attackValue)
	} else {
		attackSkill = p.GetAttackSkill(weapon)
	}

	rawTotal := p.AttackRawTotal(flatBonus, uint16(attackValue), skillLevel)
	total := p.AttackTotal(flatBonus, uint16(attackValue), skillLevel)

	w.AddU16(total)
	w.AddU16(flatBonus)
	w.AddU16(uint16(attackValue))
	w.AddByte(skillID)
	w.AddU16(attackSkill)
	w.AddU16(total - rawTotal)
	w.AddByte(game.CipbiaElementPhysical)

	// Converted Damage / Imbuement Damage
	if hasElement {
		if physicalAttack > 0 {
			w.AddDouble(float64(elementalAttack)/float64(attackValue), 4)
		} else {
			w.AddDouble(0.0, 4)
		}
		w.AddByte(game.GetCipbiaElement(int(elementType)))
	} else {
		w.AddDouble(0, 4)
		w.AddByte(0)
	}

	if isDist {
		accuracy := p.GetDamageAccuracy(weapon)
		w.AddByte(uint8(len(accuracy)))
		for i, acc := range accuracy {
			w.AddByte(uint8(i + 1))
			w.AddDouble(acc/100.0, 4)
		}
	} else {
		w.AddByte(0)
	}
}

// sendCyclopediaCharacterDefenceStats sends the frame; buildDefenceStats builds it.
// They are split so a test can walk the bytes with the client's own read
// sequence — SendToClient needs a live connection, which the protocol tests do
// not have.
func (g *GameProtocol) sendCyclopediaCharacterDefenceStats() {
	if w := g.buildDefenceStats(); w != nil {
		g.SendToClient(w)
	}
}

func (g *GameProtocol) buildDefenceStats() *netmsg.Writer {
	if g.player == nil {
		return nil
	}
	p := g.player
	w := netmsg.NewWriter()
	w.AddByte(0xDA)
	w.AddByte(cyclopediaCharacterInfoDefenceStats)
	w.AddByte(0x00)

	w.AddDouble(p.GetForgeSkillStat(5), 4) // dodge total (forge + wheel, wheel=0 for now)
	w.AddDouble(p.GetForgeSkillStat(5), 4) // dodge equipment
	w.AddDouble(0.0, 4)                    // dodge forge-only
	w.AddDouble(0.0, 4)                    // (unused)
	w.AddDouble(0.0, 4)                    // wheel dodge (TBD)

	w.AddU32(uint32(max(0, p.GetMagicShieldCapacityFlat()) * (1 + p.GetMagicShieldCapacityPercent())))
	w.AddU16(uint16(p.GetMagicShieldCapacityFlat()))
	w.AddDouble(float64(p.GetMagicShieldCapacityPercent()), 4)

	w.AddU16(uint16(p.GetReflectFlat(0)))
	w.AddU16(uint16(p.GetArmor()))
	w.AddU16(uint16(p.GetMantra()))

	shieldSkill := p.GetEffectiveSkill(game.SkillShielding)
	w.AddU16(uint16(p.GetDefense()))
	w.AddU16(p.GetDefenseEquipment())
	w.AddByte(0x06)
	w.AddU16(uint16(shieldSkill))
	w.AddU16(0) // defenseWheel from mastery (TBD)
	// A spare u16 the client reads and discards
	// (otclient/src/client/protocolgameparse.cpp:6135). It was missing, and
	// everything after it landed two bytes early.
	w.AddU16(0)

	w.AddDouble(p.GetMitigation()/100.0, 4)                  // mitigation
	w.AddDouble(0.0, 4)                                      // mitigationBase
	w.AddDouble(float64(p.GetDefenseEquipment())/10000.0, 4) // mitigationEquipment
	w.AddDouble(p.ShieldSkillMitigationFactor(), 4)          // mitigationShield
	w.AddDouble(0.0, 4)                                      // mitigationWheel (TBD)
	// mitigationCombatTactics (parse:6142). Also missing — between this and the
	// spare u16 the frame ended 7 bytes short, and the client read off the end:
	//
	//	129 bytes, 0 unread at pos 129, last opcode 0xDA (218)
	//
	// 129 is exactly what this function used to produce for a player with seven
	// absorption entries.
	w.AddDouble(0.0, 4)

	absorbs := p.GetCombatAbsorbs()
	w.AddByte(uint8(len(absorbs)))
	for _, a := range absorbs {
		w.AddByte(0x04)
		w.AddByte(byte(a.Element))
		w.AddDouble(a.Absorb, 4)
	}

	return w
}

func (g *GameProtocol) sendCyclopediaCharacterMiscStats() {
	if g.player == nil {
		return
	}
	p := g.player
	w := netmsg.NewWriter()
	w.AddByte(0xDA)
	w.AddByte(cyclopediaCharacterInfoMiscStats)
	w.AddByte(0x00)

	for i := 0; i < 5; i++ {
		w.AddDouble(p.GetForgeSkillStat(1), 4)
	}
	for i := 0; i < 4; i++ {
		w.AddDouble(p.GetForgeSkillStat(7), 4)
	}
	for i := 0; i < 3; i++ {
		w.AddDouble(p.GetForgeSkillStat(8), 4)
	}

	have := uint8(0)
	for idx := 1; idx < len(p.Blessings); idx++ {
		if p.Blessings[idx] > 0 {
			have++
		}
	}
	w.AddByte(have)
	w.AddByte(uint8(len(p.Blessings) - 1))

	concoctions := p.GetActiveConcoctions()
	w.AddByte(uint8(len(concoctions)))
	for _, c := range concoctions {
		w.AddU16(c.ItemID)
		w.AddByte(0)
		w.AddByte(0)
		w.AddU32(c.TimeLeft)
	}

	foods := p.GetActiveFoods()
	w.AddByte(uint8(len(foods)))
	for _, f := range foods {
		w.AddU16(f.ItemID)
		w.AddByte(0)
		w.AddByte(0)
		w.AddU32(f.TimeLeft)
	}

	weaponAugments := p.GetWeaponProficiencyAugments()
	w.AddByte(uint8(len(weaponAugments)))
	for _, a := range weaponAugments {
		w.AddU16(a.SpellID)
		w.AddByte(a.Id)
		w.AddDouble(a.Data, 4)
	}

	wheelAugments := p.GetWheelAugments()
	w.AddByte(uint8(len(wheelAugments)))
	for _, a := range wheelAugments {
		w.AddU16(a.SpellID)
		w.AddByte(a.Id)
		w.AddDouble(a.Data, 4)
	}

	equippedAugments := p.GetEquippedAugments()
	w.AddByte(uint8(len(equippedAugments)))
	for _, a := range equippedAugments {
		w.AddU16(a.SpellID)
		w.AddByte(a.Id)
		w.AddDouble(a.Data, 4)
	}

	g.SendToClient(w)
}