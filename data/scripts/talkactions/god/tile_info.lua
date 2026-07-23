local tileInfo = TalkAction("/tile", "/iteminfo", "/door")

function tileInfo.onSay(player, words, param)
	local pos = player:getPosition()
	pos:getNextPosition(player:getDirection(), 1)

	local tile = Tile(pos)
	if not tile then
		player:sendCancelMessage("Tile not found.")
		return true
	end

	local text = string.format("Tile (%d, %d, %d):\n", pos.x, pos.y, pos.z)

	local ground = tile:getGround()
	if ground then
		text = text .. string.format("[Ground] ID: %d | AID: %d | UID: %d\n", ground:getId(), ground:getActionId(), ground:getUniqueId())
	end

	local items = tile:getItems()
	if items and #items > 0 then
		for _, item in ipairs(items) do
			local name = item:getName()
			if name == "" then name = "Item" end
			text = text .. string.format("[%s] ID: %d | AID: %d | UID: %d\n", name, item:getId(), item:getActionId(), item:getUniqueId())
		end
	end

	player:sendTextMessage(MESSAGE_EVENT_ADVANCE, text)
	return true
end

tileInfo:separator(" ")
tileInfo:groupType("god")
tileInfo:register()
