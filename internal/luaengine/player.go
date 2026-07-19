package luaengine

import (
	lua "github.com/yuin/gopher-lua"
)

// registerPlayerType registers the Player userdata type.
func (e *Engine) registerPlayerType() {
	mt := e.L.NewTypeMetatable("Player")
	e.L.SetField(mt, "__index", e.L.SetFuncs(e.L.NewTable(), playerMethods))
}

var playerMethods = map[string]lua.LGFunction{
	"resetCharmsBestiary": playerResetcharmsbestiary,
	"unlockAllCharmRunes": playerUnlockallcharmrunes,
	"addCharmPoints": playerAddcharmpoints,
	"addMinorCharmEchoes": playerAddminorcharmechoes,
	"getCharmTier": playerGetcharmtier,
	"getCharmChance": playerGetcharmchance,
	"resetOldCharms": playerResetoldcharms,
	"isPlayer": playerIsplayer,
	"getGuid": playerGetguid,
	"getIp": playerGetip,
	"getAccountId": playerGetaccountid,
	"getLastLoginSaved": playerGetlastloginsaved,
	"getLastLogout": playerGetlastlogout,
	"getAccountType": playerGetaccounttype,
	"setAccountType": playerSetaccounttype,
	"isMonsterBestiaryUnlocked": playerIsmonsterbestiaryunlocked,
	"addBestiaryKill": playerAddbestiarykill,
	"charmExpansion": playerCharmexpansion,
	"getCharmMonsterType": playerGetcharmmonstertype,
	"isMonsterPrey": playerIsmonsterprey,
	"getPreyCards": playerGetpreycards,
	"getPreyLootPercentage": playerGetpreylootpercentage,
	"getPreyExperiencePercentage": playerGetpreyexperiencepercentage,
	"preyThirdSlot": playerPreythirdslot,
	"taskHuntingThirdSlot": playerTaskhuntingthirdslot,
	"removePreyStamina": playerRemovepreystamina,
	"addPreyCards": playerAddpreycards,
	"removeTaskHuntingPoints": playerRemovetaskhuntingpoints,
	"getTaskHuntingPoints": playerGettaskhuntingpoints,
	"addTaskHuntingPoints": playerAddtaskhuntingpoints,
	"getCapacity": playerGetcapacity,
	"setCapacity": playerSetcapacity,
	"isTraining": playerIstraining,
	"setTraining": playerSettraining,
	"getFreeCapacity": playerGetfreecapacity,
	"getKills": playerGetkills,
	"setKills": playerSetkills,
	"getReward": playerGetreward,
	"removeReward": playerRemovereward,
	"getRewardList": playerGetrewardlist,
	"setDailyReward": playerSetdailyreward,
	"sendInventory": playerSendinventory,
	"sendLootStats": playerSendlootstats,
	"updateSupplyTracker": playerUpdatesupplytracker,
	"updateKillTracker": playerUpdatekilltracker,
	"getDepotLocker": playerGetdepotlocker,
	"getDepotChest": playerGetdepotchest,
	"getInbox": playerGetinbox,
	"getSkullTime": playerGetskulltime,
	"setSkullTime": playerSetskulltime,
	"getDeathPenalty": playerGetdeathpenalty,
	"getExperience": playerGetexperience,
	"addExperience": playerAddexperience,
	"removeExperience": playerRemoveexperience,
	"getLevel": playerGetlevel,
	"getMagicShieldCapacityFlat": playerGetmagicshieldcapacityflat,
	"getMagicShieldCapacityPercent": playerGetmagicshieldcapacitypercent,
	"sendSpellCooldown": playerSendspellcooldown,
	"sendSpellGroupCooldown": playerSendspellgroupcooldown,
	"getMagicLevel": playerGetmagiclevel,
	"getBaseMagicLevel": playerGetbasemagiclevel,
	"getMana": playerGetmana,
	"addMana": playerAddmana,
	"getMaxMana": playerGetmaxmana,
	"setMaxMana": playerSetmaxmana,
	"getManaSpent": playerGetmanaspent,
	"addManaSpent": playerAddmanaspent,
	"getBaseMaxHealth": playerGetbasemaxhealth,
	"getBaseMaxMana": playerGetbasemaxmana,
	"getSkillLevel": playerGetskilllevel,
	"getEffectiveSkillLevel": playerGeteffectiveskilllevel,
	"getSkillPercent": playerGetskillpercent,
	"getSkillTries": playerGetskilltries,
	"addSkillTries": playerAddskilltries,
	"setLevel": playerSetlevel,
	"setMagicLevel": playerSetmagiclevel,
	"setSkillLevel": playerSetskilllevel,
	"addOfflineTrainingTime": playerAddofflinetrainingtime,
	"getOfflineTrainingTime": playerGetofflinetrainingtime,
	"removeOfflineTrainingTime": playerRemoveofflinetrainingtime,
	"addOfflineTrainingTries": playerAddofflinetrainingtries,
	"getOfflineTrainingSkill": playerGetofflinetrainingskill,
	"setOfflineTrainingSkill": playerSetofflinetrainingskill,
	"getItemCount": playerGetitemcount,
	"getStashItemCount": playerGetstashitemcount,
	"getItemById": playerGetitembyid,
	"getVocation": playerGetvocation,
	"setVocation": playerSetvocation,
	"isPromoted": playerIspromoted,
	"getSex": playerGetsex,
	"setSex": playerSetsex,
	"getPronoun": playerGetpronoun,
	"setPronoun": playerSetpronoun,
	"getTown": playerGettown,
	"setTown": playerSettown,
	"getGuild": playerGetguild,
	"setGuild": playerSetguild,
	"getGuildLevel": playerGetguildlevel,
	"setGuildLevel": playerSetguildlevel,
	"getGuildNick": playerGetguildnick,
	"setGuildNick": playerSetguildnick,
	"getGroup": playerGetgroup,
	"setGroup": playerSetgroup,
	"setSpecialContainersAvailable": playerSetspecialcontainersavailable,
	"getStashCount": playerGetstashcount,
	"openStash": playerOpenstash,
	"canReceiveLoot": playerCanreceiveloot,
	"getStamina": playerGetstamina,
	"setStamina": playerSetstamina,
	"getSoul": playerGetsoul,
	"addSoul": playerAddsoul,
	"getMaxSoul": playerGetmaxsoul,
	"getBankBalance": playerGetbankbalance,
	"setBankBalance": playerSetbankbalance,
	"getStorageValue": playerGetstoragevalue,
	"setStorageValue": playerSetstoragevalue,
	"addItem": playerAdditem,
	"addItemEx": playerAdditemex,
	"addItemBatchToPaginedContainer": playerAdditembatchtopaginedcontainer,
	"addItemStash": playerAdditemstash,
	"removeStashItem": playerRemovestashitem,
	"removeItem": playerRemoveitem,
	"sendContainer": playerSendcontainer,
	"sendUpdateContainer": playerSendupdatecontainer,
	"getMoney": playerGetmoney,
	"addMoney": playerAddmoney,
	"removeMoney": playerRemovemoney,
	"showTextDialog": playerShowtextdialog,
	"sendTextMessage": playerSendtextmessage,
	"sendChannelMessage": playerSendchannelmessage,
	"sendPrivateMessage": playerSendprivatemessage,
	"channelSay": playerChannelsay,
	"openChannel": playerOpenchannel,
	"getSlotItem": playerGetslotitem,
	"getBackpack": playerGetbackpack,
	"getLootPouch": playerGetlootpouch,
	"getParty": playerGetparty,
	"addOutfit": playerAddoutfit,
	"addOutfitAddon": playerAddoutfitaddon,
	"removeOutfit": playerRemoveoutfit,
	"removeOutfitAddon": playerRemoveoutfitaddon,
	"hasOutfit": playerHasoutfit,
	"sendOutfitWindow": playerSendoutfitwindow,
	"addMount": playerAddmount,
	"removeMount": playerRemovemount,
	"hasMount": playerHasmount,
	"addFamiliar": playerAddfamiliar,
	"removeFamiliar": playerRemovefamiliar,
	"hasFamiliar": playerHasfamiliar,
	"setFamiliarLooktype": playerSetfamiliarlooktype,
	"getFamiliarLooktype": playerGetfamiliarlooktype,
	"getPremiumDays": playerGetpremiumdays,
	"addPremiumDays": playerAddpremiumdays,
	"removePremiumDays": playerRemovepremiumdays,
	"getTibiaCoins": playerGettibiacoins,
	"addTibiaCoins": playerAddtibiacoins,
	"removeTibiaCoins": playerRemovetibiacoins,
	"getTransferableCoins": playerGettransferablecoins,
	"addTransferableCoins": playerAddtransferablecoins,
	"removeTransferableCoins": playerRemovetransferablecoins,
	"removeTransferableAndTibiaCoins": playerRemovetransferableandtibiacoins,
	"sendBlessStatus": playerSendblessstatus,
	"hasBlessing": playerHasblessing,
	"addBlessing": playerAddblessing,
	"removeBlessing": playerRemoveblessing,
	"getBlessingCount": playerGetblessingcount,
	"canLearnSpell": playerCanlearnspell,
	"learnSpell": playerLearnspell,
	"forgetSpell": playerForgetspell,
	"hasLearnedSpell": playerHaslearnedspell,
	"applyImbuementScroll": playerApplyimbuementscroll,
	"openImbuementWindow": playerOpenimbuementwindow,
	"closeImbuementWindow": playerCloseimbuementwindow,
	"clearAllImbuements": playerClearallimbuements,
	"sendTutorial": playerSendtutorial,
	"addMapMark": playerAddmapmark,
	"save": playerSave,
	"popupFYI": playerPopupfyi,
	"isPzLocked": playerIspzlocked,
	"getClient": playerGetclient,
	"getHouse": playerGethouse,
	"sendHouseWindow": playerSendhousewindow,
	"setEditHouse": playerSetedithouse,
	"setGhostMode": playerSetghostmode,
	"getContainerId": playerGetcontainerid,
	"getContainerById": playerGetcontainerbyid,
	"getContainerIndex": playerGetcontainerindex,
	"getInstantSpells": playerGetinstantspells,
	"canCast": playerCancast,
	"hasChaseMode": playerHaschasemode,
	"hasSecureMode": playerHassecuremode,
	"getFightMode": playerGetfightmode,
	"getBaseXpGain": playerGetbasexpgain,
	"setBaseXpGain": playerSetbasexpgain,
	"getVoucherXpBoost": playerGetvoucherxpboost,
	"setVoucherXpBoost": playerSetvoucherxpboost,
	"getGrindingXpBoost": playerGetgrindingxpboost,
	"setGrindingXpBoost": playerSetgrindingxpboost,
	"getXpBoostPercent": playerGetxpboostpercent,
	"setXpBoostPercent": playerSetxpboostpercent,
	"getStaminaXpBoost": playerGetstaminaxpboost,
	"setStaminaXpBoost": playerSetstaminaxpboost,
	"getXpBoostTime": playerGetxpboosttime,
	"setXpBoostTime": playerSetxpboosttime,
	"getIdleTime": playerGetidletime,
	"getFreeBackpackSlots": playerGetfreebackpackslots,
	"isOffline": playerIsoffline,
	"openMarket": playerOpenmarket,
	"instantSkillWOD": playerInstantskillwod,
	"upgradeSpellsWOD": playerUpgradespellswod,
	"revelationStageWOD": playerRevelationstagewod,
	"reloadData": playerReloaddata,
	"onThinkWheelOfDestiny": playerOnthinkwheelofdestiny,
	"avatarTimer": playerAvatartimer,
	"getWheelSpellAdditionalArea": playerGetwheelspelladditionalarea,
	"getWheelSpellAdditionalTarget": playerGetwheelspelladditionaltarget,
	"getWheelSpellAdditionalDuration": playerGetwheelspelladditionalduration,
	"wheelUnlockScroll": playerWheelunlockscroll,
	"openForge": playerOpenforge,
	"closeForge": playerCloseforge,
	"addForgeDusts": playerAddforgedusts,
	"removeForgeDusts": playerRemoveforgedusts,
	"getForgeDusts": playerGetforgedusts,
	"setForgeDusts": playerSetforgedusts,
	"addForgeDustLevel": playerAddforgedustlevel,
	"removeForgeDustLevel": playerRemoveforgedustlevel,
	"getForgeDustLevel": playerGetforgedustlevel,
	"getForgeSlivers": playerGetforgeslivers,
	"getForgeCores": playerGetforgecores,
	"isUIExhausted": playerIsuiexhausted,
	"updateUIExhausted": playerUpdateuiexhausted,
	"setFaction": playerSetfaction,
	"getFaction": playerGetfaction,
	"getBosstiaryLevel": playerGetbosstiarylevel,
	"getBosstiaryKills": playerGetbosstiarykills,
	"addBosstiaryKill": playerAddbosstiarykill,
	"setBossPoints": playerSetbosspoints,
	"setRemoveBossTime": playerSetremovebosstime,
	"getSlotBossId": playerGetslotbossid,
	"getBossBonus": playerGetbossbonus,
	"sendBosstiaryCooldownTimer": playerSendbosstiarycooldowntimer,
	"sendSingleSoundEffect": playerSendsinglesoundeffect,
	"sendDoubleSoundEffect": playerSenddoublesoundeffect,
	"sendAmbientSoundEffect": playerSendambientsoundeffect,
	"sendMusicSoundEffect": playerSendmusicsoundeffect,
	"getName": playerGetname,
	"changeName": playerChangename,
	"hasGroupFlag": playerHasgroupflag,
	"setGroupFlag": playerSetgroupflag,
	"removeGroupFlag": playerRemovegroupflag,
	"setHazardSystemPoints": playerSethazardsystempoints,
	"getHazardSystemPoints": playerGethazardsystempoints,
	"setLoyaltyBonus": playerSetloyaltybonus,
	"getLoyaltyBonus": playerGetloyaltybonus,
	"getLoyaltyPoints": playerGetloyaltypoints,
	"getLoyaltyTitle": playerGetloyaltytitle,
	"setLoyaltyTitle": playerSetloyaltytitle,
	"updateConcoction": playerUpdateconcoction,
	"updateFood": playerUpdatefood,
	"clearSpellCooldowns": playerClearspellcooldowns,
	"isVip": playerIsvip,
	"getVipDays": playerGetvipdays,
	"getVipTime": playerGetviptime,
	"kv": playerKv,
	"getStoreInbox": playerGetstoreinbox,
	"hasAchievement": playerHasachievement,
	"addAchievement": playerAddachievement,
	"removeAchievement": playerRemoveachievement,
	"getAchievementPoints": playerGetachievementpoints,
	"addAchievementPoints": playerAddachievementpoints,
	"removeAchievementPoints": playerRemoveachievementpoints,
	"addBadge": playerAddbadge,
	"addTitle": playerAddtitle,
	"getTitles": playerGettitles,
	"setCurrentTitle": playerSetcurrenttitle,
	"createTransactionSummary": playerCreatetransactionsummary,
	"takeScreenshot": playerTakescreenshot,
	"sendIconBakragore": playerSendiconbakragore,
	"removeIconBakragore": playerRemoveiconbakragore,
	"sendCreatureAppear": playerSendcreatureappear,
	"addAnimusMastery": playerAddanimusmastery,
	"removeAnimusMastery": playerRemoveanimusmastery,
	"hasAnimusMastery": playerHasanimusmastery,
	"setSerene": playerSetserene,
	"getVirtue": playerGetvirtue,
	"setVirtue": playerSetvirtue,
	"fillHarmony": playerFillharmony,
	"getHarmony": playerGetharmony,
	"getHarmonyDamage": playerGetharmonydamage,
	"calculateFlatDamageHealing": playerCalculateflatdamagehealing,
	"setSpeed": playerSetspeed,
	"addWeaponExperience": playerAddweaponexperience,
	"getLivestreamViewersCount": playerGetlivestreamviewerscount,
	"getLivestreamViewers": playerGetlivestreamviewers,
	"setLivestreamViewers": playerSetlivestreamviewers,
	"isLivestreamViewer": playerIslivestreamviewer,
	"getMapShader": playerGetmapshader,
	"setMapShader": playerSetmapshader,
	"removeCustomOutfit": playerRemovecustomoutfit,
	"addCustomOutfit": playerAddcustomoutfit,
}

