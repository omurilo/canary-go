local function trim(s)
	return (s:gsub("^%s*(.-)%s*$", "%1"))
end

local function parseParam(param)
	if not param or param == "" then
		return "", ""
	end
	local key, targetStr = param:match("^%s*([^%s,]+)%s*[,%s]%s*(.-)%s*$")
	if not key then
		key = trim(param)
		targetStr = ""
	else
		key = trim(key)
		targetStr = trim(targetStr)
	end
	return key:lower(), targetStr
end

local itemFunctions = {
	["actionid"] = {
		isActive = true,
		targetFunction = function(item, target)
			local num = tonumber(target)
			if not num then return false, "A numeric value is required." end
			return item:setActionId(num)
		end,
	},
	["action"] = {
		isActive = true,
		targetFunction = function(item, target)
			local num = tonumber(target)
			if not num then return false, "A numeric value is required." end
			return item:setActionId(num)
		end,
	},
	["aid"] = {
		isActive = true,
		targetFunction = function(item, target)
			local num = tonumber(target)
			if not num then return false, "A numeric value is required." end
			return item:setActionId(num)
		end,
	},
	["uniqueid"] = {
		isActive = true,
		targetFunction = function(item, target)
			local num = tonumber(target)
			if not num then return false, "A numeric value is required." end
			return item:setAttribute(ITEM_ATTRIBUTE_UNIQUEID, num)
		end,
	},
	["unique"] = {
		isActive = true,
		targetFunction = function(item, target)
			local num = tonumber(target)
			if not num then return false, "A numeric value is required." end
			return item:setAttribute(ITEM_ATTRIBUTE_UNIQUEID, num)
		end,
	},
	["uid"] = {
		isActive = true,
		targetFunction = function(item, target)
			local num = tonumber(target)
			if not num then return false, "A numeric value is required." end
			return item:setAttribute(ITEM_ATTRIBUTE_UNIQUEID, num)
		end,
	},
	["description"] = {
		isActive = true,
		targetFunction = function(item, target)
			if not target or target == "" then return false, "A text value is required." end
			return item:setAttribute(ITEM_ATTRIBUTE_DESCRIPTION, tostring(target))
		end,
	},
	["desc"] = {
		isActive = true,
		targetFunction = function(item, target)
			if not target or target == "" then return false, "A text value is required." end
			return item:setAttribute(ITEM_ATTRIBUTE_DESCRIPTION, tostring(target))
		end,
	},
	["name"] = {
		isActive = true,
		targetFunction = function(item, target)
			if not target or target == "" then return false, "A text value is required." end
			return item:setAttribute(ITEM_ATTRIBUTE_NAME, tostring(target))
		end,
	},
	["remove"] = {
		isActive = true,
		targetFunction = function(item, target)
			return item:remove()
		end,
	},
	["decay"] = {
		isActive = true,
		targetFunction = function(item, target)
			return item:decay()
		end,
	},
	["transform"] = {
		isActive = true,
		targetFunction = function(item, target)
			local num = tonumber(target)
			if not num then return false, "A numeric item ID is required." end
			return item:transform(num)
		end,
	},
	["clone"] = {
		isActive = true,
		targetFunction = function(item, target)
			return item:clone()
		end,
	},
	["attack"] = {
		isActive = true,
		targetFunction = function(item, target)
			local num = tonumber(target)
			if not num then return false, "A numeric value is required." end
			return item:setAttribute(ITEM_ATTRIBUTE_ATTACK, num)
		end,
	},
	["defense"] = {
		isActive = true,
		targetFunction = function(item, target)
			local num = tonumber(target)
			if not num then return false, "A numeric value is required." end
			return item:setAttribute(ITEM_ATTRIBUTE_DEFENSE, num)
		end,
	},
	["extradefense"] = {
		isActive = true,
		targetFunction = function(item, target)
			local num = tonumber(target)
			if not num then return false, "A numeric value is required." end
			return item:setAttribute(ITEM_ATTRIBUTE_EXTRADEFENSE, num)
		end,
	},
	["charge"] = {
		isActive = true,
		targetFunction = function(item, target)
			local num = tonumber(target)
			if not num then return false, "A numeric value is required." end
			return item:setAttribute(ITEM_ATTRIBUTE_CHARGES, num)
		end,
	},
	["armor"] = {
		isActive = true,
		targetFunction = function(item, target)
			local num = tonumber(target)
			if not num then return false, "A numeric value is required." end
			return item:setAttribute(ITEM_ATTRIBUTE_ARMOR, num)
		end,
	},
}

