# Эмбиент-миксер v1 — дизайн

**Дата:** 2026-04-25
**Статус:** одобрено, готово к имплементации
**Связанные документы:** `plans/lofi-player-plan.md` §6 Phase 4c, §11 (открытые вопросы)

---

## 1. Краткая суть

Полноэкранная модалка, открывается клавишей `x` из основного экрана.
Внутри — три горизонтальных слайдера: 🌧️ Rain, 🔥 Fire, ⚪ White
Noise. Каждый — независимый канал, громкость 0–100. Loop-файлы зашиты
в бинарь через `embed.FS`, при первом запуске распаковываются в
`~/.cache/lofi-player/ambient/`. Состояние слайдеров хранится в
`state.json`, восстанавливается при следующем запуске.

Эмбиент играет **поверх** основной станции через системный аудио-микс
(каждый канал — отдельный mpv-инстанс). На главном экране активные
каналы отображаются компактным индикатором рядом со станцией:
`🎵 SomaFM Groove Salad · 🌧️🔥`.

## 2. Зафиксированные решения

| Вопрос | Решение |
|---|---|
| Где живут loop-файлы | `embed.FS` в бинарнике, распаковка в `~/.cache/lofi-player/ambient/` при старте |
| Скоуп v1 | 3 канала: rain, fire, white_noise |
| Форм-фактор UI | Полноэкранная модалка (как `addform`) |
| Пресеты | Один автосохранённый стейт. Никаких именованных |
| Layering | Каждый канал — независимый mpv. На 0 → пауза. `V/v` трогает только станцию |
| Master volume для ambient | Нет |
| Клавиша открытия миксера | `x` (mi**x**er). `a` остаётся за `AddStation` |
| Формат файлов | Opus в OGG, ~64 kbps стерео, 3–5 минут на канал |
| Лицензия файлов | CC0 предпочтительно, CC-BY с атрибуцией в README допустимо |
| Бюджет размера | ~10 МБ суммарно на 3 канала |

## 3. Архитектура

### 3.1 Новые сущности (`internal/audio/ambient.go`)

```go
type AmbientChannel struct {
    ID       string  // "rain" | "fire" | "white_noise"
    Label    string  // для UI ("rain", "fire", "white noise")
    Icon     string  // 🌧️ / 🔥 / ⚪
    filePath string  // путь к распакованному loop-файлу
    mpv      *mpv.Instance
    volume   int     // 0..100, источник истины
    disabled bool    // true если mpv не поднялся (graceful degradation)
}

type AmbientMixer struct {
    channels []*AmbientChannel
}

// API:
func (m *AmbientMixer) Init() error
func (m *AmbientMixer) SetVolume(id string, v int) error
func (m *AmbientMixer) Volumes() map[string]int
func (m *AmbientMixer) ActiveIDs() []string  // в фиксированном порядке
func (m *AmbientMixer) Close() error
```

### 3.2 embed → cache → mpv

mpv не читает Go FS напрямую. Решение — extract-on-first-run:

1. Loop-файлы в `internal/audio/ambient_assets/`:
   ```
   rain.opus
   fire.opus
   white_noise.opus
   README.md   # источник, автор, лицензия
   ```
2. Декларация: `//go:embed ambient_assets/*.opus` в `ambient.go`.
3. На `Mixer.Init()`:
   - Резолвим `~/.cache/lofi-player/ambient/` (через `os.UserCacheDir`).
   - Для каждого канала: если файла нет ИЛИ его SHA-256 не совпадает с
     зашитым (миграция между версиями) — пишем из `embed.FS` на диск
     через atomic temp+rename.
4. mpv проигрывает с диска как обычный локальный файл, в режиме
   `loop=inf`.

Альтернативу с локальным HTTP-сервером отбросили — overengineering.

### 3.3 Жизненный цикл

- **Старт приложения:** `AmbientMixer` создаёт 3 mpv-инстанса с
  громкостью 0 и в состоянии pause. После загрузки `state.json`
  выставляются ненулевые значения, соответствующие каналы анпаузятся.
- **Изменение громкости:** `SetVolume(id, v)`:
  - `v == 0` → пауза + volume = 0.
  - `v > 0` → unpause + volume = v.
- **Закрытие приложения:** `Close()` параллельно прибивает все
  mpv-инстансы. Финальный flush стейта.

