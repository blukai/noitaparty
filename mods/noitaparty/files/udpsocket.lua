local ffi = require("ffi")

local function read_file(path)
	local file = io.open(path, "r")
	if not file then
		return nil, "Unable to open file: " .. path
	end
	local contents = file:read("*a")
	file:close()
	return contents
end

local clib = ffi.load("mods/noitaparty/files/udpsocket.dll")
ffi.cdef(read_file("mods/noitaparty/files/udpsocket.h"))

local function toerr(err)
	if err == nil then
		return nil
	end

	local buf_len = 1024
	local buf = ffi.new("uint8_t[?]", buf_len)
	local n = clib.udpsocket_error_print(err, buf, buf_len)

	return ffi.string(buf, n)
end

local function gcerr(err)
	if err ~= nil then
		clib.udpsocket_error_drop(err)
	end
end

local mod = {}

mod.bind = function(bind_addr)
	local udpsocket_out = ffi.new("void*[1]")
	local bind_err = ffi.gc(clib.udpsocket_bind(bind_addr, udpsocket_out), function(err)
		if err ~= nil then
			clib.udpsocket_error_drop(err)
		else
			clib.udpsocket_drop(udpsocket_out[0])
		end
	end)
	bind_err = toerr(bind_err)
	if bind_err ~= nil then
		return nil, bind_err
	end

	local udpsocket_mt = {
		set_nonblocking = function(self, nonblocking)
			local err = ffi.gc(clib.udpsocket_set_nonblocking(self.udpsocket, nonblocking), gcerr)
			return toerr(err)
		end,
		connect = function(self, connect_addr)
			local err = ffi.gc(clib.udpsocket_connect(self.udpsocket, connect_addr), gcerr)
			return toerr(err)
		end,
		send = function(self, buf, buf_len)
			local n = ffi.new("size_t[1]")
			local err = ffi.gc(clib.udpsocket_send(self.udpsocket, buf, buf_len, n), gcerr)
			return tonumber(n[0]), toerr(err)
		end,
		recv = function(self, buf, buf_len)
			local n = ffi.new("size_t[1]")
			local err = ffi.gc(clib.udpsocket_recv(self.udpsocket, buf, buf_len, n), gcerr)
			return tonumber(n[0]), toerr(err)
		end,
	}
	return setmetatable({ udpsocket = udpsocket_out[0] }, { __index = udpsocket_mt }), nil
end

return mod
