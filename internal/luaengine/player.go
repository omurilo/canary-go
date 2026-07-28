package luaengine

import (
	"strings"
	"sync"
	"time"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/game/vocations"
	lua "github.com/yuin/gopher-lua"
)

func checkPlayer(L *lua.LState) *game.Player {
	ud := L.CheckUserData(1)
	if v, ok := ud.Value.(*game.Player); ok {
		return v
	}
	L.ArgError(1, "Player expected")
	return nil
}

// registerPlayerType registers the Player userdata type.
func (e *Engine) registerPlayerType() {
	mt := e.L.NewTypeMetatable("Player")
	// Player IS-A Creature (C++ Player : Creature), so it must expose every
	// Creature method (getId, getPosition, getHealth, say, teleportTo, ...).
	// Layer creature methods first, then let player-specific methods override.
	// Methods live directly on the metatable (see registerCreatureType) so the
	// datapack's revscriptsys CreatureIndex (getmetatable(self)[key]) finds them.
	e.L.SetFuncs(mt, creatureMethods)
	e.L.SetFuncs(mt, playerMethods)
	e.L.SetField(mt, "teleportTo", e.L.NewFunction(e.creatureTeleportto))
	e.L.SetField(mt, "changeSpeed", e.L.NewFunction(e.creatureChangespeed))
	e.L.SetField(mt, "setSpeed", e.L.NewFunction(e.creatureSetspeed))
	e.L.SetField(mt, "getParent", e.L.NewFunction(e.creatureGetparent))
	e.L.SetField(mt, "getTile", e.L.NewFunction(e.creatureGettile))
	e.L.SetField(mt, "remove", e.L.NewFunction(e.creatureRemove))
	// Inventory bindings that need the item catalog (name->id, stack size,
	// container capacity) override the package-level stubs. Same pattern as
	// teleportTo: SetField wins over the SetFuncs map because __index == mt.
	e.L.SetField(mt, "getItemCount", e.L.NewFunction(e.playerGetitemcount))
	e.L.SetField(mt, "getItemById", e.L.NewFunction(e.playerGetitembyid))
	e.L.SetField(mt, "addItem", e.L.NewFunction(e.playerAdditem))
	e.L.SetField(mt, "addItemEx", e.L.NewFunction(e.playerAdditemex))
	e.L.SetField(mt, "removeItem", e.L.NewFunction(e.playerRemoveitem))
	e.L.SetField(mt, "getFreeBackpackSlots", e.L.NewFunction(e.playerGetfreebackpackslots))
	// Container bindings (open-container state shared with the protocol layer).
	e.L.SetField(mt, "getStoreInbox", e.L.NewFunction(e.playerGetstoreinbox))
	e.L.SetField(mt, "getInbox", e.L.NewFunction(e.playerGetinbox))
	e.L.SetField(mt, "getContainerId", e.L.NewFunction(e.playerGetcontainerid))
	e.L.SetField(mt, "getContainerById", e.L.NewFunction(e.playerGetcontainerbyid))
	e.L.SetField(mt, "getContainerIndex", e.L.NewFunction(e.playerGetcontainerindex))
	e.L.SetField(mt, "sendContainer", e.L.NewFunction(e.playerSendcontainer))
	e.L.SetField(mt, "sendUpdateContainer", e.L.NewFunction(e.playerSendupdatecontainer))
	e.L.SetField(mt, "addItemBatchToPaginedContainer", e.L.NewFunction(e.playerAdditembatchtopaginedcontainer))
	e.L.SetField(mt, "getParty", e.L.NewFunction(e.playerGetparty))
	e.L.SetField(mt, "getLootPouch", e.L.NewFunction(e.playerGetlootpouch))
	e.L.SetField(mt, "say", e.L.NewFunction(e.playerSay))
	e.L.SetField(mt, "setTown", e.L.NewFunction(e.playerSettown))
	e.L.SetField(mt, "getTown", e.L.NewFunction(e.playerGettown))
	e.L.SetField(mt, "applyImbuementScroll", e.L.NewFunction(e.playerApplyscrollimbuement))
	e.L.SetField(mt, "__index", mt)
	e.registerBosstiaryPlayerMethods()
	e.registerBestiaryPlayerMethods()
	e.registerKVStoreType()
}