local creatureFunctions = {
	["health"] = {
		isActive = true,
		targetFunction = function(creature, target)
			local num = tonumber(target)
			if not num then return false, "A numeric value is required." end
			return creature:addHealth(num)
		end,
	},
	["mana"] = {
		isActive = true,
		targetFunction = function(creature, target)
			local num = tonumber(target)
			if not num then return false, "A numeric value is required." end
			return creature:addMana(num)
		end,
	},
	["speed"] = {
		isActive = true,
		targetFunction = function(creature, target)
			local num = tonumber(target)
			if not num then return false, "A numeric value is required." end
			return creature:changeSpeed(num)
		end,
	},
	["droploot"] = {
		isActive = true,
		targetFunction = function(creature, target)
			local val = target == "true" or target == "1"
			return creature:setDropLoot(val)
		end,
	},
	["skull"] = {
		isActive = true,
		targetFunction = function(creature, target)
			local num = tonumber(target)
			if not num then return false, "A numeric value is required." end
			return creature:setSkull(num)
		end,
	},
	["direction"] = {
		isActive = true,
		targetFunction = function(creature, target)
			local num = tonumber(target)
			if not num then return false, "A numeric value is required." end
			return creature:setDirection(num)
		end,
	},
	["maxHealth"] = {
		isActive = true,
		targetFunction = function(creature, target)
			local num = tonumber(target)
			if not num then return false, "A numeric value is required." end
			return creature:setMaxHealth(num)
		end,
	},
	["say"] = {
		isActive = true,
		targetFunction = function(creature, target)
			if not target or target == "" then return false, "A text message is required." end
			creature:say(tostring(target), TALKTYPE_SAY)
			return true
		end,
	},
}

local playerFunctions = {
	["fyi"] = {
		isActive = true,
		targetFunction = function(player, target)
			if not target or target == "" then return false, "A text message is required." end
			return player:popupFYI(tostring(target))
		end,
	},
	["tutorial"] = {
		isActive = true,
		targetFunction = function(player, target)
			local num = tonumber(target)
			if not num then return false, "A numeric tutorial ID is required." end
			return player:sendTutorial(num)
		end,
	},
	["guildnick"] = {
		isActive = true,
		targetFunction = function(player, target)
			return player:setGuildNick(tostring(target or ""))
		end,
	},
	["group"] = {
		isActive = true,
		targetFunction = function(player, target)
			local num = tonumber(target)
			if not num then return false, "A numeric group ID is required." end
			local grp = Group and Group(num) or nil
			if not grp then return false, "Group not found." end
			return player:setGroup(grp)
		end,
	},
	["vocation"] = {
		isActive = true,
		targetFunction = function(player, target)
			local num = tonumber(target)
			if not num then return false, "A numeric vocation ID is required." end
			local voc = Vocation and Vocation(num) or nil
			if not voc then return false, "Vocation not found." end
			return player:setVocation(voc)
		end,
	},
	["stamina"] = {
		isActive = true,
		targetFunction = function(player, target)
			local num = tonumber(target)
			if not num then return false, "A numeric value is required." end
			return player:setStamina(num)
		end,
	},
	["town"] = {
		isActive = true,
		targetFunction = function(player, target)
			local num = tonumber(target)
			if not num then return false, "A numeric town ID is required." end
			local twn = Town and Town(num) or nil
			if not twn then return false, "Town not found." end
			return player:setTown(twn)
		end,
	},
	["balance"] = {
		isActive = true,
		targetFunction = function(player, target)
			local num = tonumber(target)
			if not num then return false, "A numeric amount is required." end
			return player:setBankBalance(num + player:getBankBalance())
		end,
	},
	["save"] = {
		isActive = true,
		targetFunction = function(player, target)
			return player:save()
		end,
	},
	["type"] = {
		isActive = true,
		targetFunction = function(player, target)
			local num = tonumber(target)
			if not num then return false, "A numeric account type is required." end
			return player:setAccountType(num)
		end,
	},
	["skullTime"] = {
		isActive = true,
		targetFunction = function(player, target)
			local num = tonumber(target)
			if not num then return false, "A numeric skull time is required." end
			return player:setSkullTime(num)
		end,
	},
	["maxMana"] = {
		isActive = true,
		targetFunction = function(player, target)
			local num = tonumber(target)
			if not num then return false, "A numeric value is required." end
			return player:setMaxMana(num)
		end,
	},
	["maxHealth"] = {
		isActive = true,
		targetFunction = function(player, target)
			local num = tonumber(target)
			if not num then return false, "A numeric value is required." end
			return player:setMaxHealth(num)
		end,
	},
	["addItem"] = {
		isActive = true,
		targetFunction = function(player, target)
			local num = tonumber(target)
			if not num then return false, "A numeric item ID is required." end
			return player:addItem(num, 1)
		end,
	},
	["removeItem"] = {
		isActive = true,
		targetFunction = function(player, target)
			local num = tonumber(target)
			if not num then return false, "A numeric item ID is required." end
			return player:removeItem(num, 1)
		end,
	},
	["premium"] = {
		isActive = true,
		targetFunction = function(player, target)
			local num = tonumber(target)
			if not num then return false, "A numeric number of days is required." end
			return player:addPremiumDays(num)
		end,
	},
}

