// Package i18n provides the small, dependency-free translation catalogue used
// by lofi-player. It deliberately localizes only text authored by the player;
// station names and stream metadata remain exactly as supplied by their source.
package i18n

import (
	"fmt"
	"os"
	"strings"
)

type Locale string

const (
	Auto Locale = "auto"
	EN   Locale = "en"
	RU   Locale = "ru"
)

// Resolve converts a persisted choice into an effective interface locale.
func Resolve(choice string) Locale {
	if Normalize(choice) == RU {
		return RU
	}
	if Normalize(choice) == EN {
		return EN
	}
	return System()
}

// Normalize accepts only the persisted language choices. Unknown and legacy
// values safely fall back to automatic detection.
func Normalize(choice string) Locale {
	switch strings.ToLower(strings.TrimSpace(choice)) {
	case "en", "english":
		return EN
	case "ru", "russian", "русский":
		return RU
	default:
		return Auto
	}
}

// System detects the conventional locale environment variables without
// mutating process state. LC_ALL takes precedence over LC_MESSAGES and LANG.
func System() Locale {
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		value := strings.ToLower(os.Getenv(name))
		if value == "" || value == "c" || value == "posix" {
			continue
		}
		value = strings.SplitN(value, ".", 2)[0]
		value = strings.SplitN(value, "@", 2)[0]
		if strings.HasPrefix(value, "ru") {
			return RU
		}
		return EN
	}
	return EN
}

// T returns a translated string. English is the complete fallback so a missing
// Russian entry can never expose an internal key in the UI.
func T(locale Locale, key string, args ...any) string {
	message, ok := catalog[locale][key]
	if !ok {
		message = catalog[EN][key]
	}
	if message == "" {
		message = key
	}
	if len(args) == 0 {
		return message
	}
	return fmt.Sprintf(message, args...)
}

// ChoiceLabel is intentionally kept here because the Settings value is a
// user-facing localized label rather than a persisted identifier.
func ChoiceLabel(locale Locale, choice string) string {
	switch Normalize(choice) {
	case EN:
		return T(locale, "language_english")
	case RU:
		return T(locale, "language_russian")
	default:
		return T(locale, "language_system")
	}
}

