package tui

import "github.com/iRootPro/lofi-player/internal/audio"

// mixerModel is the modal's transient UI state. The single source of
// truth for volumes is audio.AmbientMixer; mixerModel only knows which
// channel the user has selected with j/k.
type mixerModel struct {
	mixer    *audio.AmbientMixer
	selected int
}

func newMixerModel(am *audio.AmbientMixer) mixerModel {
	return mixerModel{mixer: am}
}

func (m mixerModel) Selected() string {
	if m.mixer == nil {
		return ""
	}
	ids := m.mixer.ChannelIDs()
	if m.selected < 0 || m.selected >= len(ids) {
		return ""
	}
	return ids[m.selected]
}

const (
	mixerStepFine   = 5
	mixerStepCoarse = 25
)

// handle applies a single key string to the mixer. Volume changes are
// pushed straight to the audio mixer; the model itself doesn't cache
// them.
func (m mixerModel) handle(key string) mixerModel {
	if m.mixer == nil {
		return m
	}
	ids := m.mixer.ChannelIDs()
	switch key {
	case "j", "down":
		if m.selected < len(ids)-1 {
			m.selected++
		}
	case "k", "up":
		if m.selected > 0 {
			m.selected--
		}
	case "l", "right":
		m.adjust(mixerStepFine)
	case "h", "left":
		m.adjust(-mixerStepFine)
	case "L":
		m.adjust(mixerStepCoarse)
	case "H":
		m.adjust(-mixerStepCoarse)
	case "0":
		m.set(0)
	case "1":
		m.set(100)
	}
	return m
}

func (m mixerModel) adjust(delta int) {
	id := m.Selected()
	if id == "" {
		return
	}
	_ = m.mixer.SetVolume(id, m.mixer.Volume(id)+delta)
}

func (m mixerModel) set(v int) {
	id := m.Selected()
	if id == "" {
		return
	}
	_ = m.mixer.SetVolume(id, v)
}
