local ffi = require("ffi")

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

local client = ffi.load("mods/noitaparty/files/client.dll")

local function cstring(str)
	local dst = ffi.new("char[?]", #str + 1)
	ffi.copy(dst, str)
	return dst
end

local mod = {}

-- char* LastErr();
function mod.LastErr()
	local last_err = client.LastErr()
	if last_err ~= nil then
		return ffi.string(last_err)
	end
	return nil
end

-- void Connect(char* network, char* address);
function mod.Connect(network, address)
	client.Connect(cstring(network), cstring(address))
	return mod.LastErr()
end

-- GoInt32 SendCCmdJoinRecvSCmdSetSeed(GoUint64 id);
function mod.SendCCmdJoinRecvSCmdSetSeed(id)
	local set_seed = client.SendCCmdJoinRecvSCmdSetSeed(id)
	return set_seed, mod.LastErr()
end

-- void SendCCmdTransformPlayer(GoUint64 id, GoInt32 x, GoInt32 y);
function mod.SendCCmdTransformPlayer(id, x, y)
	local co = coroutine.create(function()
		client.SendCCmdTransformPlayer(id, x, y)
	end)
	coroutine.resume(co)
end

-- GoSlice GetPlayers();
function mod.GetPlayers()
	local players = client.GetPlayers()
	-- TODO(blukai): figure out players representation
	print(type(players))
end

return mod