var playerMethods = map[string]lua.LGFunction{
	"resetCharmsBestiary":            playerResetcharmsbestiary,
	"getWheelPoints":                 playerGetwheelpoints,
	"getWheelSpentPoints":            playerGetwheelspentpoints,
	"getWheelSpells":                 playerGetwheelspells,
	"addWheelPoints":                 playerAddwheelpoints,
	"unlockAllCharmRunes":            playerUnlockallcharmrunes,
	"addCharmPoints":                 playerAddcharmpoints,
	"addMinorCharmEchoes":            playerAddminorcharmechoes,
	"getCharmTier":                   playerGetcharmtier,
	"getCharmChance":                 playerGetcharmchance,
	"resetOldCharms":                 playerResetoldcharms,
	"isPlayer":                       playerIsplayer,
	"getGuid":                        playerGetguid,
	"getIp":                          playerGetip,
	"getAccountId":                   playerGetaccountid,
	"getLastLoginSaved":              playerGetlastloginsaved,
	"getLastLogout":                  playerGetlastlogout,
	"getAccountType":                 playerGetaccounttype,
	"setAccountType":                 playerSetaccounttype,
	"isMonsterBestiaryUnlocked":      playerIsmonsterbestiaryunlocked,
	"addBestiaryKill":                playerAddbestiarykill,
	"charmExpansion":                 playerCharmexpansion,
	"getCharmMonsterType":            playerGetcharmmonstertype,
	"isMonsterPrey":                  playerIsmonsterprey,
	"getPreyCards":                   playerGetpreycards,
	"canReceiveStoreItems":           playerCanreceivestoreitems,
	"sendButtonIndication":           playerSendbuttonindication,
	"getPreyLootPercentage":          playerGetpreylootpercentage,
	"getPreyExperiencePercentage":    playerGetpreyexperiencepercentage,
	"preyThirdSlot":                  playerPreythirdslot,
	"taskHuntingThirdSlot":           playerTaskhuntingthirdslot,
	"removePreyStamina":              playerRemovepreystamina,
	"addPreyCards":                   playerAddpreycards,
	"removeTaskHuntingPoints":        playerRemovetaskhuntingpoints,
	"getTaskHuntingPoints":           playerGettaskhuntingpoints,
	"addTaskHuntingPoints":           playerAddtaskhuntingpoints,
	"getCapacity":                    playerGetcapacity,
	"setCapacity":                    playerSetcapacity,
	"isTraining":                     playerIstraining,
	"setTraining":                    playerSettraining,
	"getFreeCapacity":                playerGetfreecapacity,
	"getKills":                       playerGetkills,
	"setKills":                       playerSetkills,
	"getReward":                      playerGetreward,
	"removeReward":                   playerRemovereward,
	"getRewardList":                  playerGetrewardlist,
	"setDailyReward":                 playerSetdailyreward,
	"sendInventory":                  playerSendinventory,
	"sendLootStats":                  playerSendlootstats,
	"updateSupplyTracker":            playerUpdatesupplytracker,
	"updateKillTracker":              playerUpdatekilltracker,
	"getDepotLocker":                 playerGetdepotlocker,
	"getDepotChest":                  playerGetdepotchest,
	"getSkullTime":                   playerGetskulltime,
	"setSkullTime":                   playerSetskulltime,
	"getDeathPenalty":                playerGetdeathpenalty,
	"getExperience":                  playerGetexperience,
	"addExperience":                  playerAddexperience,
	"addAchievementProgress":         playerAddAchievementProgress,
	"removeExperience":               playerRemoveexperience,
	"getLevel":                       playerGetlevel,
	"getMagicShieldCapacityFlat":     playerGetmagicshieldcapacityflat,
	"getMagicShieldCapacityPercent":  playerGetmagicshieldcapacitypercent,
	"sendSpellCooldown":              playerSendspellcooldown,
	"sendSpellGroupCooldown":         playerSendspellgroupcooldown,
	"getMagicLevel":                  playerGetmagiclevel,
	"getMagicLevelPercent":           playerGetmagiclevelpercent,
	"getBaseMagicLevel":              playerGetbasemagiclevel,
	"getMana":                        playerGetmana,
	"addMana":                        playerAddmana,
	"getMaxMana":                     playerGetmaxmana,
	"setMaxMana":                     playerSetmaxmana,
	"getManaSpent":                   playerGetmanaspent,
	"addManaSpent":                   playerAddmanaspent,
	"getBaseMaxHealth":               playerGetbasemaxhealth,
	"getBaseMaxMana":                 playerGetbasemaxmana,
	"getSkillLevel":                  playerGetskilllevel,
	"getEffectiveSkillLevel":         playerGeteffectiveskilllevel,
	"getSkillPercent":                playerGetskillpercent,
	"getSkillTries":                  playerGetskilltries,
	"addSkillTries":                  playerAddskilltries,
	"setLevel":                       playerSetlevel,
	"setMagicLevel":                  playerSetmagiclevel,
	"setSkillLevel":                  playerSetskilllevel,
	"addOfflineTrainingTime":         playerAddofflinetrainingtime,
	"getOfflineTrainingTime":         playerGetofflinetrainingtime,
	"removeOfflineTrainingTime":      playerRemoveofflinetrainingtime,
	"addOfflineTrainingTries":        playerAddofflinetrainingtries,
	"getOfflineTrainingSkill":        playerGetofflinetrainingskill,
	"setOfflineTrainingSkill":        playerSetofflinetrainingskill,
	"getItemCount":                   playerGetitemcount,
	"getStashItemCount":              playerGetstashitemcount,
	"getItemById":                    playerGetitembyid,
	"getVocation":                    playerGetvocation,
	"setVocation":                    playerSetvocation,
	"isPromoted":                     playerIspromoted,
	"isPremium":                      playerIspremium,
	"getFinalBaseRateExperience":     playerGetfinalbaserateexperience,
	"getSex":                         playerGetsex,
	"setSex":                         playerSetsex,
	"getPronoun":                     playerGetpronoun,
	"setPronoun":                     playerSetpronoun,
	"getTown":                        playerGettown,
	"setTown":                        playerSettown,
	"getGuild":                       playerGetguild,
	"setGuild":                       playerSetguild,
	"getGuildLevel":                  playerGetguildlevel,
	"setGuildLevel":                  playerSetguildlevel,
	"getGuildNick":                   playerGetguildnick,
	"setGuildNick":                   playerSetguildnick,
	"getGroup":                       playerGetgroup,
	"setGroup":                       playerSetgroup,
	"setSpecialContainersAvailable":  playerSetspecialcontainersavailable,
	"getStashCount":                  playerGetstashcount,
	"openStash":                      playerOpenstash,
	"canReceiveLoot":                 playerCanreceiveloot,
	"getStamina":                     playerGetstamina,
	"setStamina":                     playerSetstamina,
	"getSoul":                        playerGetsoul,
	"addSoul":                        playerAddsoul,
	"getMaxSoul":                     playerGetmaxsoul,
	"getBankBalance":                 playerGetbankbalance,
	"setBankBalance":                 playerSetbankbalance,
	"removeMoneyBank":                playerRemovemoneybank,
	"depositMoney":                   playerDepositmoney,
	"withdrawMoney":                  playerWithdrawmoney,
	"getStorageValue":                playerGetstoragevalue,
	"setStorageValue":                playerSetstoragevalue,
	"sendCancelMessage":              func(L *lua.LState) int { return 0 },
	"addItem":                        playerAdditem,
	"addItemEx":                      playerAdditemex,
	"addItemBatchToPaginedContainer": playerAdditembatchtopaginedcontainer,
	"addItemStash":                   playerAdditemstash,
	"removeStashItem":                playerRemovestashitem,
	"removeItem":                     playerRemoveitem,
	"sendContainer":                  playerSendcontainer,
	"sendUpdateContainer":            playerSendupdatecontainer,
	"getMoney":                       playerGetmoney,
	"addMoney":                       playerAddmoney,
	"removeMoney":                    playerRemovemoney,
	"showTextDialog":                 playerShowtextdialog,
	"sendTextMessage":                playerSendtextmessage,
	"sendChannelMessage":             playerSendchannelmessage,
	"sendPrivateMessage":             playerSendprivatemessage,
	"channelSay":                     playerChannelsay,
	"openChannel":                    playerOpenchannel,
	"getSlotItem":                    playerGetslotitem,
	"getBackpack":                    playerGetbackpack,
	// getParty is registered as an engine-method override in registerPlayerType
	// (it needs e to build the Party userdata).
	"addOutfit":                       playerAddoutfit,
	"addOutfitAddon":                  playerAddoutfitaddon,
	"removeOutfit":                    playerRemoveoutfit,
	"removeOutfitAddon":               playerRemoveoutfitaddon,
	"hasOutfit":                       playerHasoutfit,
	"sendOutfitWindow":                playerSendoutfitwindow,
	"addMount":                        playerAddmount,
	"removeMount":                     playerRemovemount,
	"hasMount":                        playerHasmount,
	"addFamiliar":                     playerAddfamiliar,
	"removeFamiliar":                  playerRemovefamiliar,
	"hasFamiliar":                     playerHasfamiliar,
	"setFamiliarLooktype":             playerSetfamiliarlooktype,
	"getFamiliarLooktype":             playerGetfamiliarlooktype,
	"getPremiumDays":                  playerGetpremiumdays,
	"addPremiumDays":                  playerAddpremiumdays,
	"removePremiumDays":               playerRemovepremiumdays,
	"getTibiaCoins":                   playerGettibiacoins,
	"addTibiaCoins":                   playerAddtibiacoins,
	"removeTibiaCoins":                playerRemovetibiacoins,
	"getTransferableCoins":            playerGettransferablecoins,
	"addTransferableCoins":            playerAddtransferablecoins,
	"removeTransferableCoins":         playerRemovetransferablecoins,
	"removeTransferableAndTibiaCoins": playerRemovetransferableandtibiacoins,
	"sendBlessStatus":                 playerSendblessstatus,
	"sendBlessingsDialog":             playerSendblessingsdialog,
	"hasBlessing":                     playerHasblessing,
	"addBlessing":                     playerAddblessing,
	"removeBlessing":                  playerRemoveblessing,
	"getBlessingCount":                playerGetblessingcount,
	"canLearnSpell":                   playerCanlearnspell,
	"learnSpell":                      playerLearnspell,
	"forgetSpell":                     playerForgetspell,
	"hasLearnedSpell":                 playerHaslearnedspell,
	"openImbuementWindow":             playerOpenimbuementwindow,
	"closeImbuementWindow":            playerCloseimbuementwindow,
	"clearAllImbuements":              playerClearallimbuements,
	"sendTutorial":                    playerSendtutorial,
	"addMapMark":                      playerAddmapmark,
	"save":                            playerSave,
	"popupFYI":                        playerPopupfyi,
	"isPzLocked":                      playerIspzlocked,
	"getClient":                       playerGetclient,
	"getHouse":                        playerGethouse,
	"sendHouseWindow":                 playerSendhousewindow,
	"setEditHouse":                    playerSetedithouse,
	"setGhostMode":                    playerSetghostmode,
	"getContainerId":                  playerGetcontainerid,
	"getContainerById":                playerGetcontainerbyid,
	"getContainerIndex":               playerGetcontainerindex,
	"getInstantSpells":                playerGetinstantspells,
	"canCast":                         playerCancast,
	"hasChaseMode":                    playerHaschasemode,
	"hasSecureMode":                   playerHassecuremode,
	"getFightMode":                    playerGetfightmode,
	"getBaseXpGain":                   playerGetbasexpgain,
	"setBaseXpGain":                   playerSetbasexpgain,
	"getVoucherXpBoost":               playerGetvoucherxpboost,
	"setVoucherXpBoost":               playerSetvoucherxpboost,
	"getGrindingXpBoost":              playerGetgrindingxpboost,
	"setGrindingXpBoost":              playerSetgrindingxpboost,
	"getXpBoostPercent":               playerGetxpboostpercent,
	"setXpBoostPercent":               playerSetxpboostpercent,
	"getStaminaXpBoost":               playerGetstaminaxpboost,
	"setStaminaXpBoost":               playerSetstaminaxpboost,
	"getXpBoostTime":                  playerGetxpboosttime,
	"setXpBoostTime":                  playerSetxpboosttime,
	"getIdleTime":                     playerGetidletime,
	"getFreeBackpackSlots":            playerGetfreebackpackslots,
	"isOffline":                       playerIsoffline,
	"openMarket":                      playerOpenmarket,
	"instantSkillWOD":                 playerInstantskillwod,
	"upgradeSpellsWOD":                playerUpgradespellswod,
	"revelationStageWOD":              playerRevelationstagewod,
	"reloadData":                      playerReloaddata,
	"onThinkWheelOfDestiny":           playerOnthinkwheelofdestiny,
	"avatarTimer":                     playerAvatartimer,
	"getWheelSpellAdditionalArea":     playerGetwheelspelladditionalarea,
	"getWheelSpellAdditionalTarget":   playerGetwheelspelladditionaltarget,
	"getWheelSpellAdditionalDuration": playerGetwheelspelladditionalduration,
	"wheelUnlockScroll":               playerWheelunlockscroll,
	"openForge":                       playerOpenforge,
	"canFightBoss":                    playerCanfightboss,
	"setBossCooldown":                 playerSetbosscooldown,
	"getBossCooldown":                 playerGetbosscooldown,
	"closeForge":                      playerCloseforge,
	"addForgeDusts":                   playerAddforgedusts,
	"removeForgeDusts":                playerRemoveforgedusts,
	"getForgeDusts":                   playerGetforgedusts,
	"setForgeDusts":                   playerSetforgedusts,
	"addForgeDustLevel":               playerAddforgedustlevel,
	"removeForgeDustLevel":            playerRemoveforgedustlevel,
	"getForgeDustLevel":               playerGetforgedustlevel,
	"getForgeSlivers":                 playerGetforgeslivers,
	"getForgeCores":                   playerGetforgecores,
	"isUIExhausted":                   playerIsuiexhausted,
	"updateUIExhausted":               playerUpdateuiexhausted,
	"setFaction":                      playerSetfaction,
	"getFaction":                      playerGetfaction,
	"getBosstiaryLevel":               playerGetbosstiarylevel,
	"getBosstiaryKills":               playerGetbosstiarykills,
	"addBosstiaryKill":                playerAddbosstiarykill,
	"setBossPoints":                   playerSetbosspoints,
	"setRemoveBossTime":               playerSetremovebosstime,
	"getSlotBossId":                   playerGetslotbossid,
	"getBossBonus":                    playerGetbossbonus,
	"sendBosstiaryCooldownTimer":      playerSendbosstiarycooldowntimer,
	"sendSingleSoundEffect":           playerSendsinglesoundeffect,
	"sendDoubleSoundEffect":           playerSenddoublesoundeffect,
	"sendAmbientSoundEffect":          playerSendambientsoundeffect,
	"sendMusicSoundEffect":            playerSendmusicsoundeffect,
	"getName":                         playerGetname,
	"changeName":                      playerChangename,
	"hasGroupFlag":                    playerHasgroupflag,
	"setGroupFlag":                    playerSetgroupflag,
	"removeGroupFlag":                 playerRemovegroupflag,
	"setHazardSystemPoints":           playerSethazardsystempoints,
	"getHazardSystemPoints":           playerGethazardsystempoints,
	"setLoyaltyBonus":                 playerSetloyaltybonus,
	"getLoyaltyBonus":                 playerGetloyaltybonus,
	"getLoyaltyPoints":                playerGetloyaltypoints,
	"getLoyaltyTitle":                 playerGetloyaltytitle,
	"setLoyaltyTitle":                 playerSetloyaltytitle,
	"updateConcoction":                playerUpdateconcoction,
	"updateFood":                      playerUpdatefood,
	"feed":                            playerFeed,
	"clearSpellCooldowns":             playerClearspellcooldowns,
	"isVip":                           playerIsvip,
	"getVipDays":                      playerGetvipdays,
	"getVipTime":                      playerGetviptime,
	"kv":                              playerKv,
	"hasAchievement":                  playerHasachievement,
	"addAchievement":                  playerAddachievement,
	"removeAchievement":               playerRemoveachievement,
	"getAchievementPoints":            playerGetachievementpoints,
	"addAchievementPoints":            playerAddachievementpoints,
	"removeAchievementPoints":         playerRemoveachievementpoints,
	"addBadge":                        playerAddbadge,
	"addTitle":                        playerAddtitle,
	"getTitles":                       playerGettitles,
	"setCurrentTitle":                 playerSetcurrenttitle,
	"createTransactionSummary":        playerCreatetransactionsummary,
	"takeScreenshot":                  playerTakescreenshot,
	"sendIconBakragore":               playerSendiconbakragore,
	"removeIconBakragore":             playerRemoveiconbakragore,
	"sendCreatureAppear":              playerSendcreatureappear,
	"addAnimusMastery":                playerAddanimusmastery,
	"removeAnimusMastery":             playerRemoveanimusmastery,
	"hasAnimusMastery":                playerHasanimusmastery,
	"setSerene":                       playerSetserene,
	"getVirtue":                       playerGetvirtue,
	"setVirtue":                       playerSetvirtue,
	"fillHarmony":                     playerFillharmony,
	"getHarmony":                      playerGetharmony,
	"getHarmonyDamage":                playerGetharmonydamage,
	"calculateFlatDamageHealing":      playerCalculateflatdamagehealing,
	"setSpeed":                        playerSetspeed,
	"addWeaponExperience":             playerAddweaponexperience,
	"getLivestreamViewersCount":       playerGetlivestreamviewerscount,
	"getLivestreamViewers":            playerGetlivestreamviewers,
	"setLivestreamViewers":            playerSetlivestreamviewers,
	"isLivestreamViewer":              playerIslivestreamviewer,
	"getMapShader":                    playerGetmapshader,
	"setMapShader":                    playerSetmapshader,
	"removeCustomOutfit":              playerRemovecustomoutfit,
	"addCustomOutfit":                 playerAddcustomoutfit,
}