func playerAddachievement(L *lua.LState) int {
	// TODO: implement addAchievement
	return 0
}

func playerAddachievementpoints(L *lua.LState) int {
	// TODO: implement addAchievementPoints
	return 0
}

func playerAddanimusmastery(L *lua.LState) int {
	// TODO: implement addAnimusMastery
	return 0
}

func playerAddbadge(L *lua.LState) int {
	// TODO: implement addBadge
	return 0
}

func playerAddbestiarykill(L *lua.LState) int {
	// TODO: implement addBestiaryKill
	return 0
}

func playerAddblessing(L *lua.LState) int {
	// TODO: implement addBlessing
	return 0
}

func playerAddbosstiarykill(L *lua.LState) int {
	// TODO: implement addBosstiaryKill
	return 0
}

func playerAddcharmpoints(L *lua.LState) int {
	// TODO: implement addCharmPoints
	return 0
}

func playerAddcustomoutfit(L *lua.LState) int {
	// TODO: implement addCustomOutfit
	return 0
}

func playerAddexperience(L *lua.LState) int {
	// TODO: implement addExperience
	return 0
}

func playerAddfamiliar(L *lua.LState) int {
	// TODO: implement addFamiliar
	return 0
}

func playerAddforgedustlevel(L *lua.LState) int {
	// TODO: implement addForgeDustLevel
	return 0
}

