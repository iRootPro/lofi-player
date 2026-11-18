package tui

import "github.com/iRootPro/lofi-player/internal/i18n"

func (m Model) t(key string, args ...any) string { return i18n.T(m.locale, key, args...) }

func (m Model) effectiveLanguage() i18n.Locale { return i18n.Resolve(m.cfg.Language) }