func playerAddachievement(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	name := L.CheckString(2)
	var reg *game.AchievementRegistry
	if p.World != nil {
		reg = p.World.Achievements
	}
	L.Push(lua.LBool(p.AddAchievementByName(reg, name)))
	return 1
}

func playerAddachievementpoints(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	// Achievement points are derived from unlocked achievements.
	// The player's points are recalculated automatically.
	L.Push(lua.LTrue)
	return 1
}

func playerAddanimusmastery(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	masteryID := uint8(L.CheckInt(2))
	if int(masteryID) >= len(p.AnimusMastery) {
		L.Push(lua.LFalse)
		return 1
	}
	p.AnimusMastery[masteryID] = 1
	L.Push(lua.LTrue)
	return 1
}

func playerAddbadge(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	badgeID := uint16(L.CheckInt(2))
	badges := p.GetBadges()
	if badges != nil {
		badges.UnlockBadge(uint32(badgeID))
	}
	L.Push(lua.LTrue)
	return 1
}

func playerAddbestiarykill(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	raceID := uint16(L.CheckInt(2))
	amount := luaOptInt(L, 3)
	if amount <= 0 {
		amount = 1
	}
	p.AddBestiaryKillCount(raceID, uint32(amount))
	L.Push(lua.LTrue)
	return 1
}

func playerAddblessing(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	blessing := luaOptInt(L, 2)
	count := luaOptInt(L, 3)
	if count <= 0 {
		count = 1
	}
	if blessing >= 1 && blessing <= 8 {
		p.Blessings[blessing-1] = uint8(count)
		L.Push(lua.LTrue)
		return 1
	}
	L.Push(lua.LFalse)
	return 1
}

func playerAddbosstiarykill(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	raceID := uint16(L.CheckInt(2))
	amount := luaOptInt(L, 3)
	if amount <= 0 {
		amount = 1
	}
	p.AddBosstiaryKill(raceID, 0, uint32(amount))
	L.Push(lua.LTrue)
	return 1
}

func playerAddcharmpoints(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	points := uint32(luaOptInt(L, 2))
	p.AddCharmPoints(points)
	L.Push(lua.LNumber(p.GetCharmPoints()))
	return 1
}

func playerAddcustomoutfit(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	outfitID := uint16(L.CheckInt(2))
	p.AddOutfit(outfitID, 0)
	L.Push(lua.LTrue)
	return 1
}

func playerAddexperience(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	exp := int64(luaOptInt(L, 2))
	game.GlobalDispatcher.AddEvent(0, func() {
		if exp > 0 {
			p.AddExperience(uint64(exp))
		}
	})
	L.Push(lua.LTrue)
	return 1
}

func playerAddAchievementProgress(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	name := L.CheckString(2)
	var reg *game.AchievementRegistry
	if p.World != nil {
		reg = p.World.Achievements
	}
	L.Push(lua.LBool(p.AddAchievementByName(reg, name)))
	return 1
}

func playerAddfamiliar(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	lookType := uint16(L.CheckInt(2))
	L.Push(lua.LBool(p.AddFamiliar(lookType)))
	return 1
}

func playerAddforgedustlevel(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	p.AddForgeDustLevel(uint16(L.CheckInt(2)))
	L.Push(lua.LTrue)
	return 1
}

func playerAddforgedusts(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	p.AddForgeDusts(uint64(L.CheckInt(2)))
	L.Push(lua.LTrue)
	return 1
}

func playerAdditem(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	id := uint16(L.CheckInt(2))
	count := uint8(L.OptInt(3, 1))

	p.AddItem(id, uint64(count))
	// AddItem currently does not return the created item.
	// Scripts relying on the returned item will get nil, which is safe for most basic usage.
	L.Push(lua.LNil)
	return 1
}

func playerAdditembatchtopaginedcontainer(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerAdditemex(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerAdditemstash(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	itemID := uint16(L.CheckInt(2))
	count := uint32(luaOptInt(L, 3))
	if count <= 0 {
		count = 1
	}
	p.Stash[itemID] += count
	L.Push(lua.LTrue)
	return 1
}

func playerAddmana(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	amount := int32(L.CheckNumber(2))
	p.AddMana(amount)
	return 0
}

func playerAddmanaspent(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	amount := uint64(luaOptInt(L, 2))
	game.GlobalDispatcher.AddEvent(0, func() {
		p.AddManaSpent(amount)
	})
	L.Push(lua.LTrue)
	return 1
}

func playerAddmapmark(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerAddminorcharmechoes(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	echoes := uint32(luaOptInt(L, 2))
	p.AddMinorCharmEchoes(echoes, false)
	L.Push(lua.LNumber(p.GetMinorCharmEchoes()))
	return 1
}

func playerAddmoney(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	amount := uint64(L.CheckNumber(2))
	// C++ returns three values: success(bool), addedMoney(int), returnValue(int).
	// We always fully add, mirroring Game::addMoney's success path.
	if amount == 0 {
		L.Push(lua.LTrue)
		L.Push(lua.LNumber(0))
		L.Push(lua.LNumber(0)) // RETURNVALUE_NOERROR
		return 3
	}
	p.AddMoney(amount)
	L.Push(lua.LTrue)
	L.Push(lua.LNumber(amount))
	L.Push(lua.LNumber(0)) // RETURNVALUE_NOERROR
	return 3
}

func playerAddmount(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LFalse)
		return 1
	}
	mountID := uint16(L.CheckInt(2))
	p.AddMount(mountID)
	L.Push(lua.LTrue)
	return 1
}

func playerAddofflinetrainingtime(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	timeVal := int32(luaOptInt(L, 2))
	p.OfflineTrainingTime += timeVal
	if p.OfflineTrainingTime > 43200000 {
		p.OfflineTrainingTime = 43200000
	}
	L.Push(lua.LTrue)
	return 1
}

func playerAddofflinetrainingtries(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	luaSkill := luaOptInt(L, 2)
	triesVal := uint64(luaOptInt(L, 3))

	skillVal, isMagic, ok := mapLuaSkillToGo(luaSkill)
	if ok {
		game.GlobalDispatcher.AddEvent(0, func() {
			if isMagic {
				p.AddManaSpent(triesVal)
			} else {
				p.AddSkillTries(skillVal, triesVal)
			}
		})
	}

	L.Push(lua.LTrue)
	return 1
}

func playerAddoutfit(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LBool(false))
		return 1
	}
	lookType := uint16(L.CheckInt(2))
	addons := uint8(L.OptInt(3, 0))
	p.AddOutfit(lookType, addons)
	L.Push(lua.LBool(true))
	return 1
}

