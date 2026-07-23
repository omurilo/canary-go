-- Script for set teleport destination
-- /teleport xxxx, xxxx, x
local teleportSetDestination = TalkAction("/teleport", "/tp")

function teleportSetDestination.onSay(player, words, param)
	-- create log
	logCommand(player, words, param)

	if param == "" then
		player:sendCancelMessage("Command param required.")
		return true
	end

	local params = param:split(",")
	if params[3] then
		local position = player:getPosition()
		position:getNextPosition(player:getDirection(), 1)
		local x, y, z = tonumber(params[1]), tonumber(params[2]), tonumber(params[3])
		local destination = (x and y and z) and Position(x, y, z) or nil
		if destination then
			player:teleportTo(destination)
			if not player:isInGhostMode() then
				destination:sendMagicEffect(CONST_ME_TELEPORT)
			end
			local tp = Game.createItem(35502, 1, position)
			if tp then
				tp:setDestination(destination)
			end
			player:sendTextMessage(MESSAGE_EVENT_ADVANCE, string.format("New position: %s", param))
		else
			player:sendCancelMessage("Destination position is not valid.")
		end
	else
		player:sendCancelMessage('You need to declare the X, Y of Z of destination. Please use "/teleport X, Y, Z".')
	end
	return true
end

teleportSetDestination:separator(" ")
teleportSetDestination:groupType("gamemaster")
teleportSetDestination:register()