func playerAddforgedusts(L *lua.LState) int {
	// TODO: implement addForgeDusts
	return 0
}

func playerAdditem(L *lua.LState) int {
	// TODO: implement addItem
	return 0
}

func playerAdditembatchtopaginedcontainer(L *lua.LState) int {
	// TODO: implement addItemBatchToPaginedContainer
	return 0
}

func playerAdditemex(L *lua.LState) int {
	// TODO: implement addItemEx
	return 0
}

func playerAdditemstash(L *lua.LState) int {
	// TODO: implement addItemStash
	return 0
}

func playerAddmana(L *lua.LState) int {
	// TODO: implement addMana
	return 0
}

func playerAddmanaspent(L *lua.LState) int {
	// TODO: implement addManaSpent
	return 0
}

func playerAddmapmark(L *lua.LState) int {
	// TODO: implement addMapMark
	return 0
}

func playerAddminorcharmechoes(L *lua.LState) int {
	// TODO: implement addMinorCharmEchoes
	return 0
}

func playerAddmoney(L *lua.LState) int {
	// TODO: implement addMoney
	return 0
}

func playerAddmount(L *lua.LState) int {
	// TODO: implement addMount
	return 0
}

func playerAddofflinetrainingtime(L *lua.LState) int {
	// TODO: implement addOfflineTrainingTime
	return 0
}

