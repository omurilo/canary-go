package combat

import (
	
)

// CombatType represents the type of combat damage
type CombatType uint32

const (
	CombatNone CombatType = 0
	CombatPhysical CombatType = 1 << 0
	CombatEnergy CombatType = 1 << 1
	CombatEarth CombatType = 1 << 2
	CombatFire CombatType = 1 << 3
	CombatUndefined CombatType = 1 << 4
	CombatHealing CombatType = 1 << 5
	CombatDeath CombatType = 1 << 6
	CombatIce CombatType = 1 << 7
	CombatHoly CombatType = 1 << 8
	CombatManaDrain CombatType = 1 << 9
	CombatLifeDrain CombatType = 1 << 10
)

// ConditionType represents the type of condition
type ConditionType uint32

const (
	ConditionNone ConditionType = 0
	ConditionPoison ConditionType = 1 << 0
	ConditionFire ConditionType = 1 << 1
	ConditionEnergy ConditionType = 1 << 2
	ConditionBleeding ConditionType = 1 << 3
	ConditionHaste ConditionType = 1 << 4
	ConditionParalyze ConditionType = 1 << 5
	ConditionOutfit ConditionType = 1 << 6
	ConditionInvisible ConditionType = 1 << 7
	ConditionLight ConditionType = 1 << 8
	ConditionManaShield ConditionType = 1 << 9
	ConditionInFight ConditionType = 1 << 10
	ConditionDrunk ConditionType = 1 << 11
	ConditionExhaust ConditionType = 1 << 12
	ConditionFood ConditionType = 1 << 13
	ConditionRegeneration ConditionType = 1 << 14
	ConditionSoul ConditionType = 1 << 15
	ConditionMuted ConditionType = 1 << 16
	ConditionChannelMutedCondition ConditionType = 1 << 17
	ConditionYellTicks ConditionType = 1 << 18
	ConditionAttributes ConditionType = 1 << 19
	ConditionFreezing ConditionType = 1 << 20
	ConditionDazzled ConditionType = 1 << 21
	ConditionCursed ConditionType = 1 << 22
	ConditionPacified ConditionType = 1 << 23
	ConditionSpellCooldown ConditionType = 1 << 24
	ConditionSpellGroupCooldown ConditionType = 1 << 25
)

type ConditionId uint32

// CombatOrigin represents where the combat came from
type CombatOrigin uint8

const (
	OriginNone CombatOrigin = iota
	OriginCondition
	OriginSpell
	OriginMelee
	OriginRanged
)

// CombatParam represents parameters that can be set for a combat
type CombatParam uint32

const (
	CombatParamType CombatParam = iota
	CombatParamEffect
	CombatParamDistanceEffect
	CombatParamBlockArmor
	CombatParamBlockShield
	CombatParamTargetCasterOrTopMost
	CombatParamCreateItem
	CombatParamUpdateItem
	CombatParamAggressive
	CombatParamDispel
	CombatParamChainEffect
	CombatParamCastSound
	CombatParamImpactSound
	CombatParamUseCharges
)

// FormulaType represents how the damage formula is calculated
type FormulaType uint8

const (
	CombatFormulaUndefined FormulaType = iota
	CombatFormulaLevelMagic
	CombatFormulaSkill
	CombatFormulaDamage
)

// Interfaces for mock/integration

type Position struct {
	X, Y, Z uint16
}

type Creature interface {
	GetId() uint32
	GetPosition() Position
	GetHealth() int32
	GetMaxHealth() int32
	GetMana() int32
	GetMaxMana() int32
	AddCondition(condition Condition) error
	RemoveCondition(conditionType ConditionType)
	HasCondition(conditionType ConditionType) bool
	ChangeHealth(amount int32)
	ChangeMana(amount int32)
	NotifyStatsChange()
	GetBaseSpeed() uint16
	ChangeSpeed(delta int32)
	GetArmor() int32
	GetDefense() int32
	GetResistance(combatType CombatType) int16
	IsInProtectionZone() bool
	IsPlayer() bool
}

type Player interface {
	Creature
	GetLevel() uint32
	GetMagicLevel() uint32
	IsSecureMode() bool
	GetCriticalChance() uint16
	GetCriticalDamage() uint16
	GetLifeLeechChance() uint16
	GetLifeLeechAmount() uint16
	GetManaLeechChance() uint16
	GetManaLeechAmount() uint16
	GetReflectPercent() uint16
	GetAbsorbPercent() uint16
}

type Tile interface {
	GetPosition() Position
	GetCreatures() []Creature
}

// CombatDamage represents the damage or healing amount
type CombatDamage struct {
	PrimaryType CombatType
	PrimaryValue int32
	SecondaryType CombatType
	SecondaryValue int32
	Origin CombatOrigin
}

type IntervalInfo struct {
	Damage int32
	Ticks int32
}

// PlayerIcon represents the status icons shown in the client
type PlayerIcon uint8

const (
	PlayerIconPoison        PlayerIcon = 0
	PlayerIconBurn          PlayerIcon = 1
	PlayerIconEnergy        PlayerIcon = 2
	PlayerIconDrunk         PlayerIcon = 3
	PlayerIconManaShield    PlayerIcon = 4
	PlayerIconParalyze      PlayerIcon = 5
	PlayerIconHaste         PlayerIcon = 6
	PlayerIconSwords        PlayerIcon = 7
	PlayerIconDrowning      PlayerIcon = 8
	PlayerIconFreezing      PlayerIcon = 9
	PlayerIconDazzled       PlayerIcon = 10
	PlayerIconCursed        PlayerIcon = 11
	PlayerIconPartyBuff     PlayerIcon = 12
	PlayerIconRedSwords     PlayerIcon = 13
	PlayerIconPigeon        PlayerIcon = 14
	PlayerIconBleeding      PlayerIcon = 15
	PlayerIconLesserHex     PlayerIcon = 16
	PlayerIconIntenseHex    PlayerIcon = 17
	PlayerIconGreaterHex    PlayerIcon = 18
	PlayerIconRooted        PlayerIcon = 19
	PlayerIconFeared        PlayerIcon = 20
	PlayerIconGoshnarTaint1 PlayerIcon = 21
	PlayerIconGoshnarTaint2 PlayerIcon = 22
	PlayerIconGoshnarTaint3 PlayerIcon = 23
	PlayerIconGoshnarTaint4 PlayerIcon = 24
	PlayerIconGoshnarTaint5 PlayerIcon = 25
	PlayerIconNewManaShield PlayerIcon = 26
	PlayerIconAgony         PlayerIcon = 27
	PlayerIconPowerless     PlayerIcon = 28
	PlayerIconMentorOther   PlayerIcon = 29
	PlayerIconCount         PlayerIcon = 30
)
