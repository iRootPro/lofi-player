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