### 3.4 Graceful degradation

Если конкретный mpv не запустился (битый кэш-файл, некорректный
формат после ручной правки и т.п.) — канал помечается `disabled`.
В UI слайдер рендерится серым с пометкой `unavailable`. Остальные
каналы продолжают работать. Не падаем.

## 4. UI

### 4.1 Layout модалки

```
╭──────────────── ambient mixer ────────────────╮
│                                                │
│   🌧️  rain          ▰▰▰▰▱▱▱▱▱▱   40           │
│                                                │
│ > 🔥  fire          ▰▱▱▱▱▱▱▱▱▱   10  <         │
│                                                │
│   ⚪  white noise   ▱▱▱▱▱▱▱▱▱▱    0           │
│                                                │
╰────────── j/k select · h/l ±5 · 0 mute · x close ──╯
```

- Активный канал: подсветка строки в Primary цвете + стрелочки `> <`.
- Заглушенные каналы (volume == 0): Subtle цвет.
- Disabled каналы: `unavailable` вместо bar'а в Subtle.
- Hint-bar в нижней рамке (как в основном view через `frame.go`).

### 4.2 Keybindings внутри модалки

| Клавиша | Действие |
|---|---|
| `j` / `↓` | следующий канал |
| `k` / `↑` | предыдущий канал |
| `l` / `→` | громкость +5 (clamp 100) |
| `h` / `←` | громкость −5 (clamp 0) |
| `L` | громкость +25 |
| `H` | громкость −25 |
| `0` | активный канал → 0 |
| `1` | активный канал → 100 |
| `x` / `esc` | закрыть модалку |
| `q` / `ctrl+c` | выход из приложения |
| `?` | help |

### 4.3 Глобальные клавиши, отключённые в модалке

`space`, `+`, `-`, `t`, `m`, `a` — не реагируют пока миксер открыт.
Это совпадает с поведением `addform` и предотвращает случайные
нажатия (например, `+` думая что это громкость канала).

### 4.4 Индикатор активного ambient на главном экране

Рядом со станцией в статус-баре: `🎵 SomaFM Groove Salad · 🌧️🔥`.
- Только иконки активных каналов (volume > 0), в фиксированном
  порядке: rain, fire, white_noise.
- Цвет — Primary (как `?` сейчас).
- Если все каналы на 0 — индикатор полностью пропадает (`·` с
  иконками не рендерится).
- Без процентов — только иконки.

## 5. Loop-файлы

| Параметр | Значение |
|---|---|
| Формат | Opus в OGG (`.opus`) |
| Битрейт | ~64 kbps стерео |
| Длина | 3–5 минут на канал |
| Размер | ~2.5–3 МБ на канал, ~10 МБ суммарно |
| Источник | freesound.org (предпочтительно CC0) |
| Атрибуция | `internal/audio/ambient_assets/README.md` + корневой `ATTRIBUTIONS.md` если CC-BY |
| Подбор и редактирование | Отдельная задача параллельно с кодом — обрезка, нормализация, fade-in/out для бесшовного loop'а через ffmpeg или Audacity |

На время разработки кода — файлы-плейсхолдеры (любой `.opus`).
Финальные файлы коммитятся отдельным коммитом перед релизом v0.4.0.

## 6. State и persistence

### 6.1 Расширение `internal/state/state.go`

```go
type State struct {
    Theme            string         `json:"theme,omitempty"`
    Volume           int            `json:"volume,omitempty"`
    LastStationName  string         `json:"last_station_name,omitempty"`
    Ambient          map[string]int `json:"ambient,omitempty"`
}
```

`Ambient` — карта `channel_id → volume`. Храним все три канала
явно (для отладки), даже если значение 0.

### 6.2 Forward/backward compat

- Старый `state.json` без `Ambient` → загружается, миксер на нулях.
- Новый `state.json` с дополнительными каналами → текущая версия
  игнорирует неизвестные ключи.
- Удалённый канал → лишний ключ просто не используется.

Работает «само» благодаря `json.Unmarshal` + `omitempty`. Миграции
писать не нужно.

### 6.3 Когда сохраняем

**Дебаунс 500 мс** после последнего движения слайдера. Каждое
нажатие `h`/`l` пишет в память; `state.Save()` вызывается через
500 мс тишины. Иначе при удержании клавиши получим 20 атомарных
rename-ов на каждый sweep 0→100. Реализуем через `tea.Tick` или
канал (паттерн с toast уже есть в `internal/tui/toast.go`).

