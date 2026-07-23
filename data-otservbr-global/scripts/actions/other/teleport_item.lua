-- Script for items that teleport when giving use
-- Add a new item in the action_unique table at the correct range

local teleportItem = Action()

function teleportItem.onUse(player, item, fromPosition, target, toPosition, isHotkey)
	local setting = TeleportItemUnique[item.uid] or TeleportItemAction[item.actionid]
	if not (setting and setting.destination) then
		setting = nil
		for _, v in pairs(TeleportItemUnique) do
			if v.destination and v.itemPos and v.itemPos.x == fromPosition.x and v.itemPos.y == fromPosition.y and v.itemPos.z == fromPosition.z then
				setting = v
				break
			end
		end
	end
	if not (setting and setting.destination) then
		setting = nil
		for _, v in pairs(TeleportItemAction) do
			if v.destination and v.itemPos then
				if type(v.itemPos) == "table" and v.itemPos.x == fromPosition.x and v.itemPos.y == fromPosition.y and v.itemPos.z == fromPosition.z then
					setting = v
					break
				elseif type(v.itemPos) == "table" then
					for _, pos in ipairs(v.itemPos) do
						if type(pos) == "table" and pos.x == fromPosition.x and pos.y == fromPosition.y and pos.z == fromPosition.z then
							setting = v
							break
						end
					end
				end
			end
			if setting and setting.destination then break end
		end
	end

	if setting and setting.destination then
		player:teleportTo(setting.destination)
		local eff = setting.effect or CONST_ME_TELEPORT
		player:getPosition():sendMagicEffect(eff)
	end
	return true
end

for uniqueRange = 15001, 20000 do
	teleportItem:uid(uniqueRange)
end

teleportItem:aid(30255)
teleportItem:id(1759, 31673)

teleportItem:register()
