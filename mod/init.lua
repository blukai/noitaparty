local ffi = require("ffi")

local client = ffi.load("mods/noitaparty/files/client.dll")

-- http://lua-users.org/wiki/StringRecipes
local function string_starts_with(str, start)
	return str:sub(1, #start) == start
end

-- NOTE(blukai): lua's c language support is not complete..
-- see https://luajit.org/ext_ffi_semantics.html#clang
-- thus certain things need to be removed.
-- what is removed?
-- - #ifdef .. #endif
-- what is kept?
-- - lines that start with typedef, extern
local function cdef_header_file(filename)
	local file = io.open(filename, "r")
	assert(file ~= nil)

	local lines = {}
	local inside_ifdef = false
	for line in file:lines() do
		if inside_ifdef then
			inside_ifdef = not string_starts_with(line, "#endif")
		elseif string_starts_with(line, "#ifdef") then
			inside_ifdef = true
		elseif string_starts_with(line, "typedef") or string_starts_with(line, "extern") then
			table.insert(lines, line)
		end
	end

	file:close()
	ffi.cdef(table.concat(lines, "\n"))
end

cdef_header_file("mods/noitaparty/files/client.h")

local function cstring(str)
	local dst = ffi.new("char[?]", #str + 1)
	ffi.copy(dst, str)
	return dst
end

client.Connect(cstring("udp4"), cstring("127.0.0.1:8008"))
print("CONNECTED!!!!!!!!!!!!!!!!!!!!!!")
print(type(client.LastErr()))

client.SendPing()
print("SENT PING!!!!!!!!!!!!!!!!!!!!!!!!!!")
print(type(client.LastErr()))

client.Disconnect()
print("DISCONNECTED!!!!!!!!!!!!!!!!!!!!!!!!!!")
print(type(client.LastErr()))

-- NOTE(blukai): might need this later
-- dofile_once("data/scripts/lib/utilities.lua")

-- TODO(blukai): introduce some kind of global state "object" that would be more
-- convenient to deal with then a bunch of individual globals.
KUMMITUS_ENTITY = nil

local function get_player_entity()
	local players = EntityGetWithTag("player_unit")
	-- NOTE(blukai): there may be more than one player for some reason. on
	-- the internet people just assume that the first item in an array is
	-- the one.
	return players[1]
end

-- Called in order upon loading a new(?) game:
function OnModPreInit()
	-- TODO(blukai): connect to server and set world seed (SetWorldSeed)...
	-- maybe blocking server connection ui (with address input).
end

function OnModInit() end

function OnModPostInit() end

-- Called when player entity has been created. Ensures chunks around the player have been loaded & created.
function OnPlayerSpawned(player_entity)
	KUMMITUS_ENTITY = EntityLoad("mods/noitaparty/files/kummitus.xml")
end

-- Called when the player dies
function OnPlayerDied(player_entity) end

-- Called once the game world is initialized. Doesn't ensure any chunks around the player.
function OnWorldInitialized() end

-- Called *every* time the game is about to start updating the world
function OnWorldPreUpdate() end

-- Called *every* time the game has finished updating the world
function OnWorldPostUpdate()
	local player_entity = get_player_entity()
	if player_entity ~= nil then
		local player_x, player_y = EntityGetTransform(player_entity)
		EntitySetTransform(KUMMITUS_ENTITY, player_x - 10, player_y - 10)
	end
end

-- Called when the biome config is loaded.
function OnBiomeConfigLoaded() end

-- The last point where the Mod API is available. After this materials.xml will be loaded.
function OnMagicNumbersAndWorldSeedInitialized() end

-- Called when the game is paused or unpaused.
function OnPausedChanged(is_paused, is_inventory_pause) end

-- Will be called when the game is unpaused, if player changed any mod settings while the game was paused
function OnModSettingsChanged() end

-- Will be called when the game is paused, either by the pause menu or some inventory menus. Please be careful with this, as not everything will behave well when called while the game is paused.
function OnPausePreUpdate() end