При корректном выходе (`q`, `ctrl+c`) — финальный синхронный flush.
При `kill -9` теряем максимум 500 мс изменений — приемлемо.

### 6.4 Восстановление при старте

После распаковки embed-файлов `Mixer.Init()` смотрит в `state.Ambient`,
выставляет громкости, анпаузит каналы с `vol > 0`. Параллельно
основной плеер поднимает станцию — две независимые очереди mpv,
race-conditions невозможны.

## 7. Тестирование

### 7.1 `internal/audio/ambient_test.go` (unit)

- `TestMixerExtractsEmbedToCache` — на пустом tmp `Init()` создаёт
  3 файла с правильным размером и SHA-256.
- `TestMixerSkipsExtractIfHashMatches` — повторный `Init()` не пишет
  если кэш свежий.
- `TestMixerOverwritesIfHashMismatch` — битый кэш-файл
  перезаписывается.
- `TestSetVolumePausesAtZero` / `TestSetVolumeUnpausesAboveZero` —
  против мока mpv-инстанса (вводим интерфейс если нет).
- `TestActiveIDsOrder` — фиксированный порядок `rain, fire,
  white_noise`, только > 0.
- `TestDisabledChannelOnInitFailure` — битый mpv → канал disabled,
  остальные работают.
- `TestVolumesSnapshot` — карта совпадает с реально выставленным.
- `TestCloseTerminatesAllChannels` — `Close()` шлёт Quit во все mpv.

### 7.2 `internal/tui/ambient_test.go` (UI)

- `TestOpenMixerByX` — `x` → `viewAmbient`.
- `TestNavigateChannels` — `j`/`k` двигают selectedChannel с clamp.
- `TestVolumeAdjust` — `h`/`l` ±5, `H`/`L` ±25, clamp 0..100.
- `TestZeroAndOneShortcuts` — `0` → 0, `1` → 100.
- `TestCloseModalKeepsState` — `esc`/`x` возвращает в основной view,
  громкости сохраняются.
- `TestGlobalKeysDisabledInModal` — `space`, `+`, `t`, `m`, `a` не
  реагируют пока миксер открыт.
- `TestAmbientIndicatorRenders` — основной view показывает иконки
  только активных каналов; нет активных → индикатор пуст.

### 7.3 `internal/state/state_test.go` (расширение)

- `TestLoadStateWithoutAmbient` — старый файл грузится, `Ambient` нул.
- `TestRoundtripWithAmbient` — Save+Load с тремя каналами даёт ту же
  карту.
- `TestUnknownAmbientKeyIgnored` — стейт с `cafe: 50` грузится, лишний
  ключ игнорируется при использовании.

### 7.4 Интеграция (опционально, как `player_test.go`)

- `TestMixerPlaysRealLoopFile` — реальный mpv, реальный файл, через
  200 мс читаем volume и pause через mpv get_property. `t.Skip` если
  mpv не на PATH.

### 7.5 Чего НЕ тестируем

- Качество звука / отсутствие щелчка на стыке loop'а — ответственность
  подбора файла.
- Микширование на уровне аудио-стека — работа OS.
- Точность дебаунса 500 мс — flaky на CI; ручная проверка.

## 8. Что явно НЕ делаем в v1

- Именованные пресеты, save-as, переключение `1/2/3` между ними.
- Master volume для всего ambient слоя.
- Конфиг `ambient.custom_path` для своих файлов (разумный follow-up,
  не сейчас).
- Дополнительные каналы (cafe, wind, storm, forest и т.д.). Архитектура
  расширяется одной строкой в массиве каналов + одним файлом в
  `embed.FS`, но v1 — три.
- Кросс-фейды между loop-итерациями. Это ответственность подбора файла,
  не кода.
- Эквалайзер / фильтры / эффекты.

## 9. Критерий готовности v1

Юзер запускает приложение, нажимает `x`, видит три слайдера, крутит
«Дождь» до 40 — слышит дождь поверх лофи. На главном экране рядом
со станцией появляется иконка 🌧️. Закрывает приложение, открывает
снова — дождь снова на 40, играет автоматически, индикатор тот же.