func playerAddoutfitaddon(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LBool(false))
		return 1
	}
	lookType := uint16(L.CheckInt(2))
	addon := uint8(L.CheckInt(3))
	p.AddOutfit(lookType, addon)
	L.Push(lua.LBool(true))
	return 1
}

func playerAddpremiumdays(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerAddpreycards(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	cards := uint32(luaOptInt(L, 2))
	p.PreyCards += cards
	L.Push(lua.LNumber(p.PreyCards))
	return 1
}

func playerAddskilltries(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	luaSkill := luaOptInt(L, 2)
	triesVal := uint64(luaOptInt(L, 3))

	skillVal, isMagic, ok := mapLuaSkillToGo(luaSkill)
	if ok {
		game.GlobalDispatcher.AddEvent(0, func() {
			if isMagic {
				p.AddManaSpent(triesVal)
			} else {
				p.AddSkillTries(skillVal, triesVal)
			}
		})
	}

	L.Push(lua.LTrue)
	return 1
}

func playerAddsoul(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	delta := luaOptInt(L, 2)
	game.GlobalDispatcher.AddEvent(0, func() {
		v := int(p.Soul) + delta
		if v < 0 {
			v = 0
		}
		if v > 255 {
			v = 255
		}
		p.Soul = uint8(v)
	})
	L.Push(lua.LTrue)
	return 1
}

func playerAddtaskhuntingpoints(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerAddtibiacoins(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	p.CoinBalance += uint32(L.CheckInt(2))
	L.Push(lua.LTrue)
	return 1
}

func playerAddtitle(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	title := L.CheckString(2)
	for _, t := range p.TitleStrings {
		if t == title {
			L.Push(lua.LFalse)
			return 1
		}
	}
	p.TitleStrings = append(p.TitleStrings, title)
	L.Push(lua.LTrue)
	return 1
}

func playerAddtransferablecoins(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	coins := uint32(luaOptInt(L, 2))
	p.CoinTransferable += coins
	L.Push(lua.LNumber(p.CoinTransferable))
	return 1
}

func playerAddweaponexperience(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerAvatartimer(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerCalculateflatdamagehealing(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerCancast(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	// canCast := combat.CastSpell(p, spellName)
	L.Push(lua.LBool(true))
	return 1
}

func playerCanlearnspell(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	spellName := L.CheckString(2)
	L.Push(lua.LBool(!p.HasLearnedSpell(spellName)))
	return 1
}

func playerCanreceiveloot(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LFalse)
		return 1
	}
	L.Push(lua.LTrue)
	return 1
}

func playerChangename(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	newName := L.CheckString(2)
	p.Name = newName
	L.Push(lua.LTrue)
	return 1
}

func playerChannelsay(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	channelID := uint16(L.CheckInt(2))
	message := L.CheckString(3)
	if p.Session != nil {
		p.Session.SendToChannel(0, p.Name, p.Level, 0x01, channelID, message)
	}
	L.Push(lua.LTrue)
	return 1
}

func playerCharmexpansion(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LBool(p.CharmExpansion))
	return 1
}

func playerClearallimbuements(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	for _, item := range p.Inventory {
		if item != nil {
			item.ClearImbuement(0)
			item.ClearImbuement(1)
		}
	}
	L.Push(lua.LTrue)
	return 1
}

func playerClearspellcooldowns(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	p.Cooldowns().AddCooldown(0, 0)
	L.Push(lua.LTrue)
	return 1
}

func playerCloseforge(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerCloseimbuementwindow(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerCreatetransactionsummary(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerFillharmony(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerForgetspell(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerGetaccountid(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.AccountID))
	return 1
}

func playerGetaccounttype(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetachievementpoints(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetbackpack(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	if p.Inventory[3] != nil {
		ud := L.NewUserData()
		ud.Value = p.Inventory[3]
		L.Push(ud)
		return 1
	}
	L.Push(lua.LNil)
	return 1
}

func playerGetbankbalance(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.BankBalance))
	return 1
}

// playerRemovemoneybank removes cost from the bank balance (NPC travel/bank
// flows use removeMoneyBank). Returns false without deducting when the balance
// is insufficient.
func playerRemovemoneybank(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LFalse)
		return 1
	}
	cost := L.CheckNumber(2)
	if cost <= 0 {
		L.Push(lua.LTrue)
		return 1
	}
	// Pay from inventory first, then the bank (mirrors the Lua removeMoneyBank).
	L.Push(lua.LBool(p.RemoveMoney(uint64(cost), true)))
	return 1
}

func playerDepositmoney(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LFalse)
		return 1
	}
	// Move inventory cash into the bank (never credit without debiting).
	// Normally shadowed by the Lua Bank.deposit wrapper; kept correct so it
	// can't create money if the shadow ever goes away.
	amount := uint64(L.CheckNumber(2))
	if amount == 0 {
		L.Push(lua.LTrue)
		return 1
	}
	if !p.RemoveMoney(amount, false) {
		L.Push(lua.LFalse)
		return 1
	}
	p.BankBalance += amount
	L.Push(lua.LTrue)
	return 1
}

func playerWithdrawmoney(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LFalse)
		return 1
	}
	amount := L.CheckNumber(2)
	if amount <= 0 {
		L.Push(lua.LTrue)
		return 1
	}
	// Debit the bank and credit inventory coins (never debit without crediting).
	if p.BankBalance < uint64(amount) {
		L.Push(lua.LFalse)
		return 1
	}
	p.BankBalance -= uint64(amount)
	p.AddMoney(uint64(amount))
	L.Push(lua.LTrue)
	return 1
}

func playerGetbasemagiclevel(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.MagLevel))
	return 1
}

func playerGetbasemaxhealth(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.MaxHealth))
	return 1
}

func playerGetbasemaxmana(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.MaxMana))
	return 1
}

func playerGetbasexpgain(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetblessingcount(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	var total int
	for _, b := range p.Blessings {
		if b > 0 {
			total++
		}
	}
	L.Push(lua.LNumber(total))
	return 1
}

func playerGetbossbonus(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetbosstiarykills(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetbosstiarylevel(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetcapacity(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.GetCapacity()))
	return 1
}

func playerGetcharmchance(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetcharmmonstertype(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNil)
	return 1
}

func playerGetcharmtier(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetclient(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	// Return a client info table. Scripts read `.version` to branch protocol
	// behaviour (e.g. the gamestore module's openStore/parseRequestStoreOffers
	// serialize different fields per version). This MUST match the version the
	// client actually negotiated, or the client desyncs and drops packets — the
	// game handshake enforces protocol 1525 (see protocol.ClientVersion), so
	// report that. os=2 is the standard "new client" (CIPSOFT/OTClientV8) flag.
	t := L.NewTable()
	L.SetField(t, "version", lua.LNumber(1525))
	L.SetField(t, "os", lua.LNumber(2))
	L.Push(t)
	return 1
}

func playerGetcontainerbyid(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	containerID := uint8(L.CheckInt(2))
	if itemID, ok := p.ManagedContainers[containerID]; ok {
		ud := L.NewUserData()
		ud.Value = itemID
		L.Push(ud)
		return 1
	}
	L.Push(lua.LNil)
	return 1
}

func playerGetcontainerid(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetcontainerindex(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetdeathpenalty(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.GetDeathPenalty()))
	return 1
}

func playerGetdepotchest(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	depotID := uint16(L.CheckInt(2))
	if item, ok := p.DepotLockers[depotID]; ok {
		ud := L.NewUserData()
		ud.Value = item
		L.Push(ud)
		return 1
	}
	L.Push(lua.LNil)
	return 1
}

func playerGetdepotlocker(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	depotID := uint16(L.CheckInt(2))
	if item, ok := p.DepotLockers[depotID]; ok {
		ud := L.NewUserData()
		ud.Value = item
		L.Push(ud)
		return 1
	}
	L.Push(lua.LNil)
	return 1
}

func playerGeteffectiveskilllevel(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	skill := luaOptInt(L, 2)
	if skill < 0 || skill >= int(game.SkillCount) {
		L.Push(lua.LNumber(0))
		return 1
	}
	L.Push(lua.LNumber(p.Skills[skill]))
	return 1
}

func playerGetexperience(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.Experience))
	return 1
}

func playerGetfaction(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetfamiliarlooktype(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.GetFamiliarLooktype()))
	return 1
}