local attributes = TalkAction("/attr")

function attributes.onSay(player, words, param)
	-- create log
	logCommand(player, words, param)

	if not param or param == "" then
		player:sendCancelMessage("Command param required. Example: /attr actionid 100")
		return true
	end

	local key, targetStr = parseParam(param)
	if key == "" then
		player:sendCancelMessage("Command param required. Example: /attr actionid 100")
		return true
	end

	local itemFunction = itemFunctions[key]
	local creatureFunction = creatureFunctions[key]
	local playerFunction = playerFunctions[key]

	local position = player:getPosition()
	position:getNextPosition(player:getDirection(), 1)

	local tile = Tile(position)
	if not tile then
		player:sendCancelMessage("Tile not found.")
		return true
	end

	if itemFunction and itemFunction.isActive then
		local item = tile:getTopVisibleThing(player)
		if not item or not item:isItem() then
			player:sendCancelMessage("Item not found.")
			return true
		end
		local ok, result, errMsg = pcall(itemFunction.targetFunction, item, targetStr)
		if ok and (result == true or result == nil) then
			position:sendMagicEffect(CONST_ME_MAGIC_GREEN)
		else
			local msg = (type(result) == "string" and result) or errMsg or "You cannot add that attribute to this item."
			player:sendCancelMessage(msg)
		end
	elseif creatureFunction and creatureFunction.isActive then
		local creature = tile:getTopCreature()
		if not creature or not creature:isCreature() then
			player:sendCancelMessage("Creature not found.")
			return true
		end
		local ok, result, errMsg = pcall(creatureFunction.targetFunction, creature, targetStr)
		if ok and (result == true or result == nil) then
			position:sendMagicEffect(CONST_ME_MAGIC_GREEN)
		else
			local msg = (type(result) == "string" and result) or errMsg or "You cannot add that attribute to this creature."
			player:sendCancelMessage(msg)
		end
	elseif playerFunction and playerFunction.isActive then
		local targetPlayer = tile:getTopCreature()
		if not targetPlayer or not targetPlayer:getPlayer() then
			player:sendCancelMessage("Player not found.")
			return true
		end
		local ok, result, errMsg = pcall(playerFunction.targetFunction, targetPlayer, targetStr)
		if ok and (result == true or result == nil) then
			position:sendMagicEffect(CONST_ME_MAGIC_GREEN)
		else
			local msg = (type(result) == "string" and result) or errMsg or "You cannot add that attribute to this player."
			player:sendCancelMessage(msg)
		end
	else
		player:sendCancelMessage("Unknown attribute: " .. key)
	end
	return true
end

attributes:separator(" ")
attributes:groupType("god")
attributes:register()
