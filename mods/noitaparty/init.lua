local ffi = require("ffi")

local steam_api = dofile_once("mods/noitaparty/files/steam_api.lua")
local udpsocket = dofile_once("mods/noitaparty/files/udpsocket.lua")

-- NOTE(blukai): might need this later
-- dofile_once("data/scripts/lib/utilities.lua")

local STEAM_ID = nil
local PERSONA_NAME = nil

-- TODO(blukai): find a nicer way to do error reporting, more sane
local INIT_ERR = nil
local CRITICAL_ERROR_ENDING = ". can't continue. seek help!"

local LAST_PLAYER_X = nil
local LAST_PLAYER_Y = nil

local UDPSOCK_BUF = ffi.new("uint8_t[256]")

local function get_player_entity()
	local players = EntityGetWithTag("player_unit")
	-- NOTE(blukai): there may be more than one player for some reason. on
	-- the internet people just assume that the first item in an array is
	-- the one.
	return players[1]
end

-- Called in order upon loading a new(?) game:
function OnModPreInit() end

function OnModInit()
	STEAM_ID = steam_api.ISteamUser.GetSteamID()
	if STEAM_ID == nil then
		INIT_ERR = "could not get steam id" .. CRITICAL_ERROR_ENDING
		print(INIT_ERR)
		return
	end

	PERSONA_NAME = steam_api.ISteamFriends.GetPersonaName()
	if PERSONA_NAME == nil then
		INIT_ERR = "could not get persona name" .. CRITICAL_ERROR_ENDING
		print(INIT_ERR)
		return
	end

	local socket, bind_err = udpsocket.bind("127.0.0.1:34254")
	if bind_err ~= nil then
		INIT_ERR = "could not bind socket: " .. bind_err
		print(INIT_ERR)
		return
	end

	local connect_err = socket:connect("127.0.0.1:5000")
	if connect_err ~= nil then
		INIT_ERR = "could not connect socket: " .. connect_err
		print(INIT_ERR)
		return
	end

	-- TODO: message encoding/decoding
	UDPSOCK_BUF[0] = 1
	local n, send_err = socket:send(UDPSOCK_BUF, 1)
	if send_err ~= nil then
		INIT_ERR = "could not send: " .. send_err
		print(INIT_ERR)
		return
	end
end

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
	if INIT_ERR ~= nil then
		GamePrintImportant("noitaparty error", INIT_ERR)
		INIT_ERR = nil
	end

	-- local player_entity = get_player_entity()
	-- if player_entity ~= nil then
	-- 	local x, y = EntityGetTransform(player_entity)
	-- 	-- NOTE(blukai): it turns out that x and y are actually floats..
	-- 	-- do we care about precision?
	-- 	x, y = math.floor(x), math.floor(y)
	-- 	if x ~= LAST_PLAYER_X or y ~= LAST_PLAYER_Y then
	-- 		client.SendCCmdTransformPlayer(STEAM_ID, x, y)
	-- 		LAST_PLAYER_X, LAST_PLAYER_Y = x, y
	-- 	end
	-- end
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