func playerAddofflinetrainingtries(L *lua.LState) int {
	// TODO: implement addOfflineTrainingTries
	return 0
}

func playerAddoutfit(L *lua.LState) int {
	// TODO: implement addOutfit
	return 0
}

func playerAddoutfitaddon(L *lua.LState) int {
	// TODO: implement addOutfitAddon
	return 0
}

func playerAddpremiumdays(L *lua.LState) int {
	// TODO: implement addPremiumDays
	return 0
}

func playerAddpreycards(L *lua.LState) int {
	// TODO: implement addPreyCards
	return 0
}

func playerAddskilltries(L *lua.LState) int {
	// TODO: implement addSkillTries
	return 0
}

func playerAddsoul(L *lua.LState) int {
	// TODO: implement addSoul
	return 0
}

func playerAddtaskhuntingpoints(L *lua.LState) int {
	// TODO: implement addTaskHuntingPoints
	return 0
}

func playerAddtibiacoins(L *lua.LState) int {
	// TODO: implement addTibiaCoins
	return 0
}

func playerAddtitle(L *lua.LState) int {
	// TODO: implement addTitle
	return 0
}

func playerAddtransferablecoins(L *lua.LState) int {
	// TODO: implement addTransferableCoins
	return 0
}

func playerAddweaponexperience(L *lua.LState) int {
	// TODO: implement addWeaponExperience
	return 0
}

func playerApplyimbuementscroll(L *lua.LState) int {
	// TODO: implement applyImbuementScroll
	return 0
}

func playerAvatartimer(L *lua.LState) int {
	// TODO: implement avatarTimer
	return 0
}

func playerCalculateflatdamagehealing(L *lua.LState) int {
	// TODO: implement calculateFlatDamageHealing
	return 0
}

func playerCancast(L *lua.LState) int {
	// TODO: implement canCast
	return 0
}

func playerCanlearnspell(L *lua.LState) int {
	// TODO: implement canLearnSpell
	return 0
}

func playerCanreceiveloot(L *lua.LState) int {
	// TODO: implement canReceiveLoot
	return 0
}

func playerChangename(L *lua.LState) int {
	// TODO: implement changeName
	return 0
}

func playerChannelsay(L *lua.LState) int {
	// TODO: implement channelSay
	return 0
}

func playerCharmexpansion(L *lua.LState) int {
	// TODO: implement charmExpansion
	return 0
}

func playerClearallimbuements(L *lua.LState) int {
	// TODO: implement clearAllImbuements
	return 0
}

func playerClearspellcooldowns(L *lua.LState) int {
	// TODO: implement clearSpellCooldowns
	return 0
}

func playerCloseforge(L *lua.LState) int {
	// TODO: implement closeForge
	return 0
}

func playerCloseimbuementwindow(L *lua.LState) int {
	// TODO: implement closeImbuementWindow
	return 0
}

func playerCreatetransactionsummary(L *lua.LState) int {
	// TODO: implement createTransactionSummary
	return 0
}

func playerFillharmony(L *lua.LState) int {
	// TODO: implement fillHarmony
	return 0
}

func playerForgetspell(L *lua.LState) int {
	// TODO: implement forgetSpell
	return 0
}

func playerGetaccountid(L *lua.LState) int {
	// TODO: implement getAccountId
	return 0
}

func playerGetaccounttype(L *lua.LState) int {
	// TODO: implement getAccountType
	return 0
}

func playerGetachievementpoints(L *lua.LState) int {
	// TODO: implement getAchievementPoints
	return 0
}

func playerGetbackpack(L *lua.LState) int {
	// TODO: implement getBackpack
	return 0
}

func playerGetbankbalance(L *lua.LState) int {
	// TODO: implement getBankBalance
	return 0
}

func playerGetbasemagiclevel(L *lua.LState) int {
	// TODO: implement getBaseMagicLevel
	return 0
}

func playerGetbasemaxhealth(L *lua.LState) int {
	// TODO: implement getBaseMaxHealth
	return 0
}

func playerGetbasemaxmana(L *lua.LState) int {
	// TODO: implement getBaseMaxMana
	return 0
}

func playerGetbasexpgain(L *lua.LState) int {
	// TODO: implement getBaseXpGain
	return 0
}

func playerGetblessingcount(L *lua.LState) int {
	// TODO: implement getBlessingCount
	return 0
}

func playerGetbossbonus(L *lua.LState) int {
	// TODO: implement getBossBonus
	return 0
}

func playerGetbosstiarykills(L *lua.LState) int {
	// TODO: implement getBosstiaryKills
	return 0
}

func playerGetbosstiarylevel(L *lua.LState) int {
	// TODO: implement getBosstiaryLevel
	return 0
}

func playerGetcapacity(L *lua.LState) int {
	// TODO: implement getCapacity
	return 0
}

func playerGetcharmchance(L *lua.LState) int {
	// TODO: implement getCharmChance
	return 0
}

func playerGetcharmmonstertype(L *lua.LState) int {
	// TODO: implement getCharmMonsterType
	return 0
}

func playerGetcharmtier(L *lua.LState) int {
	// TODO: implement getCharmTier
	return 0
}

func playerGetclient(L *lua.LState) int {
	// TODO: implement getClient
	return 0
}

func playerGetcontainerbyid(L *lua.LState) int {
	// TODO: implement getContainerById
	return 0
}

func playerGetcontainerid(L *lua.LState) int {
	// TODO: implement getContainerId
	return 0
}

func playerGetcontainerindex(L *lua.LState) int {
	// TODO: implement getContainerIndex
	return 0
}

func playerGetdeathpenalty(L *lua.LState) int {
	// TODO: implement getDeathPenalty
	return 0
}

func playerGetdepotchest(L *lua.LState) int {
	// TODO: implement getDepotChest
	return 0
}

func playerGetdepotlocker(L *lua.LState) int {
	// TODO: implement getDepotLocker
	return 0
}

func playerGeteffectiveskilllevel(L *lua.LState) int {
	// TODO: implement getEffectiveSkillLevel
	return 0
}

