function CheckNamelock(player)
	if not player or not player:isPlayer() then
		return true
	end
	local kvStore = player:kv()
	if not kvStore then
		return true
	end
	local namelockReason = kvStore:get("namelock")
	if not namelockReason then
		return true
	end

	player:setMoveLocked(true)
	player:teleportTo(player:getTown():getTemplePosition())
	player:sendTextMessage(MESSAGE_ADMINISTRATOR, "Your name has been locked for the following reason: " .. namelockReason .. ".")
	player:openStore("extras")
	addPlayerEvent(sendRequestPurchaseData, 50, player, 65002, GameStore.ClientOfferTypes.CLIENT_STORE_OFFER_NAMECHANGE)
	addPlayerEvent(CheckNamelock, 30000, player)
end

local playerLogin = CreatureEvent("NamelockLogin")

function playerLogin.onLogin(player)
	addPlayerEvent(CheckNamelock, 1000, player)
	return true
end

playerLogin:register()