func playerGetfightmode(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.FightMode))
	return 1
}

func playerGetforgecores(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	// Cores are inventory items; this free binding has no catalog handle, so it
	// reports 0. The core forge flow reads the real count via the protocol layer.
	L.Push(lua.LNumber(0))
	return 1
}

func playerGetforgedustlevel(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.GetForgeDustLevel()))
	return 1
}

func playerGetforgedusts(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.GetForgeDusts()))
	return 1
}

func playerGetforgeslivers(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	// Slivers are inventory items; this free binding has no catalog handle, so it
	// reports 0. The core forge flow reads the real count via the protocol layer.
	L.Push(lua.LNumber(0))
	return 1
}

func playerGetfreebackpackslots(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetfreecapacity(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.GetFreeCapacity()))
	return 1
}

func playerGetgrindingxpboost(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetgroup(L *lua.LState) int {
	p := checkPlayer(L)
	groupID := uint32(1)
	if p != nil && p.GroupID != 0 {
		groupID = uint32(p.GroupID)
	}
	tbl := L.NewTable()
	L.SetField(tbl, "getId", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(groupID))
		return 1
	}))
	L.SetField(tbl, "getName", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString("Player"))
		return 1
	}))
	L.SetField(tbl, "getFlags", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(0))
		return 1
	}))
	L.SetField(tbl, "getAccess", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LFalse)
		return 1
	}))
	L.Push(tbl)
	return 1
}

func playerGetguid(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.DBID))
	return 1
}

func playerGetguild(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	if p.GuildName == "" {
		L.Push(lua.LNil)
		return 1
	}
	tbl := L.NewTable()
	L.SetField(tbl, "name", lua.LString(p.GuildName))
	L.SetField(tbl, "rank", lua.LString(p.GuildRankName))
	L.Push(tbl)
	return 1
}

func playerGetguildlevel(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetguildnick(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LString(p.GuildNick))
	return 1
}

func playerGetharmony(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetharmonydamage(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGethazardsystempoints(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGethouse(L *lua.LState) int {
	p := checkPlayerArg(L, 1)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	// Find the player's house by iterating all houses
	houses := p.GetWorld().AllHouses()
	for _, h := range houses {
		if h.OwnerID == p.DBID {
			ud := L.NewUserData()
			ud.Value = h
			L.SetMetatable(ud, L.GetTypeMetatable(houseTypeName))
			L.Push(ud)
			return 1
		}
	}
	L.Push(lua.LNil)
	return 1
}

func playerGetidletime(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func (e *Engine) playerGetinbox(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	if p.Inbox == nil {
		p.Inbox = &game.Item{ID: game.ItemInbox, Contents: make([]*game.Item, 0), Pagination: true}
	}
	e.pushContainer(L, p.Inbox)
	return 1
}

func playerGetinstantspells(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(L.NewTable())
		return 1
	}
	tbl := L.NewTable()
	i := 1
	for spell := range p.GetLearnedSpells() {
		L.RawSetInt(tbl, i, lua.LString(spell))
		i++
	}
	L.Push(tbl)
	return 1
}

func playerGetip(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetitembyid(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	itemID := uint16(L.CheckInt(2))
	for _, item := range p.Inventory {
		if item != nil && item.ID == itemID {
			ud := L.NewUserData()
			ud.Value = item
			L.Push(ud)
			return 1
		}
	}
	L.Push(lua.LNil)
	return 1
}

func playerGetitemcount(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetkills(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(L.NewTable())
		return 1
	}
	tbl := L.NewTable()
	L.Push(tbl)
	return 1
}

func playerGetlastloginsaved(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.LastLogin))
	return 1
}

func playerGetlastlogout(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.LastLogout))
	return 1
}

func playerGetlevel(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.Level))
	return 1
}

func playerGetlivestreamviewers(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(L.NewTable())
		return 1
	}
	tbl := L.NewTable()
	L.Push(tbl)
	return 1
}

func playerGetlivestreamviewerscount(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func (e *Engine) playerGetlootpouch(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	// C++: Player::getLootPouch — search entire inventory tree for ITEM_GOLD_POUCH
	// (getInventoryItemsFromId + getContainer)
	for _, item := range p.Inventory {
		if item == nil {
			continue
		}
		if item.ID == game.ItemGoldPouch {
			e.pushContainer(L, item)
			return 1
		}
		if found := findItemInTree(item, game.ItemGoldPouch); found != nil {
			e.pushContainer(L, found)
			return 1
		}
	}
	L.Push(lua.LNil)
	return 1
}

// findItemInTree recursively searches an item's contents for a matching ID.
func findItemInTree(parent *game.Item, id uint16) *game.Item {
	if parent == nil {
		return nil
	}
	for _, child := range parent.Contents {
		if child == nil {
			continue
		}
		if child.ID == id {
			return child
		}
		if found := findItemInTree(child, id); found != nil {
			return found
		}
	}
	return nil
}

func playerGetloyaltybonus(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetloyaltypoints(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetloyaltytitle(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LString(""))
	return 1
}

func playerGetmagiclevel(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.MagLevel))
	return 1
}

func playerGetmagicshieldcapacityflat(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetmagicshieldcapacitypercent(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetmana(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.GetMana()))
	return 1
}

func playerGetmanaspent(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.ManaSpent))
	return 1
}

func playerGetmapshader(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LString(""))
	return 1
}

func playerGetmaxmana(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.GetMaxMana()))
	return 1
}

func playerGetmaxsoul(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetmoney(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.GetMoney()))
	return 1
}

func playerGetname(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LString(p.GetName()))
	return 1
}

func playerGetofflinetrainingskill(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.OfflineTrainingSkill))
	return 1
}

func playerGetofflinetrainingtime(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.OfflineTrainingTime))
	return 1
}

func (e *Engine) playerGetparty(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil || p.Party == nil {
		L.Push(lua.LNil)
		return 1
	}
	e.pushParty(L, p.Party)
	return 1
}

func playerGetpremiumdays(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNumber(0))
		return 1
	}
	// Return premium days from account. Default 0 if not available.
	premDays := int32(0)
	_ = premDays
	L.Push(lua.LNumber(0))
	return 1
}

func playerGetpreycards(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetpreyexperiencepercentage(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetpreylootpercentage(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetpronoun(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetreward(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	if p.RewardChest != nil {
		ud := L.NewUserData()
		ud.Value = p.RewardChest
		L.Push(ud)
		return 1
	}
	L.Push(lua.LNil)
	return 1
}

func playerGetrewardlist(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(L.NewTable())
		return 1
	}
	tbl := L.NewTable()
	L.Push(tbl)
	return 1
}

func playerGetsex(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.Sex))
	return 1
}

func playerGetskilllevel(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	skill := luaOptInt(L, 2)
	if skill < 0 || skill >= int(game.SkillCount) {
		L.Push(lua.LNumber(0))
		return 1
	}
	L.Push(lua.LNumber(p.Skills[skill]))
	return 1
}

func playerGetskillpercent(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	skill := luaOptInt(L, 2)
	if skill < 0 || skill >= int(game.SkillCount) {
		L.Push(lua.LNumber(0))
		return 1
	}
	pct := float64(p.GetSkillPercent(game.Skill(skill))) / 100.0
	L.Push(lua.LNumber(pct))
	return 1
}

func playerGetmagiclevelpercent(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	pct := float64(p.GetMagLevelPercent()) / 100.0
	L.Push(lua.LNumber(pct))
	return 1
}

func playerGetskilltries(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	skill := luaOptInt(L, 2)
	if skill < 0 || skill >= int(game.SkillCount) {
		L.Push(lua.LNumber(0))
		return 1
	}
	L.Push(lua.LNumber(p.SkillTries[skill]))
	return 1
}

func playerGetskulltime(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetslotbossid(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetslotitem(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	slot := luaOptInt(L, 2)
	if slot < 1 || slot >= len(p.Inventory) {
		L.Push(lua.LNil)
		return 1
	}
	it := p.Inventory[slot]
	if it == nil {
		L.Push(lua.LNil)
		return 1
	}
	ud := L.NewUserData()
	ud.Value = luaItem{item: it}
	L.SetMetatable(ud, L.GetTypeMetatable(itemTypeName))
	L.Push(ud)
	return 1
}

func playerGetsoul(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.Soul))
	return 1
}

func playerGetstamina(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetstaminaxpboost(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetstashcount(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetstashitemcount(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetstoragevalue(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.GetStorageValue(uint32(luaOptInt(L, 2)))))
	return 1
}

// storeInboxItemID is ITEM_STORE_INBOX (src/utils/utils_definitions.hpp): the
// container in-game store purchases are delivered to.
const storeInboxItemID = 23396

// playerGetstoreinbox returns the player's Store Inbox as a real Container so
// the datapack's Player:addItemStoreInbox can add purchased items to it
// (inbox:addItemEx). The inbox is created lazily and held on the player.
func (e *Engine) playerGetstoreinbox(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	if p.StoreInbox == nil {
		p.StoreInbox = &game.Item{ID: storeInboxItemID}
	}
	e.pushContainer(L, p.StoreInbox)
	return 1
}

func playerGettaskhuntingpoints(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGettibiacoins(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNumber(0))
		return 1
	}
	L.Push(lua.LNumber(p.CoinBalance))
	return 1
}

func playerGettitles(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	tbl := L.NewTable()
	for _, title := range p.TitleStrings {
		tbl.Append(lua.LString(title))
	}
	L.Push(tbl)
	return 1
}

func playerGettown(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.TownID))
	return 1
}