var catalog = map[Locale]map[string]string{
	EN: {
		"language_system": "System", "language_english": "English", "language_russian": "Русский",
		"settings": "settings", "language": "language", "network_buffer": "network buffer", "initial_buffer": "initial buffer",
		"mpv_default": "mpv default", "off": "off", "audio_ahead": "audio kept ahead for unstable connections",
		"initial_buffer_desc": "wait before start/resume, capped at 10s for live radio", "effective_default": "effective read-ahead: mpv default", "effective_read_ahead": "effective read-ahead: %ds",
		"select": "select", "change": "change", "change_language": "change language", "save": "save", "save_preview": "save preview", "cancel": "cancel", "close": "close", "help": "help", "settings_saved": "settings saved — restart to apply buffer changes", "error_label": "error: ", "success_label": "ok: ", "note_label": "note: ", "update_check_failed": "Could not check for updates.", "usage": "Usage",
		"language_saved": "language saved", "save_settings_failed": "Could not save settings. Check your config directory permissions.",
		"play_pause": "play/pause", "station": "station", "discover": "discover", "volume": "volume", "share_import": "share/import", "save_paste": "save/paste", "mixer": "mixer", "themes": "themes",
		"loading_station":  "Loading %s…",
		"playing_station":  "Playing %s",
		"playing":          "playing",
		"paused":           "paused",
		"update_available": "update %s available", "no_station": "— no station selected —", "loading": "loading…", "unavailable": "unavailable", "press_a": "press a", "to_add_one": "to add one", "more_up": "↑ %d more", "more_down": "↓ %d more",
		"navigation": "navigation", "playback": "playback", "app": "app", "not_saved": "not saved",
		"add_station": "─── add station ───", "edit_station": "─── edit station ───", "name": "name  ", "url": "url   ", "name_required": "name is required", "url_required": "url is required", "next_field": "next field",
		"delete": "delete ", "confirm": "confirm", "share_station": "─── share station ───", "copy": "copy", "nothing_to_share": "nothing to share",
		"import_stations": "─── import stations ───", "import_count": "import %d station(s)?", "and_more": "…and %d more", "skipping_duplicates": "skipping %d duplicate(s)", "import": "import",
		"theme_picker": "─── themes ───", "preview": "preview", "ambient_mixer": "ambient mixer", "mute": "mute",
		"discover_radio": "─── discover radio ───", "search": "search  ", "searching": "searching Radio Browser…", "error": "error: ", "cached_results": "cached results — Radio Browser is unavailable", "type_two": "type at least 2 characters; results appear automatically", "no_stations": "no stations found", "more_results": "%d more results", "results": "results", "input": "input", "play": "play", "details": "details",
		"selected": "selected  ", "tags": "tags  ", "press_v": "press v for full details", "station_details": "─── station details ───", "lines_of": "%d–%d of %d lines", "back": "back", "scroll": "scroll",
		"history": "─── listening history ───", "today": "today", "seven_days": "7 days", "all_time": "all time", "sessions": "%d sessions", "now_playing": "now playing  ", "top_stations": "top stations", "recent": "recent", "history_empty": "History appears after 30 seconds of confirmed playback.", "period": "period", "clear": "clear", "clear_history": "─── clear listening history? ───", "clear_history_body": "This permanently removes local completed sessions.", "clear_history_hint": "Current playback continues as a fresh session.",
		"doctor": "─── playback doctor ───", "doctor_report": "playback doctor", "checking": "Checking mpv, config, DNS, and Radio Browser…", "copy_report": "copy safe report", "copy_host": "copy stream host", "run_checks": "run checks", "application": "application", "player": "player", "stream_host": "stream host", "discovery_cache": "discovery/cache", "last_issue": "last issue",
		"update_title": "─── update available ───", "update_body": "lofi-player %s is available.", "other": "other", "release_notes": "see the release notes:",
		"reconnecting": "reconnecting stream…", "save_mixer_failed": "Could not save ambient mixer settings.", "clipboard_unavailable": "Clipboard is unavailable.", "copy_manual": "Clipboard is unavailable. Copy the station text manually.", "snippet_copied": "station snippet copied", "report_copied": "safe doctor report copied", "host_copied": "stream hostname copied", "no_stream_host": "No active stream hostname to copy.", "history_save_failed": "Could not save listening history.", "history_cleared": "listening history cleared", "play_failed": "Could not play this station. Try another one.", "stream_ended": "The stream ended.", "stream_stalled": "stream stalled — reconnecting if it does not recover", "command_failed": "Could not %s. Try again.", "pause": "pause", "resume": "resume", "set_volume": "set volume", "rain": "rain", "fire": "fire", "white_noise": "white noise", "cafe": "cafe", "thunder": "thunder",
		"already_library": "Already in your library: %s", "save_station_failed": "Could not save this station. Check your config directory permissions.", "saved_library": "Saved %s to your library.", "changes_save_failed": "Could not save your station changes. Check your config directory permissions.", "added": "added: %s", "updated": "updated: %s", "removed_save_failed": "Could not save the station removal. Check your config directory permissions.", "deleted": "deleted: %s", "youtube_missing": "yt-dlp not installed — install it to play YouTube stations", "import_failed": "Could not import stations. Check the copied station data.", "already_exist": "all pasted stations already exist", "import_save_failed": "Could not save imported stations. Check your config directory permissions.", "imported": "imported %d station(s)", "share_prepare_failed": "Could not prepare this station for sharing.",
		"rb_slow": "Radio Browser is taking too long. Try again.", "rb_limited": "Radio Browser is limiting requests. Try again shortly.", "rb_temporary": "Radio Browser is temporarily unavailable. Try again later.", "rb_search_failed": "Radio Browser could not complete this search. Try again.", "rb_invalid": "Radio Browser sent an invalid response. Try again.", "rb_connection": "Can't reach Radio Browser. Check your connection.", "rb_unavailable": "Radio Browser is unavailable. Try again later.",
		"usage_doctor": "usage: lofi-player doctor", "flag_statusline": "print one status-line snapshot to stdout and exit (no TUI)", "flag_version": "print version and exit", "flag_version_short": "print version and exit (shorthand)", "flag_update": "check GitHub Releases for a newer version and exit", "flag_export_all": "print all stations as a shareable YAML snippet and exit", "flag_export_station": "print one station by name as a shareable YAML snippet and exit", "flag_import": "import stations from a YAML snippet file, or '-' for stdin, and exit", "non_release": "lofi-player non-release build; see https://github.com/iRootPro/lofi-player/releases", "up_to_date": "lofi-player %s is up to date", "available_cli": "lofi-player %s is available: %s", "station_not_found": "station %q not found", "all_exist_cli": "lofi-player: all %d station(s) already exist\n", "imported_cli": "lofi-player: imported %d station(s), skipped %d duplicate(s)\n", "mpv_role": "audio engine, required for all playback", "cannot_start": "can't start", "install_again": "Install and run lofi-player again.",
	},
	RU: {
		"language_system": "Системный", "language_english": "English", "language_russian": "Русский",
		"settings": "настройки", "language": "язык", "network_buffer": "сетевой буфер", "initial_buffer": "начальный буфер", "mpv_default": "по умолчанию mpv", "off": "выкл.", "audio_ahead": "запас аудио для нестабильного соединения", "initial_buffer_desc": "ожидание перед стартом/продолжением, максимум 10 с для радио", "effective_default": "эффективный запас: по умолчанию mpv", "effective_read_ahead": "эффективный запас: %d с", "select": "выбрать", "change": "изменить", "change_language": "сменить язык", "save": "сохранить", "save_preview": "сохранить предпросмотр", "cancel": "отмена", "close": "закрыть", "help": "помощь", "settings_saved": "настройки сохранены — перезапустите для применения буфера", "language_saved": "язык сохранён", "save_settings_failed": "Не удалось сохранить настройки. Проверьте права на каталог конфигурации.", "error_label": "ошибка: ", "success_label": "готово: ", "note_label": "заметка: ", "update_check_failed": "Не удалось проверить обновления.", "usage": "Использование",
		"play_pause": "пуск/пауза", "station": "станция", "discover": "поиск", "volume": "громкость", "share_import": "поделиться/импорт", "save_paste": "сохранить/вставить", "mixer": "микшер", "themes": "темы", "update_available": "доступно обновление %s", "no_station": "— станция не выбрана —", "loading": "загрузка…", "unavailable": "недоступно", "press_a": "нажмите a", "to_add_one": "чтобы добавить станцию", "more_up": "↑ ещё %d", "more_down": "↓ ещё %d", "navigation": "навигация", "playback": "воспроизведение", "app": "приложение", "not_saved": "не сохранено",
		"loading_station": "Загрузка %s…",
		"playing_station": "Играет %s",
		"playing":         "играет",
		"paused":          "на паузе",
		"add_station":     "─── добавить станцию ───", "edit_station": "─── изменить станцию ───", "name": "имя  ", "url": "URL   ", "name_required": "нужно указать имя", "url_required": "нужно указать URL", "next_field": "следующее поле", "delete": "удалить ", "confirm": "подтвердить", "share_station": "─── поделиться станцией ───", "copy": "копировать", "nothing_to_share": "нечем поделиться", "import_stations": "─── импорт станций ───", "import_count": "импортировать станций: %d?", "and_more": "…и ещё %d", "skipping_duplicates": "пропущено дубликатов: %d", "import": "импорт", "theme_picker": "─── темы ───", "preview": "предпросмотр", "ambient_mixer": "эмбиент-микшер", "mute": "выкл. звук",
		"discover_radio": "─── поиск радио ───", "search": "поиск  ", "searching": "Поиск в Radio Browser…", "error": "ошибка: ", "cached_results": "результаты из кеша — Radio Browser недоступен", "type_two": "введите минимум 2 символа; результаты появятся автоматически", "no_stations": "станции не найдены", "more_results": "ещё результатов: %d", "results": "результаты", "input": "ввод", "play": "слушать", "details": "подробности", "selected": "выбрано  ", "tags": "теги  ", "press_v": "нажмите v для подробностей", "station_details": "─── сведения о станции ───", "lines_of": "%d–%d из %d строк", "back": "назад", "scroll": "прокрутка",
		"history": "─── история прослушивания ───", "today": "сегодня", "seven_days": "7 дней", "all_time": "всё время", "sessions": "сессий: %d", "now_playing": "сейчас играет  ", "top_stations": "популярные станции", "recent": "недавние", "history_empty": "История появится после 30 секунд подтверждённого воспроизведения.", "period": "период", "clear": "очистить", "clear_history": "─── очистить историю? ───", "clear_history_body": "Это навсегда удалит завершённые локальные сессии.", "clear_history_hint": "Текущее воспроизведение продолжится как новая сессия.",
		"doctor": "─── диагностика воспроизведения ───", "doctor_report": "диагностика воспроизведения", "checking": "Проверяются mpv, конфигурация, DNS и Radio Browser…", "copy_report": "копировать безопасный отчёт", "copy_host": "копировать хост потока", "run_checks": "запустить проверки", "application": "приложение", "player": "плеер", "stream_host": "хост потока", "discovery_cache": "поиск/кеш", "last_issue": "последняя проблема", "update_title": "─── доступно обновление ───", "update_body": "Доступен lofi-player %s.", "other": "другое", "release_notes": "смотрите заметки к релизу:",
		"reconnecting": "переподключение к потоку…", "save_mixer_failed": "Не удалось сохранить настройки эмбиент-микшера.", "clipboard_unavailable": "Буфер обмена недоступен.", "copy_manual": "Буфер обмена недоступен. Скопируйте текст станции вручную.", "snippet_copied": "сниппет станции скопирован", "report_copied": "безопасный отчёт скопирован", "host_copied": "имя хоста потока скопировано", "no_stream_host": "Нет активного хоста потока для копирования.", "history_save_failed": "Не удалось сохранить историю прослушивания.", "history_cleared": "история прослушивания очищена", "play_failed": "Не удалось воспроизвести эту станцию. Попробуйте другую.", "stream_ended": "Поток завершился.", "stream_stalled": "поток остановился — будет переподключение, если он не восстановится", "command_failed": "Не удалось %s. Попробуйте ещё раз.", "pause": "поставить на паузу", "resume": "продолжить", "set_volume": "изменить громкость", "rain": "дождь", "fire": "огонь", "white_noise": "белый шум", "cafe": "кафе", "thunder": "гроза", "already_library": "Уже в библиотеке: %s", "save_station_failed": "Не удалось сохранить станцию. Проверьте права на каталог конфигурации.", "saved_library": "%s сохранена в библиотеку.", "changes_save_failed": "Не удалось сохранить изменения станции. Проверьте права на каталог конфигурации.", "added": "добавлено: %s", "updated": "обновлено: %s", "removed_save_failed": "Не удалось сохранить удаление станции. Проверьте права на каталог конфигурации.", "deleted": "удалено: %s", "youtube_missing": "yt-dlp не установлен — установите его для станций YouTube", "import_failed": "Не удалось импортировать станции. Проверьте скопированные данные.", "already_exist": "все вставленные станции уже существуют", "import_save_failed": "Не удалось сохранить импортированные станции. Проверьте права на каталог конфигурации.", "imported": "импортировано станций: %d", "share_prepare_failed": "Не удалось подготовить станцию для отправки.",
		"rb_slow": "Radio Browser отвечает слишком долго. Попробуйте ещё раз.", "rb_limited": "Radio Browser ограничивает запросы. Попробуйте чуть позже.", "rb_temporary": "Radio Browser временно недоступен. Попробуйте позже.", "rb_search_failed": "Radio Browser не смог выполнить поиск. Попробуйте ещё раз.", "rb_invalid": "Radio Browser вернул некорректный ответ. Попробуйте ещё раз.", "rb_connection": "Не удаётся подключиться к Radio Browser. Проверьте соединение.", "rb_unavailable": "Radio Browser недоступен. Попробуйте позже.",
		"usage_doctor": "использование: lofi-player doctor", "flag_statusline": "вывести строку статуса и завершиться (без TUI)", "flag_version": "вывести версию и завершиться", "flag_version_short": "вывести версию и завершиться (сокращение)", "flag_update": "проверить GitHub Releases на обновление и завершиться", "flag_export_all": "вывести все станции как YAML-сниппет и завершиться", "flag_export_station": "вывести станцию по имени как YAML-сниппет и завершиться", "flag_import": "импортировать станции из YAML-файла или '-' для stdin и завершиться", "non_release": "сборка lofi-player не является релизной; см. https://github.com/iRootPro/lofi-player/releases", "up_to_date": "lofi-player %s актуален", "available_cli": "доступен lofi-player %s: %s", "station_not_found": "станция %q не найдена", "all_exist_cli": "lofi-player: все %d станций уже существуют\n", "imported_cli": "lofi-player: импортировано %d станций, пропущено дубликатов: %d\n", "mpv_role": "аудиодвижок, необходимый для воспроизведения", "cannot_start": "не удалось запустить", "install_again": "Установите зависимость и снова запустите lofi-player.",
	},
}