func playerGetexperience(L *lua.LState) int {
	// TODO: implement getExperience
	return 0
}

func playerGetfaction(L *lua.LState) int {
	// TODO: implement getFaction
	return 0
}

func playerGetfamiliarlooktype(L *lua.LState) int {
	// TODO: implement getFamiliarLooktype
	return 0
}

func playerGetfightmode(L *lua.LState) int {
	// TODO: implement getFightMode
	return 0
}

func playerGetforgecores(L *lua.LState) int {
	// TODO: implement getForgeCores
	return 0
}

func playerGetforgedustlevel(L *lua.LState) int {
	// TODO: implement getForgeDustLevel
	return 0
}

func playerGetforgedusts(L *lua.LState) int {
	// TODO: implement getForgeDusts
	return 0
}

func playerGetforgeslivers(L *lua.LState) int {
	// TODO: implement getForgeSlivers
	return 0
}

func playerGetfreebackpackslots(L *lua.LState) int {
	// TODO: implement getFreeBackpackSlots
	return 0
}

func playerGetfreecapacity(L *lua.LState) int {
	// TODO: implement getFreeCapacity
	return 0
}

func playerGetgrindingxpboost(L *lua.LState) int {
	// TODO: implement getGrindingXpBoost
	return 0
}

func playerGetgroup(L *lua.LState) int {
	// TODO: implement getGroup
	return 0
}

func playerGetguid(L *lua.LState) int {
	// TODO: implement getGuid
	return 0
}

func playerGetguild(L *lua.LState) int {
	// TODO: implement getGuild
	return 0
}

func playerGetguildlevel(L *lua.LState) int {
	// TODO: implement getGuildLevel
	return 0
}

func playerGetguildnick(L *lua.LState) int {
	// TODO: implement getGuildNick
	return 0
}

func playerGetharmony(L *lua.LState) int {
	// TODO: implement getHarmony
	return 0
}

func playerGetharmonydamage(L *lua.LState) int {
	// TODO: implement getHarmonyDamage
	return 0
}

func playerGethazardsystempoints(L *lua.LState) int {
	// TODO: implement getHazardSystemPoints
	return 0
}

func playerGethouse(L *lua.LState) int {
	// TODO: implement getHouse
	return 0
}

func playerGetidletime(L *lua.LState) int {
	// TODO: implement getIdleTime
	return 0
}

func playerGetinbox(L *lua.LState) int {
	// TODO: implement getInbox
	return 0
}

func playerGetinstantspells(L *lua.LState) int {
	// TODO: implement getInstantSpells
	return 0
}

func playerGetip(L *lua.LState) int {
	// TODO: implement getIp
	return 0
}

func playerGetitembyid(L *lua.LState) int {
	// TODO: implement getItemById
	return 0
}

func playerGetitemcount(L *lua.LState) int {
	// TODO: implement getItemCount
	return 0
}

func playerGetkills(L *lua.LState) int {
	// TODO: implement getKills
	return 0
}

func playerGetlastloginsaved(L *lua.LState) int {
	// TODO: implement getLastLoginSaved
	return 0
}

func playerGetlastlogout(L *lua.LState) int {
	// TODO: implement getLastLogout
	return 0
}

func playerGetlevel(L *lua.LState) int {
	// TODO: implement getLevel
	return 0
}

func playerGetlivestreamviewers(L *lua.LState) int {
	// TODO: implement getLivestreamViewers
	return 0
}

func playerGetlivestreamviewerscount(L *lua.LState) int {
	// TODO: implement getLivestreamViewersCount
	return 0
}

func playerGetlootpouch(L *lua.LState) int {
	// TODO: implement getLootPouch
	return 0
}

func playerGetloyaltybonus(L *lua.LState) int {
	// TODO: implement getLoyaltyBonus
	return 0
}

func playerGetloyaltypoints(L *lua.LState) int {
	// TODO: implement getLoyaltyPoints
	return 0
}

func playerGetloyaltytitle(L *lua.LState) int {
	// TODO: implement getLoyaltyTitle
	return 0
}

func playerGetmagiclevel(L *lua.LState) int {
	// TODO: implement getMagicLevel
	return 0
}

func playerGetmagicshieldcapacityflat(L *lua.LState) int {
	// TODO: implement getMagicShieldCapacityFlat
	return 0
}

func playerGetmagicshieldcapacitypercent(L *lua.LState) int {
	// TODO: implement getMagicShieldCapacityPercent
	return 0
}

func playerGetmana(L *lua.LState) int {
	// TODO: implement getMana
	return 0
}

func playerGetmanaspent(L *lua.LState) int {
	// TODO: implement getManaSpent
	return 0
}

func playerGetmapshader(L *lua.LState) int {
	// TODO: implement getMapShader
	return 0
}

func playerGetmaxmana(L *lua.LState) int {
	// TODO: implement getMaxMana
	return 0
}

func playerGetmaxsoul(L *lua.LState) int {
	// TODO: implement getMaxSoul
	return 0
}

func playerGetmoney(L *lua.LState) int {
	// TODO: implement getMoney
	return 0
}

func playerGetname(L *lua.LState) int {
	// TODO: implement getName
	return 0
}

func playerGetofflinetrainingskill(L *lua.LState) int {
	// TODO: implement getOfflineTrainingSkill
	return 0
}

func playerGetofflinetrainingtime(L *lua.LState) int {
	// TODO: implement getOfflineTrainingTime
	return 0
}

func playerGetparty(L *lua.LState) int {
	// TODO: implement getParty
	return 0
}

func playerGetpremiumdays(L *lua.LState) int {
	// TODO: implement getPremiumDays
	return 0
}

func playerGetpreycards(L *lua.LState) int {
	// TODO: implement getPreyCards
	return 0
}

func playerGetpreyexperiencepercentage(L *lua.LState) int {
	// TODO: implement getPreyExperiencePercentage
	return 0
}

func playerGetpreylootpercentage(L *lua.LState) int {
	// TODO: implement getPreyLootPercentage
	return 0
}

func playerGetpronoun(L *lua.LState) int {
	// TODO: implement getPronoun
	return 0
}

func playerGetreward(L *lua.LState) int {
	// TODO: implement getReward
	return 0
}

func playerGetrewardlist(L *lua.LState) int {
	// TODO: implement getRewardList
	return 0
}

func playerGetsex(L *lua.LState) int {
	// TODO: implement getSex
	return 0
}