func playerGettransferablecoins(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNumber(0))
		return 1
	}
	L.Push(lua.LNumber(p.CoinTransferable))
	return 1
}

func playerGetvipdays(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetviptime(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetvirtue(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetvocation(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	voc := vocations.GetVocation(uint32(p.Vocation))
	if voc == nil {
		// Fall back to a default vocation so callers (e.g. Player.feed, which
		// needs the HP/mana gain rates) still work for characters whose vocation
		// isn't in the registry. Defaults mirror the "None" vocation regen.
		voc = &vocations.Vocation{
			ID: uint32(p.Vocation), Name: "None",
			GainHPAmount: 1, GainHPTicks: 6, GainManaAmount: 1, GainManaTicks: 6,
			AttackSpeed: 2000, BaseSpeed: 220,
		}
	}
	pushVocation(L, voc)
	return 1
}

func playerIspremium(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LFalse)
		return 1
	}
	L.Push(lua.LTrue)
	return 1
}

func playerGetfinalbaserateexperience(L *lua.LState) int {
	L.Push(lua.LNumber(1.0))
	return 1
}

func playerGetvoucherxpboost(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetwheelspelladditionalarea(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(0))
	return 1
}

func playerGetwheelspelladditionalduration(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetwheelspelladditionaltarget(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetxpboostpercent(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetxpboosttime(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerHasachievement(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	name := L.CheckString(2)
	var reg *game.AchievementRegistry
	if p.World != nil {
		reg = p.World.Achievements
	}
	L.Push(lua.LBool(p.HasAchievementByName(reg, name)))
	return 1
}

func playerHasanimusmastery(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	masteryID := uint8(L.CheckInt(2))
	if int(masteryID) < len(p.AnimusMastery) {
		L.Push(lua.LBool(p.AnimusMastery[masteryID] > 0))
		return 1
	}
	L.Push(lua.LFalse)
	return 1
}

func playerHasblessing(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	blessing := luaOptInt(L, 2)
	if blessing >= 1 && blessing <= 8 {
		L.Push(lua.LBool(p.Blessings[blessing-1] > 0))
		return 1
	}
	L.Push(lua.LFalse)
	return 1
}

func playerHaschasemode(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LBool(p.ChaseMode))
	return 1
}

func playerHasfamiliar(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	lookType := uint16(L.CheckInt(2))
	L.Push(lua.LBool(p.HasFamiliar(lookType)))
	return 1
}

func playerHasgroupflag(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LBool(p.GroupID > 0))
	return 1
}

func playerHaslearnedspell(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LFalse)
		return 1
	}
	L.Push(lua.LBool(p.HasLearnedSpell(L.CheckString(2))))
	return 1
}

func playerHasmount(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LFalse)
		return 1
	}
	mountID := uint16(L.CheckInt(2))
	L.Push(lua.LBool(p.HasMount(mountID)))
	return 1
}

func playerHasoutfit(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LBool(false))
		return 1
	}
	lookType := uint16(L.CheckInt(2))
	addon := uint8(L.OptInt(3, 0))
	
	if !p.HasOutfit(lookType) {
		L.Push(lua.LBool(false))
		return 1
	}
	
	if addon > 0 {
		currentAddons := p.GetOutfitAddons(lookType)
		L.Push(lua.LBool((currentAddons & addon) == addon))
		return 1
	}
	
	L.Push(lua.LBool(true))
	return 1
}

func playerHassecuremode(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LBool(p.SecureMode))
	return 1
}

func playerInstantskillwod(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LFalse)
	return 1
}

func playerIslivestreamviewer(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LFalse)
	return 1
}

func playerIsmonsterbestiaryunlocked(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	raceID := uint16(L.CheckInt(2))
	kills := p.GetBestiaryKillCount(raceID)
	L.Push(lua.LBool(kills > 0))
	return 1
}

func playerIsmonsterprey(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	if p.Prey != nil {
		for _, slot := range p.Prey.Slots {
			if slot != nil && slot.State == 2 && slot.SelectedRaceID == uint16(L.CheckInt(2)) {
				L.Push(lua.LTrue)
				return 1
			}
		}
	}
	L.Push(lua.LFalse)
	return 1
}

func playerIsoffline(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LBool(p.Session == nil))
	return 1
}

func playerIsplayer(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LFalse)
		return 1
	}
	L.Push(lua.LTrue)
	return 1
}

func playerIspromoted(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LBool(p.Vocation > 4 && p.Vocation <= 8))
	return 1
}

func playerIspzlocked(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LFalse)
	return 1
}

func playerIstraining(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	if p.IsTraining {
		L.Push(lua.LNumber(1))
	} else {
		L.Push(lua.LNumber(0))
	}
	return 1
}

func playerIsuiexhausted(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LFalse)
	return 1
}

func playerIsvip(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LFalse)
	return 1
}

func playerKv(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	kv := &LuaKVStore{
		Player: p,
		Scope:  []string{},
	}
	ud := L.NewUserData()
	ud.Value = kv
	L.SetMetatable(ud, L.GetTypeMetatable("LuaKVStore"))
	L.Push(ud)
	return 1
}

var (
	globalKVStore   = make(map[string]any)
	globalKVStoreMu sync.RWMutex
)

type LuaKVStore struct {
	Player *game.Player
	Scope  []string
}

func checkKVStore(L *lua.LState) *LuaKVStore {
	ud := L.CheckUserData(1)
	if v, ok := ud.Value.(*LuaKVStore); ok {
		return v
	}
	L.ArgError(1, "LuaKVStore expected")
	return nil
}

func (e *Engine) registerKVStoreType() {
	mt := e.L.NewTypeMetatable("LuaKVStore")
	methods := map[string]lua.LGFunction{
		"get":    kvStoreGet,
		"set":    kvStoreSet,
		"remove": kvStoreRemove,
		"scoped": kvStoreScoped,
	}
	e.L.SetField(mt, "__index", e.L.SetFuncs(e.L.NewTable(), methods))

	// Register global KV variable matching C++ global KV instance
	gKv := &LuaKVStore{
		Player: nil,
		Scope:  nil,
	}
	ud := e.L.NewUserData()
	ud.Value = gKv
	e.L.SetMetatable(ud, mt)
	e.L.SetGlobal("KV", ud)
	e.L.SetGlobal("kv", ud)
}

func kvStoreGet(L *lua.LState) int {
	var kv *LuaKVStore
	var key string

	if ud, ok := L.Get(1).(*lua.LUserData); ok {
		if k, ok := ud.Value.(*LuaKVStore); ok {
			kv = k
			key = L.CheckString(2)
		}
	}

	if kv == nil {
		kv = &LuaKVStore{
			Player: nil,
			Scope:  nil,
		}
		key = L.CheckString(1)
	}

	fullKey := key
	if len(kv.Scope) > 0 {
		fullKey = strings.Join(kv.Scope, ".") + "." + key
	}

	var val any
	var exists bool

	if kv.Player != nil {
		if kv.Player.KVStore == nil {
			kv.Player.KVStore = make(map[string]any)
		}
		val, exists = kv.Player.KVStore[fullKey]
	} else {
		globalKVStoreMu.RLock()
		val, exists = globalKVStore[fullKey]
		globalKVStoreMu.RUnlock()
	}

	if !exists {
		L.Push(lua.LNil)
		return 1
	}

	switch v := val.(type) {
	case string:
		L.Push(lua.LString(v))
	case int:
		L.Push(lua.LNumber(v))
	case int32:
		L.Push(lua.LNumber(v))
	case int64:
		L.Push(lua.LNumber(v))
	case uint32:
		L.Push(lua.LNumber(v))
	case uint64:
		L.Push(lua.LNumber(v))
	case float64:
		L.Push(lua.LNumber(v))
	case bool:
		L.Push(lua.LBool(v))
	default:
		L.Push(lua.LNil)
	}
	return 1
}

