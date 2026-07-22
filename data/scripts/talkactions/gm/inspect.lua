local inspect = TalkAction("/inspect")

function inspect.onSay(player, words, param)
	logCommand(player, words, param)

	if param == "" then
		player:sendCancelMessage("Command param required. Usage: /inspect PlayerName")
		return true
	end

	local target = Player(param)
	if not target then
		player:sendCancelMessage("Player not found or offline.")
		return true
	end

	local slotNames = {
		[1] = "Head (Helmet)",
		[2] = "Neck (Amulet)",
		[3] = "Backpack",
		[4] = "Body (Armor)",
		[5] = "Right Hand",
		[6] = "Left Hand",
		[7] = "Legs",
		[8] = "Feet (Boots)",
		[9] = "Ring",
		[10] = "Ammo / Extra"
	}

	local text = "=== Player Inspection: " .. target:getName() .. " ===\n\n"
	text = text .. "Level: " .. target:getLevel() .. "\n"
	text = text .. "Vocation: " .. target:getVocation():getName() .. "\n"
	text = text .. "Health: " .. target:getHealth() .. " / " .. target:getMaxHealth() .. "\n"
	text = text .. "Mana: " .. target:getMana() .. " / " .. target:getMaxMana() .. "\n"
	text = text .. "Speed: " .. target:getSpeed() .. "\n"
	text = text .. "Capacity: " .. (target:getCapacity() / 100) .. " oz\n"
	text = text .. "Position: " .. string.format("(%d, %d, %d)", target:getPosition().x, target:getPosition().y, target:getPosition().z) .. "\n"
	text = text .. "IP: " .. Game.convertIpToString(target:getIp()) .. "\n\n"

	text = text .. "--- Skills ---\n"
	text = text .. "Magic Level: " .. target:getMagicLevel() .. "\n"
	text = text .. "Fist: " .. target:getSkillLevel(SKILL_FIST) .. " | Club: " .. target:getSkillLevel(SKILL_CLUB) .. "\n"
	text = text .. "Sword: " .. target:getSkillLevel(SKILL_SWORD) .. " | Axe: " .. target:getSkillLevel(SKILL_AXE) .. "\n"
	text = text .. "Distance: " .. target:getSkillLevel(SKILL_DISTANCE) .. " | Shielding: " .. target:getSkillLevel(SKILL_SHIELD) .. "\n\n"

	text = text .. "--- Equipped Items ---\n"
	for slot = 1, 10 do
		local item = target:getSlotItem(slot)
		if item and item.itemid > 0 then
			local countStr = ""
			if item:getCount() > 1 then
				countStr = " (" .. item:getCount() .. "x)"
			end
			text = text .. slotNames[slot] .. ": " .. item:getName() .. countStr .. "\n"
		else
			text = text .. slotNames[slot] .. ": Empty\n"
		end
	end

	player:popupFYI(text)
	return true
end

inspect:separator(" ")
inspect:groupType("gamemaster")
inspect:register()