func playerGetskilllevel(L *lua.LState) int {
	// TODO: implement getSkillLevel
	return 0
}

func playerGetskillpercent(L *lua.LState) int {
	// TODO: implement getSkillPercent
	return 0
}

func playerGetskilltries(L *lua.LState) int {
	// TODO: implement getSkillTries
	return 0
}

func playerGetskulltime(L *lua.LState) int {
	// TODO: implement getSkullTime
	return 0
}

func playerGetslotbossid(L *lua.LState) int {
	// TODO: implement getSlotBossId
	return 0
}

func playerGetslotitem(L *lua.LState) int {
	// TODO: implement getSlotItem
	return 0
}

func playerGetsoul(L *lua.LState) int {
	// TODO: implement getSoul
	return 0
}

func playerGetstamina(L *lua.LState) int {
	// TODO: implement getStamina
	return 0
}

func playerGetstaminaxpboost(L *lua.LState) int {
	// TODO: implement getStaminaXpBoost
	return 0
}

func playerGetstashcount(L *lua.LState) int {
	// TODO: implement getStashCount
	return 0
}

func playerGetstashitemcount(L *lua.LState) int {
	// TODO: implement getStashItemCount
	return 0
}

func playerGetstoragevalue(L *lua.LState) int {
	// TODO: implement getStorageValue
	return 0
}

func playerGetstoreinbox(L *lua.LState) int {
	// TODO: implement getStoreInbox
	return 0
}

func playerGettaskhuntingpoints(L *lua.LState) int {
	// TODO: implement getTaskHuntingPoints
	return 0
}

func playerGettibiacoins(L *lua.LState) int {
	// TODO: implement getTibiaCoins
	return 0
}

func playerGettitles(L *lua.LState) int {
	// TODO: implement getTitles
	return 0
}

func playerGettown(L *lua.LState) int {
	// TODO: implement getTown
	return 0
}

func playerGettransferablecoins(L *lua.LState) int {
	// TODO: implement getTransferableCoins
	return 0
}

func playerGetvipdays(L *lua.LState) int {
	// TODO: implement getVipDays
	return 0
}

func playerGetviptime(L *lua.LState) int {
	// TODO: implement getVipTime
	return 0
}

func playerGetvirtue(L *lua.LState) int {
	// TODO: implement getVirtue
	return 0
}

func playerGetvocation(L *lua.LState) int {
	// TODO: implement getVocation
	return 0
}

func playerGetvoucherxpboost(L *lua.LState) int {
	// TODO: implement getVoucherXpBoost
	return 0
}

func playerGetwheelspelladditionalarea(L *lua.LState) int {
	// TODO: implement getWheelSpellAdditionalArea
	return 0
}

func playerGetwheelspelladditionalduration(L *lua.LState) int {
	// TODO: implement getWheelSpellAdditionalDuration
	return 0
}

func playerGetwheelspelladditionaltarget(L *lua.LState) int {
	// TODO: implement getWheelSpellAdditionalTarget
	return 0
}

func playerGetxpboostpercent(L *lua.LState) int {
	// TODO: implement getXpBoostPercent
	return 0
}

func playerGetxpboosttime(L *lua.LState) int {
	// TODO: implement getXpBoostTime
	return 0
}

func playerHasachievement(L *lua.LState) int {
	// TODO: implement hasAchievement
	return 0
}

func playerHasanimusmastery(L *lua.LState) int {
	// TODO: implement hasAnimusMastery
	return 0
}

func playerHasblessing(L *lua.LState) int {
	// TODO: implement hasBlessing
	return 0
}

func playerHaschasemode(L *lua.LState) int {
	// TODO: implement hasChaseMode
	return 0
}

func playerHasfamiliar(L *lua.LState) int {
	// TODO: implement hasFamiliar
	return 0
}

func playerHasgroupflag(L *lua.LState) int {
	// TODO: implement hasGroupFlag
	return 0
}

func playerHaslearnedspell(L *lua.LState) int {
	// TODO: implement hasLearnedSpell
	return 0
}

func playerHasmount(L *lua.LState) int {
	// TODO: implement hasMount
	return 0
}

func playerHasoutfit(L *lua.LState) int {
	// TODO: implement hasOutfit
	return 0
}

func playerHassecuremode(L *lua.LState) int {
	// TODO: implement hasSecureMode
	return 0
}

func playerInstantskillwod(L *lua.LState) int {
	// TODO: implement instantSkillWOD
	return 0
}

func playerIslivestreamviewer(L *lua.LState) int {
	// TODO: implement isLivestreamViewer
	return 0
}

func playerIsmonsterbestiaryunlocked(L *lua.LState) int {
	// TODO: implement isMonsterBestiaryUnlocked
	return 0
}

func playerIsmonsterprey(L *lua.LState) int {
	// TODO: implement isMonsterPrey
	return 0
}

func playerIsoffline(L *lua.LState) int {
	// TODO: implement isOffline
	return 0
}

func playerIsplayer(L *lua.LState) int {
	// TODO: implement isPlayer
	return 0
}

func playerIspromoted(L *lua.LState) int {
	// TODO: implement isPromoted
	return 0
}

func playerIspzlocked(L *lua.LState) int {
	// TODO: implement isPzLocked
	return 0
}

func playerIstraining(L *lua.LState) int {
	// TODO: implement isTraining
	return 0
}

func playerIsuiexhausted(L *lua.LState) int {
	// TODO: implement isUIExhausted
	return 0
}

func playerIsvip(L *lua.LState) int {
	// TODO: implement isVip
	return 0
}

func playerKv(L *lua.LState) int {
	// TODO: implement kv
	return 0
}

func playerLearnspell(L *lua.LState) int {
	// TODO: implement learnSpell
	return 0
}

func playerOnthinkwheelofdestiny(L *lua.LState) int {
	// TODO: implement onThinkWheelOfDestiny
	return 0
}

func playerOpenchannel(L *lua.LState) int {
	// TODO: implement openChannel
	return 0
}

func playerOpenforge(L *lua.LState) int {
	// TODO: implement openForge
	return 0
}

func playerOpenimbuementwindow(L *lua.LState) int {
	// TODO: implement openImbuementWindow
	return 0
}

func playerOpenmarket(L *lua.LState) int {
	// TODO: implement openMarket
	return 0
}

