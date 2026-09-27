-- Lars-Win-AI — WezTerm-Konfiguration (WS-A)
-- Vertrag: docs/CONTRACTS.md §7. Getestet mit WezTerm 20240203 (Windows).
--
-- Start (durch Daemon/Installer, NICHT hier erzwingen):
--   wezterm.exe [--config-file <repo>\config\wezterm\wezterm.lua] start -- herdr
-- Der feste Fenstertitel "AI-Assistant" wird hier erzwungen, damit der Daemon
-- das Fenster ueber (Titel + Prozess wezterm-gui.exe) findet.

local wezterm = require 'wezterm'
local act = wezterm.action

-- Fester Fenstertitel (statt dynamisch aus Tab/Prozess).
wezterm.on('format-window-title', function()
  return 'AI-Assistant'
end)

-- Vi-artige Copy-Mode-Tastentabelle (h/j/k/l, w/b/e, v/y, /?/n/N, ...).
local copy_mode = {
  -- Verlassen (i / Ctrl+C / Ctrl+Shift+Space)
  { key = 'Escape', mods = 'NONE', action = act.CopyMode 'Close' },
  { key = 'q', mods = 'NONE', action = act.CopyMode 'Close' },
  { key = 'c', mods = 'CTRL', action = act.CopyMode 'Close' },
  { key = 'g', mods = 'CTRL', action = act.CopyMode 'Close' },

  -- Kopieren und verlassen (y)
  { key = 'y', mods = 'NONE', action = act.Multiple { { CopyTo = 'ClipboardAndPrimarySelection' }, { CopyMode = 'Close' } } },

  -- Bewegung h/j/k/l + Pfeile
  { key = 'h', mods = 'NONE', action = act.CopyMode 'MoveLeft' },
  { key = 'j', mods = 'NONE', action = act.CopyMode 'MoveDown' },
  { key = 'k', mods = 'NONE', action = act.CopyMode 'MoveUp' },
  { key = 'l', mods = 'NONE', action = act.CopyMode 'MoveRight' },
  { key = 'LeftArrow', mods = 'NONE', action = act.CopyMode 'MoveLeft' },
  { key = 'DownArrow', mods = 'NONE', action = act.CopyMode 'MoveDown' },
  { key = 'UpArrow', mods = 'NONE', action = act.CopyMode 'MoveUp' },
  { key = 'RightArrow', mods = 'NONE', action = act.CopyMode 'MoveRight' },

  -- Zeilenanfang / -ende (0 ^ $ Home End)
  { key = '0', mods = 'NONE', action = act.CopyMode 'MoveToStartOfLine' },
  { key = 'Home', mods = 'NONE', action = act.CopyMode 'MoveToStartOfLine' },
  { key = '^', mods = 'NONE', action = act.CopyMode 'MoveToStartOfLineContent' },
  { key = '^', mods = 'SHIFT', action = act.CopyMode 'MoveToStartOfLineContent' },
  { key = '$', mods = 'NONE', action = act.CopyMode 'MoveToEndOfLineContent' },
  { key = '$', mods = 'SHIFT', action = act.CopyMode 'MoveToEndOfLineContent' },
  { key = 'End', mods = 'NONE', action = act.CopyMode 'MoveToEndOfLineContent' },

  -- Wortbewegung (w/b/e; keine semantische Variante)
  { key = 'w', mods = 'NONE', action = act.CopyMode 'MoveForwardWord' },
  { key = 'b', mods = 'NONE', action = act.CopyMode 'MoveBackwardWord' },
  { key = 'e', mods = 'NONE', action = act.CopyMode 'MoveForwardWordEnd' },
  { key = 'W', mods = 'SHIFT', action = act.CopyMode 'MoveForwardWord' },
  { key = 'B', mods = 'SHIFT', action = act.CopyMode 'MoveBackwardWord' },
  { key = 'E', mods = 'SHIFT', action = act.CopyMode 'MoveForwardWordEnd' },

  -- Viewport / Scrollback (Shift+H/M/L, g/G)
  { key = 'H', mods = 'SHIFT', action = act.CopyMode 'MoveToViewportTop' },
  { key = 'M', mods = 'SHIFT', action = act.CopyMode 'MoveToViewportMiddle' },
  { key = 'L', mods = 'SHIFT', action = act.CopyMode 'MoveToViewportBottom' },
  { key = 'g', mods = 'NONE', action = act.CopyMode 'MoveToScrollbackTop' },
  { key = 'G', mods = 'SHIFT', action = act.CopyMode 'MoveToScrollbackBottom' },

  -- Seiten / halbe Seiten (Ctrl+B/F, Ctrl+U/D, Ctrl+Y/E)
  { key = 'b', mods = 'CTRL', action = act.CopyMode 'PageUp' },
  { key = 'f', mods = 'CTRL', action = act.CopyMode 'PageDown' },
  { key = 'u', mods = 'CTRL', action = act.CopyMode { MoveByPage = -0.5 } },
  { key = 'd', mods = 'CTRL', action = act.CopyMode { MoveByPage = 0.5 } },
  { key = 'y', mods = 'CTRL', action = act.CopyMode 'MoveUp' },
  { key = 'e', mods = 'CTRL', action = act.CopyMode 'MoveDown' },
  { key = 'PageUp', mods = 'NONE', action = act.CopyMode 'PageUp' },
  { key = 'PageDown', mods = 'NONE', action = act.CopyMode 'PageDown' },

  -- Selektion (v, Shift+V, Ctrl+V, Alt+V)
  { key = 'v', mods = 'NONE', action = act.CopyMode { SetSelectionMode = 'Cell' } },
  { key = 'v', mods = 'SHIFT', action = act.CopyMode { SetSelectionMode = 'Line' } },
  { key = 'V', mods = 'SHIFT', action = act.CopyMode { SetSelectionMode = 'Line' } },
  { key = 'v', mods = 'CTRL', action = act.CopyMode { SetSelectionMode = 'Block' } },
  { key = 'v', mods = 'ALT', action = act.CopyMode { SetSelectionMode = 'SemanticZone' } },
  { key = 'Space', mods = 'NONE', action = act.CopyMode { SetSelectionMode = 'Cell' } },
  { key = 'o', mods = 'NONE', action = act.CopyMode 'MoveToSelectionOtherEnd' },
  { key = 'O', mods = 'SHIFT', action = act.CopyMode 'MoveToSelectionOtherEndHoriz' },

  -- Inline-Suche (f/F/t/T ; ,)
  { key = 'f', mods = 'NONE', action = act.CopyMode { JumpForward = { prev_char = false } } },
  { key = 'F', mods = 'SHIFT', action = act.CopyMode { JumpBackward = { prev_char = false } } },
  { key = 't', mods = 'NONE', action = act.CopyMode { JumpForward = { prev_char = true } } },
  { key = 'T', mods = 'SHIFT', action = act.CopyMode { JumpBackward = { prev_char = true } } },
  { key = ';', mods = 'NONE', action = act.CopyMode 'JumpAgain' },
  { key = ',', mods = 'NONE', action = act.CopyMode 'JumpReverse' },

  -- Suche (/ ? n N)
  { key = '/', mods = 'NONE', action = act.Search 'CurrentSelectionOrEmptyString' },
  { key = '?', mods = 'SHIFT', action = act.Search 'CurrentSelectionOrEmptyString' },

  -- Enter: springt an den Zeilenanfang (kein URL-Open).
  { key = 'Enter', mods = 'NONE', action = act.CopyMode 'MoveToStartOfNextLine' },
}