func kvStoreSet(L *lua.LState) int {
	var kv *LuaKVStore
	var key string
	var val lua.LValue

	if ud, ok := L.Get(1).(*lua.LUserData); ok {
		if k, ok := ud.Value.(*LuaKVStore); ok {
			kv = k
			key = L.CheckString(2)
			val = L.Get(3)
		}
	}

	if kv == nil {
		kv = &LuaKVStore{
			Player: nil,
			Scope:  nil,
		}
		key = L.CheckString(1)
		val = L.Get(2)
	}

	fullKey := key
	if len(kv.Scope) > 0 {
		fullKey = strings.Join(kv.Scope, ".") + "." + key
	}

	if kv.Player != nil {
		if kv.Player.KVStore == nil {
			kv.Player.KVStore = make(map[string]any)
		}
		if val == lua.LNil {
			delete(kv.Player.KVStore, fullKey)
		} else {
			switch v := val.(type) {
			case lua.LString:
				kv.Player.KVStore[fullKey] = string(v)
			case lua.LNumber:
				kv.Player.KVStore[fullKey] = float64(v)
			case lua.LBool:
				kv.Player.KVStore[fullKey] = bool(v)
			default:
				// ignore unsupported complex types for now
			}
		}
	} else {
		globalKVStoreMu.Lock()
		if val == lua.LNil {
			delete(globalKVStore, fullKey)
		} else {
			switch v := val.(type) {
			case lua.LString:
				globalKVStore[fullKey] = string(v)
			case lua.LNumber:
				globalKVStore[fullKey] = float64(v)
			case lua.LBool:
				globalKVStore[fullKey] = bool(v)
			default:
				// ignore unsupported complex types for now
			}
		}
		globalKVStoreMu.Unlock()
	}
	L.Push(lua.LTrue)
	return 1
}

func kvStoreRemove(L *lua.LState) int {
	var kv *LuaKVStore
	var key string

	if ud, ok := L.Get(1).(*lua.LUserData); ok {
		if k, ok := ud.Value.(*LuaKVStore); ok {
			kv = k
			key = L.CheckString(2)
		}
	}

	if kv == nil {
		kv = &LuaKVStore{
			Player: nil,
			Scope:  nil,
		}
		key = L.CheckString(1)
	}

	fullKey := key
	if len(kv.Scope) > 0 {
		fullKey = strings.Join(kv.Scope, ".") + "." + key
	}

	if kv.Player != nil {
		if kv.Player.KVStore != nil {
			delete(kv.Player.KVStore, fullKey)
		}
	} else {
		globalKVStoreMu.Lock()
		delete(globalKVStore, fullKey)
		globalKVStoreMu.Unlock()
	}
	L.Push(lua.LTrue)
	return 1
}

func kvStoreScoped(L *lua.LState) int {
	var kv *LuaKVStore
	var scopeName string

	if ud, ok := L.Get(1).(*lua.LUserData); ok {
		if k, ok := ud.Value.(*LuaKVStore); ok {
			kv = k
			scopeName = L.CheckString(2)
		}
	}

	if kv == nil {
		kv = &LuaKVStore{
			Player: nil,
			Scope:  nil,
		}
		scopeName = L.CheckString(1)
	}

	newScope := append([]string{}, kv.Scope...)
	newScope = append(newScope, scopeName)

	newKv := &LuaKVStore{
		Player: kv.Player,
		Scope:  newScope,
	}
	ud := L.NewUserData()
	ud.Value = newKv
	L.SetMetatable(ud, L.GetTypeMetatable("LuaKVStore"))
	L.Push(ud)
	return 1
}

func playerLearnspell(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	p.LearnSpell(L.CheckString(2))
	L.Push(lua.LTrue)
	return 1
}

func playerOnthinkwheelofdestiny(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerOpenchannel(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	channelID := uint16(L.CheckInt(2))
	if p.Session != nil {
		p.Session.SendToChannel(0, p.Name, p.Level, 0x01, channelID, "")
	}
	L.Push(lua.LTrue)
	return 1
}

// playerCanreceivestoreitems reports whether the player can receive store items
// right now (C++ checks capacity/house/pz; a lenient default of true is used
// until those constraints are modeled).
func playerCanreceivestoreitems(L *lua.LState) int {
	L.Push(lua.LTrue)
	return 1
}

// playerSendbuttonindication is the store button blink hint; a no-op here.
func playerSendbuttonindication(L *lua.LState) int {
	L.Push(lua.LTrue)
	return 1
}

func playerOpenforge(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	if p.Session != nil {
		p.Session.SendOpenForge()
	}
	L.Push(lua.LTrue)
	return 1
}

// playerCanfightboss mirrors Player:canFightBoss(name) — true when the boss
// fight cooldown has elapsed.
func playerCanfightboss(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LFalse)
		return 1
	}
	L.Push(lua.LBool(p.CanFightBoss(L.CheckString(2), time.Now().Unix())))
	return 1
}

// playerSetbosscooldown mirrors Player:setBossCooldown(name, timestamp).
func playerSetbosscooldown(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	p.SetBossCooldown(L.CheckString(2), int64(L.CheckNumber(3)))
	L.Push(lua.LTrue)
	return 1
}

// playerGetbosscooldown mirrors Player:getBossCooldown(name).
func playerGetbosscooldown(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNumber(0))
		return 1
	}
	L.Push(lua.LNumber(p.GetBossCooldown(L.CheckString(2))))
	return 1
}

func playerOpenimbuementwindow(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil || p.Session == nil {
		return 0
	}

	action := game.ImbuementActionOpen
	if L.GetTop() >= 2 {
		action = game.ImbuementAction(luaOptInt(L, 2))
	}

	if session, ok := p.Session.(interface{ SendImbuementWindow(game.ImbuementAction, *game.Item) }); ok {
		session.SendImbuementWindow(action, nil)
	}

	return 0
}

func playerOpenmarket(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	if session, ok := p.Session.(interface{ SendOpenMarket() }); ok {
		session.SendOpenMarket()
	}
	L.Push(lua.LTrue)
	return 1
}

func playerOpenstash(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	if p.Session != nil {
		p.Session.SendOpenStash()
	}
	return 0
}

func playerPopupfyi(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	text := L.CheckString(2)
	p.SendFYIBox(text)
	L.Push(lua.LTrue)
	return 1
}

func playerPreythirdslot(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LFalse)
	return 1
}

func playerReloaddata(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerRemoveachievement(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerRemoveachievementpoints(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerRemoveanimusmastery(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerRemoveblessing(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	blessing := luaOptInt(L, 2)
	if blessing >= 1 && blessing <= 8 {
		p.Blessings[blessing-1] = 0
		L.Push(lua.LTrue)
		return 1
	}
	L.Push(lua.LFalse)
	return 1
}

func playerRemovecustomoutfit(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerRemoveexperience(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	exp := uint64(luaOptInt(L, 2))
	game.GlobalDispatcher.AddEvent(0, func() {
		p.RemoveExperience(exp) // subtracts and recomputes level downward
		if p.Session != nil {
			p.Session.SendStats()
		}
	})
	L.Push(lua.LTrue)
	return 1
}

func playerRemovefamiliar(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	lookType := uint16(L.CheckInt(2))
	L.Push(lua.LBool(p.RemoveFamiliar(lookType)))
	return 1
}

func playerRemoveforgedustlevel(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerRemoveforgedusts(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LBool(p.RemoveForgeDusts(uint64(L.CheckInt(2)))))
	return 1
}

func playerRemovegroupflag(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerRemoveiconbakragore(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerRemoveitem(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	itemID := uint16(L.CheckInt(2))
	count := uint32(luaOptInt(L, 3))
	if count <= 0 {
		count = 1
	}
	p.RemoveItemOfType(nil, itemID, count, 0, false)
	L.Push(lua.LTrue)
	return 1
}

func playerRemovemoney(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	amount := uint64(L.CheckNumber(2))
	// C++ signature: removeMoney(money[, flags=0[, useBank=true]]). arg 3 flags
	// are unmodelled; arg 4 useBank defaults to true.
	useBank := true
	if L.GetTop() >= 4 && L.Get(4).Type() == lua.LTBool {
		useBank = lua.LVAsBool(L.Get(4))
	}
	L.Push(lua.LBool(p.RemoveMoney(amount, useBank)))
	return 1
}

func playerRemovemount(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LFalse)
		return 1
	}
	mountID := uint16(L.CheckInt(2))
	p.RemoveMount(mountID)
	L.Push(lua.LTrue)
	return 1
}

func playerRemoveofflinetrainingtime(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	timeVal := int32(luaOptInt(L, 2))
	p.OfflineTrainingTime -= timeVal
	if p.OfflineTrainingTime < 0 {
		p.OfflineTrainingTime = 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerRemoveoutfit(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LBool(false))
		return 1
	}
	lookType := uint16(L.CheckInt(2))
	removed := p.RemoveOutfit(lookType)
	L.Push(lua.LBool(removed))
	return 1
}

func playerRemoveoutfitaddon(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LBool(false))
		return 1
	}
	lookType := uint16(L.CheckInt(2))
	addon := uint8(L.CheckInt(3))
	currentAddons := p.GetOutfitAddons(lookType)
	newAddons := currentAddons &^ addon
	if currentAddons == newAddons {
		L.Push(lua.LBool(false))
		return 1
	}
	p.RemoveOutfit(lookType)
	if newAddons > 0 {
		p.AddOutfit(lookType, newAddons)
	}
	L.Push(lua.LBool(true))
	return 1
}

func playerRemovepremiumdays(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerRemovepreystamina(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerRemovereward(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerRemovestashitem(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerRemovetaskhuntingpoints(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

// playerRemovetibiacoins debits normal coins, falling back to transferable
// coins for any shortfall (mirrors Account::removeCoins).
func playerRemovetibiacoins(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LFalse)
		return 1
	}
	amount := uint32(L.CheckInt(2))
	if p.CoinBalance+p.CoinTransferable < amount {
		L.Push(lua.LFalse)
		return 1
	}
	if p.CoinBalance >= amount {
		p.CoinBalance -= amount
	} else {
		p.CoinTransferable -= amount - p.CoinBalance
		p.CoinBalance = 0
	}
	L.Push(lua.LTrue)
	return 1
}

// playerRemovetransferableandtibiacoins spends from the combined coin pool,
// transferable first then normal — C++ account->removeCoins(Transferable,
// Normal, amount). Used by store purchases of Transferable-coin offers (the
// default), so this must actually deduct or coins never decrease.
func playerRemovetransferableandtibiacoins(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LFalse)
		return 1
	}
	amount := uint32(L.CheckInt(2))
	if p.CoinTransferable+p.CoinBalance < amount {
		L.Push(lua.LFalse)
		return 1
	}
	if p.CoinTransferable >= amount {
		p.CoinTransferable -= amount
	} else {
		p.CoinBalance -= amount - p.CoinTransferable
		p.CoinTransferable = 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerRemovetransferablecoins(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LFalse)
		return 1
	}
	amount := uint32(L.CheckInt(2))
	if p.CoinTransferable < amount {
		L.Push(lua.LFalse)
		return 1
	}
	p.CoinTransferable -= amount
	L.Push(lua.LTrue)
	return 1
}

func playerResetcharmsbestiary(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerResetoldcharms(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerRevelationstagewod(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerGetwheelpoints(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNumber(0))
		return 1
	}
	wheel := p.GetWheel()
	L.Push(lua.LNumber(wheel.GetTotalPoints(p.Level)))
	return 1
}

func playerGetwheelspentpoints(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNumber(0))
		return 1
	}
	wheel := p.GetWheel()
	L.Push(lua.LNumber(wheel.GetSpentPoints()))
	return 1
}

func playerGetwheelspells(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(L.NewTable())
		return 1
	}
	tbl := L.NewTable()
	L.Push(tbl)
	return 1
}

func playerAddwheelpoints(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LFalse)
		return 1
	}
	amount := uint16(L.CheckNumber(2))
	wheel := p.GetWheel()
	wheel.BonusPoints += amount
	L.Push(lua.LTrue)
	return 1
}