func playerOpenstash(L *lua.LState) int {
	// TODO: implement openStash
	return 0
}

func playerPopupfyi(L *lua.LState) int {
	// TODO: implement popupFYI
	return 0
}

func playerPreythirdslot(L *lua.LState) int {
	// TODO: implement preyThirdSlot
	return 0
}

func playerReloaddata(L *lua.LState) int {
	// TODO: implement reloadData
	return 0
}

func playerRemoveachievement(L *lua.LState) int {
	// TODO: implement removeAchievement
	return 0
}

func playerRemoveachievementpoints(L *lua.LState) int {
	// TODO: implement removeAchievementPoints
	return 0
}

func playerRemoveanimusmastery(L *lua.LState) int {
	// TODO: implement removeAnimusMastery
	return 0
}

func playerRemoveblessing(L *lua.LState) int {
	// TODO: implement removeBlessing
	return 0
}

func playerRemovecustomoutfit(L *lua.LState) int {
	// TODO: implement removeCustomOutfit
	return 0
}

func playerRemoveexperience(L *lua.LState) int {
	// TODO: implement removeExperience
	return 0
}

func playerRemovefamiliar(L *lua.LState) int {
	// TODO: implement removeFamiliar
	return 0
}

func playerRemoveforgedustlevel(L *lua.LState) int {
	// TODO: implement removeForgeDustLevel
	return 0
}

func playerRemoveforgedusts(L *lua.LState) int {
	// TODO: implement removeForgeDusts
	return 0
}

func playerRemovegroupflag(L *lua.LState) int {
	// TODO: implement removeGroupFlag
	return 0
}

func playerRemoveiconbakragore(L *lua.LState) int {
	// TODO: implement removeIconBakragore
	return 0
}

func playerRemoveitem(L *lua.LState) int {
	// TODO: implement removeItem
	return 0
}

func playerRemovemoney(L *lua.LState) int {
	// TODO: implement removeMoney
	return 0
}

func playerRemovemount(L *lua.LState) int {
	// TODO: implement removeMount
	return 0
}

func playerRemoveofflinetrainingtime(L *lua.LState) int {
	// TODO: implement removeOfflineTrainingTime
	return 0
}

func playerRemoveoutfit(L *lua.LState) int {
	// TODO: implement removeOutfit
	return 0
}

func playerRemoveoutfitaddon(L *lua.LState) int {
	// TODO: implement removeOutfitAddon
	return 0
}

func playerRemovepremiumdays(L *lua.LState) int {
	// TODO: implement removePremiumDays
	return 0
}

func playerRemovepreystamina(L *lua.LState) int {
	// TODO: implement removePreyStamina
	return 0
}

func playerRemovereward(L *lua.LState) int {
	// TODO: implement removeReward
	return 0
}

func playerRemovestashitem(L *lua.LState) int {
	// TODO: implement removeStashItem
	return 0
}

func playerRemovetaskhuntingpoints(L *lua.LState) int {
	// TODO: implement removeTaskHuntingPoints
	return 0
}

func playerRemovetibiacoins(L *lua.LState) int {
	// TODO: implement removeTibiaCoins
	return 0
}

func playerRemovetransferableandtibiacoins(L *lua.LState) int {
	// TODO: implement removeTransferableAndTibiaCoins
	return 0
}

func playerRemovetransferablecoins(L *lua.LState) int {
	// TODO: implement removeTransferableCoins
	return 0
}

func playerResetcharmsbestiary(L *lua.LState) int {
	// TODO: implement resetCharmsBestiary
	return 0
}

func playerResetoldcharms(L *lua.LState) int {
	// TODO: implement resetOldCharms
	return 0
}

func playerRevelationstagewod(L *lua.LState) int {
	// TODO: implement revelationStageWOD
	return 0
}

func playerSave(L *lua.LState) int {
	// TODO: implement save
	return 0
}

func playerSendambientsoundeffect(L *lua.LState) int {
	// TODO: implement sendAmbientSoundEffect
	return 0
}

func playerSendblessstatus(L *lua.LState) int {
	// TODO: implement sendBlessStatus
	return 0
}

func playerSendbosstiarycooldowntimer(L *lua.LState) int {
	// TODO: implement sendBosstiaryCooldownTimer
	return 0
}

func playerSendchannelmessage(L *lua.LState) int {
	// TODO: implement sendChannelMessage
	return 0
}

func playerSendcontainer(L *lua.LState) int {
	// TODO: implement sendContainer
	return 0
}

func playerSendcreatureappear(L *lua.LState) int {
	// TODO: implement sendCreatureAppear
	return 0
}

func playerSenddoublesoundeffect(L *lua.LState) int {
	// TODO: implement sendDoubleSoundEffect
	return 0
}

func playerSendhousewindow(L *lua.LState) int {
	// TODO: implement sendHouseWindow
	return 0
}

func playerSendiconbakragore(L *lua.LState) int {
	// TODO: implement sendIconBakragore
	return 0
}

func playerSendinventory(L *lua.LState) int {
	// TODO: implement sendInventory
	return 0
}

func playerSendlootstats(L *lua.LState) int {
	// TODO: implement sendLootStats
	return 0
}

func playerSendmusicsoundeffect(L *lua.LState) int {
	// TODO: implement sendMusicSoundEffect
	return 0
}

func playerSendoutfitwindow(L *lua.LState) int {
	// TODO: implement sendOutfitWindow
	return 0
}

func playerSendprivatemessage(L *lua.LState) int {
	// TODO: implement sendPrivateMessage
	return 0
}

func playerSendsinglesoundeffect(L *lua.LState) int {
	// TODO: implement sendSingleSoundEffect
	return 0
}

func playerSendspellcooldown(L *lua.LState) int {
	// TODO: implement sendSpellCooldown
	return 0
}

func playerSendspellgroupcooldown(L *lua.LState) int {
	// TODO: implement sendSpellGroupCooldown
	return 0
}

func playerSendtextmessage(L *lua.LState) int {
	// TODO: implement sendTextMessage
	return 0
}

func playerSendtutorial(L *lua.LState) int {
	// TODO: implement sendTutorial
	return 0
}

func playerSendupdatecontainer(L *lua.LState) int {
	// TODO: implement sendUpdateContainer
	return 0
}

func playerSetaccounttype(L *lua.LState) int {
	// TODO: implement setAccountType
	return 0
}

