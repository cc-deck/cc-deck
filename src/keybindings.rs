use zellij_tile::prelude::*;

use crate::config::PluginConfig;

/// Register all cc-deck keybindings via Zellij's reconfigure API.
///
/// Keybindings are registered in the `shared` section so they work in all modes.
/// Each keybinding sends a `MessagePluginId` to the plugin's pipe handler.
pub fn register_keybindings(plugin_id: u32, config: &PluginConfig) {
    let kdl = format!(
        r#"
keybinds {{
    shared {{
        bind "{picker_key}" {{
            MessagePluginId {id} {{
                name "open_picker"
            }}
        }}
        bind "{new_session_key}" {{
            MessagePluginId {id} {{
                name "new_session"
            }}
        }}
        bind "{rename_key}" {{
            MessagePluginId {id} {{
                name "rename_session"
            }}
        }}
        bind "{close_key}" {{
            MessagePluginId {id} {{
                name "close_session"
            }}
        }}
    }}
}}
"#,
        id = plugin_id,
        picker_key = config.picker_key,
        new_session_key = config.new_session_key,
        rename_key = config.rename_key,
        close_key = config.close_key,
    );

    reconfigure(kdl, false);
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_default_config_generates_valid_keybindings() {
        let config = PluginConfig::default();
        // Verify the config values are as expected
        assert_eq!(config.picker_key, "Ctrl Shift t");
        assert_eq!(config.new_session_key, "Ctrl Shift n");
        assert_eq!(config.rename_key, "Ctrl Shift r");
        assert_eq!(config.close_key, "Ctrl Shift x");
    }
}
