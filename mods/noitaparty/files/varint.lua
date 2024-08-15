-- NOTE: in noita bitops is savailable in bit module;
-- see https://noita.wiki.gg/wiki/Modding:_Lua_API
local ok, bit = pcall(require, "bit")
-- NOTE: on fedora i had to install lua-bit32, but it seems like they are the
-- same thing?
if not ok then
	bit = require("bit32")
end

local unpack = table.unpack or unpack

local function zigzag_encode(x)
	return bit.bxor(bit.lshift(x, 1), bit.rshift(x, 31))
end

local function zigzag_decode(x)
	return bit.bxor(bit.rshift(x, 1), -bit.band(x, 1))
end

local mod = {}

function mod.encode_uvarint(value) end

function mod.decode_uvarint(data, pos) end

function mod.encode_varint(value)
	return mod.encode_uvarint(zigzag_encode(value))
end

function mod.decode_varint(value)
	local result, pos = mod.decode_uvarint(value)
	return zigzag_decode(result), pos
end

local encoded = mod.encode_varint(-42)
print("Encoded:", encoded, type(encoded))

local decoded, new_pos = mod.decode_varint(encoded)
print("Decoded:", decoded)

return mod