func playerSetbankbalance(L *lua.LState) int {
	// TODO: implement setBankBalance
	return 0
}

func playerSetbasexpgain(L *lua.LState) int {
	// TODO: implement setBaseXpGain
	return 0
}

func playerSetbosspoints(L *lua.LState) int {
	// TODO: implement setBossPoints
	return 0
}

func playerSetcapacity(L *lua.LState) int {
	// TODO: implement setCapacity
	return 0
}

func playerSetcurrenttitle(L *lua.LState) int {
	// TODO: implement setCurrentTitle
	return 0
}

func playerSetdailyreward(L *lua.LState) int {
	// TODO: implement setDailyReward
	return 0
}

func playerSetedithouse(L *lua.LState) int {
	// TODO: implement setEditHouse
	return 0
}

func playerSetfaction(L *lua.LState) int {
	// TODO: implement setFaction
	return 0
}

func playerSetfamiliarlooktype(L *lua.LState) int {
	// TODO: implement setFamiliarLooktype
	return 0
}

func playerSetforgedusts(L *lua.LState) int {
	// TODO: implement setForgeDusts
	return 0
}

func playerSetghostmode(L *lua.LState) int {
	// TODO: implement setGhostMode
	return 0
}

func playerSetgrindingxpboost(L *lua.LState) int {
	// TODO: implement setGrindingXpBoost
	return 0
}

func playerSetgroup(L *lua.LState) int {
	// TODO: implement setGroup
	return 0
}

func playerSetgroupflag(L *lua.LState) int {
	// TODO: implement setGroupFlag
	return 0
}

func playerSetguild(L *lua.LState) int {
	// TODO: implement setGuild
	return 0
}

func playerSetguildlevel(L *lua.LState) int {
	// TODO: implement setGuildLevel
	return 0
}

func playerSetguildnick(L *lua.LState) int {
	// TODO: implement setGuildNick
	return 0
}

func playerSethazardsystempoints(L *lua.LState) int {
	// TODO: implement setHazardSystemPoints
	return 0
}

func playerSetkills(L *lua.LState) int {
	// TODO: implement setKills
	return 0
}

func playerSetlevel(L *lua.LState) int {
	// TODO: implement setLevel
	return 0
}

func playerSetlivestreamviewers(L *lua.LState) int {
	// TODO: implement setLivestreamViewers
	return 0
}

func playerSetloyaltybonus(L *lua.LState) int {
	// TODO: implement setLoyaltyBonus
	return 0
}

func playerSetloyaltytitle(L *lua.LState) int {
	// TODO: implement setLoyaltyTitle
	return 0
}

func playerSetmagiclevel(L *lua.LState) int {
	// TODO: implement setMagicLevel
	return 0
}

func playerSetmapshader(L *lua.LState) int {
	// TODO: implement setMapShader
	return 0
}

func playerSetmaxmana(L *lua.LState) int {
	// TODO: implement setMaxMana
	return 0
}

func playerSetofflinetrainingskill(L *lua.LState) int {
	// TODO: implement setOfflineTrainingSkill
	return 0
}

func playerSetpronoun(L *lua.LState) int {
	// TODO: implement setPronoun
	return 0
}

func playerSetremovebosstime(L *lua.LState) int {
	// TODO: implement setRemoveBossTime
	return 0
}

func playerSetserene(L *lua.LState) int {
	// TODO: implement setSerene
	return 0
}

func playerSetsex(L *lua.LState) int {
	// TODO: implement setSex
	return 0
}

func playerSetskilllevel(L *lua.LState) int {
	// TODO: implement setSkillLevel
	return 0
}

func playerSetskulltime(L *lua.LState) int {
	// TODO: implement setSkullTime
	return 0
}

func playerSetspecialcontainersavailable(L *lua.LState) int {
	// TODO: implement setSpecialContainersAvailable
	return 0
}

func playerSetspeed(L *lua.LState) int {
	// TODO: implement setSpeed
	return 0
}

func playerSetstamina(L *lua.LState) int {
	// TODO: implement setStamina
	return 0
}

func playerSetstaminaxpboost(L *lua.LState) int {
	// TODO: implement setStaminaXpBoost
	return 0
}

func playerSetstoragevalue(L *lua.LState) int {
	// TODO: implement setStorageValue
	return 0
}

func playerSettown(L *lua.LState) int {
	// TODO: implement setTown
	return 0
}

func playerSettraining(L *lua.LState) int {
	// TODO: implement setTraining
	return 0
}

func playerSetvirtue(L *lua.LState) int {
	// TODO: implement setVirtue
	return 0
}

func playerSetvocation(L *lua.LState) int {
	// TODO: implement setVocation
	return 0
}

func playerSetvoucherxpboost(L *lua.LState) int {
	// TODO: implement setVoucherXpBoost
	return 0
}

func playerSetxpboostpercent(L *lua.LState) int {
	// TODO: implement setXpBoostPercent
	return 0
}

func playerSetxpboosttime(L *lua.LState) int {
	// TODO: implement setXpBoostTime
	return 0
}

func playerShowtextdialog(L *lua.LState) int {
	// TODO: implement showTextDialog
	return 0
}

func playerTakescreenshot(L *lua.LState) int {
	// TODO: implement takeScreenshot
	return 0
}

func playerTaskhuntingthirdslot(L *lua.LState) int {
	// TODO: implement taskHuntingThirdSlot
	return 0
}

func playerUnlockallcharmrunes(L *lua.LState) int {
	// TODO: implement unlockAllCharmRunes
	return 0
}

func playerUpdateconcoction(L *lua.LState) int {
	// TODO: implement updateConcoction
	return 0
}

func playerUpdatefood(L *lua.LState) int {
	// TODO: implement updateFood
	return 0
}

func playerUpdatekilltracker(L *lua.LState) int {
	// TODO: implement updateKillTracker
	return 0
}

func playerUpdatesupplytracker(L *lua.LState) int {
	// TODO: implement updateSupplyTracker
	return 0
}

func playerUpdateuiexhausted(L *lua.LState) int {
	// TODO: implement updateUIExhausted
	return 0
}

func playerUpgradespellswod(L *lua.LState) int {
	// TODO: implement upgradeSpellsWOD
	return 0
}

func playerWheelunlockscroll(L *lua.LState) int {
	// TODO: implement wheelUnlockScroll
	return 0
}

