local steam_api = dofile_once("mods/noitaparty/files/steam_api.lua")
-- TODO(blukai): rename client into something more verbose (like lobbyclient or
-- something)
local client = dofile_once("mods/noitaparty/files/client.lua")

-- NOTE(blukai): might need this later
-- dofile_once("data/scripts/lib/utilities.lua")

local STEAM_ID = nil

-- TODO(blukai): find a nicer way to do error reporting, more sane
local UNPRINTED_ERR = nil
local CRITICAL_ERROR_ENDING = ". can't continue. seek help!"

local LAST_PLAYER_X = nil
local LAST_PLAYER_Y = nil

-- TODO(blukai): introduce some kind of global state "object" that would be more
-- convenient to deal with then a bunch of individual globals.
-- KUMMITUS_ENTITY = nil
-- KUMMITUS_ENTITY = EntityLoad("mods/noitaparty/files/kummitus.xml")
-- local player_x, player_y = EntityGetTransform(player_entity)
-- EntitySetTransform(KUMMITUS_ENTITY, player_x - 10, player_y - 10)

local function get_player_entity()
	local players = EntityGetWithTag("player_unit")
	-- NOTE(blukai): there may be more than one player for some reason. on
	-- the internet people just assume that the first item in an array is
	-- the one.
	return players[1]
end

-- Called in order upon loading a new(?) game:
function OnModPreInit()
	STEAM_ID = steam_api.ISteamUser.GetSteamID()
	if STEAM_ID == nil then
		UNPRINTED_ERR = "could not get steam id" .. CRITICAL_ERROR_ENDING
		print(UNPRINTED_ERR)
		return
	end

	local connect_err = client.Connect("udp4", "127.0.0.1:8008")
	if connect_err ~= nil then
		UNPRINTED_ERR = "could not connect: " .. connect_err .. CRITICAL_ERROR_ENDING
		print(UNPRINTED_ERR)
		return
	end

	local seed, seed_err = client.SendCCmdJoinRecvSCmdSetSeed(STEAM_ID)
	if seed_err ~= nil then
		UNPRINTED_ERR = "could not get server seed: " .. seed_err .. CRITICAL_ERROR_ENDING
		print(UNPRINTED_ERR)
		return
	end
	SetWorldSeed(seed)
end

function OnModInit() end

function OnModPostInit() end

-- Called when player entity has been created. Ensures chunks around the player have been loaded & created.
function OnPlayerSpawned(player_entity) end

-- Called when the player dies
function OnPlayerDied(player_entity) end

-- Called once the game world is initialized. Doesn't ensure any chunks around the player.
function OnWorldInitialized() end

-- Called *every* time the game is about to start updating the world
function OnWorldPreUpdate() end

-- Called *every* time the game has finished updating the world
function OnWorldPostUpdate()
	if STEAM_ID == nil then
		return
	end

	if UNPRINTED_ERR ~= nil then
		GamePrintImportant("noitaparty error", UNPRINTED_ERR)
		UNPRINTED_ERR = nil
	end

	local last_err = client.LastErr()
	if client.LastErr() ~= nil then
		GamePrint("noitaparty error: " .. last_err)
		return
	end

	local player_entity = get_player_entity()
	if player_entity ~= nil then
		local x, y = EntityGetTransform(player_entity)
		-- NOTE(blukai): it turns out that x and y are actually floats..
		-- do we care about precision?
		x, y = math.floor(x), math.floor(y)
		if x ~= LAST_PLAYER_X or y ~= LAST_PLAYER_Y then
			client.SendCCmdTransformPlayer(STEAM_ID, x, y)
			LAST_PLAYER_X, LAST_PLAYER_Y = x, y
		end
	end

	-- TODO(blukai): draw other players !
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
