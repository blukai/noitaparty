local ffi = require("ffi")

-- NOTE(blukai): some of those defs come from steamworks sdk; some others are
-- hackery.
ffi.cdef([[
typedef int32_t int32;
typedef uint64_t uint64;

typedef struct ISteamClient {} ISteamClient;
typedef int32 HSteamPipe;
typedef int32 HSteamUser;
typedef struct ISteamUser {} ISteamUser;
typedef uint64 uint64_steamid;
typedef struct ISteamFriends {} ISteamFriends;

ISteamClient * SteamClient();
HSteamPipe SteamAPI_ISteamClient_CreateSteamPipe( ISteamClient* self );
HSteamUser SteamAPI_ISteamClient_ConnectToGlobalUser( ISteamClient* self, HSteamPipe hSteamPipe );

ISteamUser * SteamAPI_ISteamClient_GetISteamUser( ISteamClient* self, HSteamUser hSteamUser, HSteamPipe hSteamPipe, const char * pchVersion );
ISteamFriends * SteamAPI_ISteamClient_GetISteamFriends( ISteamClient* self, HSteamUser hSteamUser, HSteamPipe hSteamPipe, const char * pchVersion );

uint64_steamid SteamAPI_ISteamUser_GetSteamID( ISteamUser* self );
const char * SteamAPI_ISteamFriends_GetPersonaName( ISteamFriends* self );
]])

-- NOTE(blukai): steam_api.dll is provided by noita
local steam_api = ffi.load("steam_api.dll")

local iSteamClient = steam_api.SteamClient()
assert(iSteamClient ~= nil)

local hSteamPipe = steam_api.SteamAPI_ISteamClient_CreateSteamPipe(iSteamClient)
assert(hSteamPipe ~= nil)

local hSteamUser = steam_api.SteamAPI_ISteamClient_ConnectToGlobalUser(iSteamClient, hSteamPipe)
assert(hSteamUser ~= nil)

local iSteamUser = steam_api.SteamAPI_ISteamClient_GetISteamUser(iSteamClient, hSteamUser, hSteamPipe, "SteamUser019")
assert(iSteamUser ~= nil)

local iSteamFriends =
	steam_api.SteamAPI_ISteamClient_GetISteamFriends(iSteamClient, hSteamUser, hSteamPipe, "SteamFriends015")
assert(iSteamFriends ~= nil)

local mod = {}

mod.ISteamUser = {}
function mod.ISteamUser.GetSteamID()
	return steam_api.SteamAPI_ISteamUser_GetSteamID(iSteamUser)
end

mod.ISteamFriends = {}
function mod.ISteamFriends.GetPersonaName()
	local persona_name = steam_api.SteamAPI_ISteamFriends_GetPersonaName(iSteamFriends)
	if persona_name ~= nil then
		return ffi.string(persona_name)
	end
	return nil
end

return mod
