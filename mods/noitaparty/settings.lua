-- see <noita>/mods/example/settings.lua for docs

dofile("data/scripts/lib/mod_settings.lua")

local mod_id = "noitaparty"
mod_settings_version = 1
mod_settings = {
	-- {
	-- 	id = "lobby_token",
	-- 	ui_name = "Lobby token",
	-- 	value_default = "",
	-- 	scope = MOD_SETTING_SCOPE_NEW_GAME,
	-- 	text_max_length = 32,
	-- },
	{
		id = "server_address",
		ui_name = "Server address",
		value_default = "noitaparty.ayaya.moe",
		scope = MOD_SETTING_SCOPE_NEW_GAME,
		text_max_length = 32,
	},
}

function ModSettingsUpdate(init_scope)
	local old_version = mod_settings_get_version(mod_id)
	mod_settings_update(mod_id, mod_settings, init_scope)
end

function ModSettingsGuiCount()
	return mod_settings_gui_count(mod_id, mod_settings)
end

function ModSettingsGui(gui, in_main_menu)
	mod_settings_gui(mod_id, mod_settings, gui, in_main_menu)
end
