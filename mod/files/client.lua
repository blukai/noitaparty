local ffi = require("ffi")

ffi.cdef([[
typedef unsigned char GoUint8;
typedef int GoInt32;
typedef GoInt32 GoInt;
typedef unsigned long long GoUint64;

typedef struct PlayerIter {} PlayerIter;

typedef struct Int32Vector2 {
	GoInt32 X;
	GoInt32 Y;
} Int32Vector2;
typedef struct TransformPlayer {
	GoUint64     ID;
	Int32Vector2 Transform;
} TransformPlayer;

char* LastErr();
void Connect(char* network, char* address);
GoInt32 SendCCmdJoinRecvSCmdSetSeed(GoUint64 id);
void SendCCmdTransformPlayer(GoUint64 id, GoInt32 x, GoInt32 y);

GoInt IterLen(void* iterPtr);
GoInt IterPos(void* iterPtr);
GoUint8 IterHasNext(void* iterPtr);
void IterFree(void* iterPtr);

TransformPlayer* GetNextPlayerInIter(void* iter_ptr);
PlayerIter* GetPlayerIter();
]])

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
mod.SendCCmdTransformPlayer = client.SendCCmdTransformPlayer

-- GoInt IterLen(void* iterPtr);
mod.IterLen = client.IterLen

-- GoInt IterPos(void* iterPtr);
mod.IterPos = client.IterPos

-- GoUint8 IterHasNext(void* iterPtr);
function mod.IterHasNext(iter_ptr)
	return client.IterHasNext(iter_ptr) == 1
end

-- void IterFree(void* iterPtr);
mod.IterFree = client.IterFree

-- void* GetNextPlayerInIter(void* iter_ptr);
mod.GetNextPlayerInIter = client.GetNextPlayerInIter

-- void* GetPlayerIter();
mod.GetPlayerIter = client.GetPlayerIter

return mod