local search_mode = {
  { key = 'n', mods = 'NONE', action = act.CopyMode 'NextMatch' },
  { key = 'N', mods = 'SHIFT', action = act.CopyMode 'NextMatch' },
  { key = 'p', mods = 'CTRL', action = act.CopyMode 'PriorMatch' },
  { key = 'n', mods = 'CTRL', action = act.CopyMode 'NextMatch' },
  { key = 'Escape', mods = 'NONE', action = act.Multiple { { CopyMode = 'ClearPattern' }, { CopyMode = 'Close' } } },
}

return {
  -- Ligaturen: WezTerm shaped via harfbuzz (calt/liga/clig).
  font = wezterm.font 'CaskaydiaCove Nerd Font',
  font_size = 15.0,
  harfbuzz_features = { 'calt=1', 'liga=1', 'clig=1' },

  -- Optik: keine native Titelleiste (RESIZE behaelt den Resize-Rand; NONE
  -- verursacht Resize-/Minimize-Probleme), keine Tab-Leiste bei einem Tab.
  window_decorations = 'RESIZE',
  hide_tab_bar_if_only_one_tab = true,
  window_background_opacity = 0.90,
  colors = { background = '#000000' },

  -- Copy-Mode-Toggle: Ctrl+Shift+Space aktiviert den Copy-Mode.
  keys = {
    { key = 'Space', mods = 'CTRL|SHIFT', action = act.ActivateCopyMode },
  },

  key_tables = {
    copy_mode = copy_mode,
    search_mode = search_mode,
  },
}
