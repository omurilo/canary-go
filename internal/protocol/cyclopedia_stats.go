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

	w.AddDouble(0.0, 4)

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

	w.AddU16(0)
	w.AddU16(0)
	w.AddU16(0)
	w.AddU16(0)

	w.AddByte(0)
	w.AddByte(0)
	w.AddByte(0)

	w.AddDouble(0.0, 4)
	w.AddDouble(0.0, 4)
	w.AddDouble(0.0, 4)
	w.AddByte(0)

	g.SendToClient(w)
}

func writeOffenceWeapon(g *GameProtocol, w *netmsg.Writer, p *game.Player, weapon *game.Item, flatBonus uint16) {
	cat := g.deps.Items
	attackValue := uint16(7)
	skillLevel := p.GetEffectiveSkill(game.SkillFist)
	skillID := uint8(game.CipbiaSkillFist)

	if weapon != nil {
		wt := weapon.WeaponType(cat)
		skillID = game.GetWeaponSkillId(wt)
		switch wt {
		case "sword":
			skillLevel = p.GetEffectiveSkill(game.SkillSword)
		case "axe":
			skillLevel = p.GetEffectiveSkill(game.SkillAxe)
		case "club":
			skillLevel = p.GetEffectiveSkill(game.SkillClub)
		case "distance", "ammunition", "missile":
			skillLevel = p.GetEffectiveSkill(game.SkillDistance)
		}
		if wt != "wand" && wt != "" {
			attackValue = uint16(weapon.Attack(cat))
			if attackValue == 0 {
				attackValue = 7
			}
		}
	}

	var attackSkill uint16
	if weapon != nil {
		wt := weapon.WeaponType(cat)
		if wt == "distance" || wt == "ammunition" || wt == "missile" {
			attackSkill = p.GetDistanceAttackSkill(int32(skillLevel), int32(attackValue))
		} else {
			attackSkill = p.GetAttackSkill(weapon)
		}
	} else {
		attackSkill = p.GetDistanceAttackSkill(int32(skillLevel), int32(attackValue))
	}

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
}

func (g *GameProtocol) sendCyclopediaCharacterDefenceStats() {
	if g.player == nil {
		return
	}
	p := g.player
	w := netmsg.NewWriter()
	w.AddByte(0xDA)
	w.AddByte(cyclopediaCharacterInfoDefenceStats)
	w.AddByte(0x00)

	w.AddDouble(p.GetForgeSkillStat(5), 4)
	w.AddDouble(p.GetForgeSkillStat(5), 4)
	w.AddDouble(0.0, 4)
	w.AddDouble(0.0, 4)
	w.AddDouble(0.0, 4)

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
	w.AddU16(0)

	w.AddDouble(p.GetMitigation(), 4)
	w.AddDouble(0.0, 4)
	w.AddDouble(0.0, 4)
	w.AddDouble(0.0, 4)
	w.AddDouble(0.0, 4)

	absorbs := p.GetCombatAbsorbs()
	w.AddByte(uint8(len(absorbs)))
	for _, a := range absorbs {
		w.AddByte(0x04)
		w.AddByte(byte(a.Element))
		w.AddDouble(a.Absorb, 4)
	}

	g.SendToClient(w)
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

	w.AddByte(uint8(len(p.GetWeaponProficiencyAugments())))
	w.AddByte(uint8(len(p.GetWheelAugments())))
	w.AddByte(uint8(len(p.GetEquippedAugments())))

	g.SendToClient(w)
}