func playerSave(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	_ = p
	L.Push(lua.LTrue)
	return 1
}

func playerSendambientsoundeffect(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSendblessstatus(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSendbosstiarycooldowntimer(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSendchannelmessage(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSendcontainer(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSendcreatureappear(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSenddoublesoundeffect(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSendhousewindow(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	// Check if this is a race condition or an already open window and
	// just return success to avoid Lua errors.
	L.Push(lua.LTrue)
	return 1
}

func playerSendiconbakragore(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSendinventory(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	if p.Session != nil {
		p.Session.SendInventoryIds()
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSendlootstats(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSendmusicsoundeffect(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSendoutfitwindow(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	if p.Session != nil {
		if s, ok := p.Session.(interface{ SendOutfitWindow() }); ok {
			s.SendOutfitWindow()
		}
	}
	return 0
}

func playerSendprivatemessage(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSendsinglesoundeffect(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSendspellcooldown(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSendspellgroupcooldown(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSendtextmessage(L *lua.LState) int {
	player := checkPlayer(L)
	if player == nil {
		return 0
	}
	msgType := uint8(L.CheckNumber(2))
	text := L.CheckString(3)
	player.SendTextMessage(msgType, text)
	return 0
}

func playerSendtutorial(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSendupdatecontainer(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSetaccounttype(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	p.AccountType = uint8(L.CheckInt(2))
	L.Push(lua.LTrue)
	return 1
}

func playerSetbankbalance(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	bal := L.CheckNumber(2)
	if bal < 0 {
		bal = 0
	}
	p.BankBalance = uint64(bal)
	L.Push(lua.LTrue)
	return 1
}

func playerSetbasexpgain(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSetbosspoints(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	p.SetBossPoints(uint32(luaOptInt(L, 2)))
	L.Push(lua.LTrue)
	return 1
}

func playerSetcapacity(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	p.Capacity = uint32(luaOptInt(L, 2))
	if p.Session != nil {
		p.Session.SendStats()
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSetcurrenttitle(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSetdailyreward(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSetedithouse(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSetfaction(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSetfamiliarlooktype(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	lookType := uint16(L.CheckInt(2))
	L.Push(lua.LBool(p.SetFamiliarLooktype(lookType)))
	return 1
}

func playerSetforgedusts(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	p.SetForgeDusts(uint64(L.CheckInt(2)))
	L.Push(lua.LTrue)
	return 1
}

func playerSetghostmode(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	p.SetGhostMode(luaOptBool(L, 2))
	L.Push(lua.LTrue)
	return 1
}

func playerSetgrindingxpboost(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSetgroup(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	p.GroupID = uint16(L.CheckInt(2))
	L.Push(lua.LTrue)
	return 1
}

func playerSetgroupflag(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSetguild(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSetguildlevel(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSetguildnick(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	p.GuildNick = L.CheckString(2)
	L.Push(lua.LTrue)
	return 1
}

func playerSethazardsystempoints(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	p.HazardPoints = uint32(L.CheckInt(2))
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerSetkills(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSetlevel(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	p.Level = uint16(luaOptInt(L, 2))
	L.Push(lua.LTrue)
	return 1
}

func playerSetlivestreamviewers(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSetloyaltybonus(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSetloyaltytitle(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSetmagiclevel(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	p.MagLevel = uint16(luaOptInt(L, 2))
	L.Push(lua.LTrue)
	return 1
}

func playerSetmapshader(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSetmaxmana(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	p.MaxMana = uint32(luaOptInt(L, 2))
	if p.Mana > p.MaxMana {
		p.Mana = p.MaxMana
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSetofflinetrainingskill(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	skillVal := int8(luaOptInt(L, 2))
	p.OfflineTrainingSkill = skillVal
	L.Push(lua.LTrue)
	return 1
}

func playerSetpronoun(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSetremovebosstime(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSetserene(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSetsex(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	p.Sex = uint8(luaOptInt(L, 2))
	L.Push(lua.LTrue)
	return 1
}

func playerSetskilllevel(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	skill := luaOptInt(L, 2)
	if skill < 0 || skill >= int(game.SkillCount) {
		L.Push(lua.LFalse)
		return 1
	}
	p.Skills[skill] = uint16(luaOptInt(L, 3))
	if L.GetTop() >= 4 {
		p.SkillTries[skill] = uint64(luaOptInt(L, 4))
	}
	game.GlobalDispatcher.AddEvent(0, func() {
		if p.Session != nil {
			p.Session.SendSkills()
		}
	})
	L.Push(lua.LTrue)
	return 1
}

func playerSetskulltime(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	p.SkullTime = int64(L.CheckNumber(2))
	L.Push(lua.LTrue)
	return 1
}

func playerSetspecialcontainersavailable(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSetspeed(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	p.Speed = uint16(luaOptInt(L, 2))
	L.Push(lua.LTrue)
	return 1
}

func playerSetstamina(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSetstaminaxpboost(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSetstoragevalue(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	p.SetStorageValue(uint32(luaOptInt(L, 2)), int32(luaOptInt(L, 3)))
	L.Push(lua.LTrue)
	return 1
}

func playerSettown(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	p.TownID = uint16(L.CheckInt(2))
	L.Push(lua.LTrue)
	return 1
}

func playerSettraining(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	p.IsTraining = L.CheckBool(2)
	L.Push(lua.LTrue)
	return 1
}

func playerSetvirtue(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSetvocation(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	newVoc := uint16(luaOptInt(L, 2))
	if newVoc == 0 && p.Vocation != 0 {
		L.Push(lua.LTrue)
		return 1
	}
	p.Vocation = newVoc
	L.Push(lua.LTrue)
	return 1
}

func playerSetvoucherxpboost(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSetxpboostpercent(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerSetxpboosttime(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerShowtextdialog(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	var itemID uint16 = 2160
	var text string
	if L.GetTop() >= 3 {
		itemID = uint16(L.OptInt(2, 2160))
		text = L.CheckString(3)
	} else if L.GetTop() >= 2 {
		if L.Get(2).Type() == lua.LTNumber {
			itemID = uint16(L.CheckInt(2))
		} else {
			text = L.CheckString(2)
		}
	}
	p.SendTextWindow(100, itemID, text)
	L.Push(lua.LTrue)
	return 1
}

func playerTakescreenshot(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerTaskhuntingthirdslot(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LFalse)
	return 1
}

func playerUnlockallcharmrunes(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerUpdateconcoction(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerUpdatefood(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerFeed(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerUpdatekilltracker(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerUpdatesupplytracker(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerUpdateuiexhausted(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func playerUpgradespellswod(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(p.HazardPoints))
	return 1
}

func playerWheelunlockscroll(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

func mapLuaSkillToGo(luaSkill int) (game.Skill, bool, bool) {
	if luaSkill == 14 { // SKILL_MAGLEVEL
		return 0, true, true
	}
	if luaSkill >= 1 && luaSkill <= 7 {
		return game.Skill(luaSkill - 1), false, true
	}
	return 0, false, false
}
