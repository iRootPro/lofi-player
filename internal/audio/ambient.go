package audio

// AmbientChannel is the static metadata for one ambient channel.
type AmbientChannel struct {
	ID    string
	Label string
	Icon  string
	asset string
}

var ambientChannels = []AmbientChannel{
	{ID: "rain", Label: "rain", Icon: "🌧️", asset: "rain.opus"},
	{ID: "fire", Label: "fire", Icon: "🔥", asset: "fire.opus"},
	{ID: "white_noise", Label: "white noise", Icon: "⚪", asset: "white_noise.opus"},
}

type AmbientMixer struct {
	channels []AmbientChannel
}

func NewAmbientMixer() *AmbientMixer {
	return &AmbientMixer{channels: append([]AmbientChannel(nil), ambientChannels...)}
}

func (m *AmbientMixer) ChannelIDs() []string {
	ids := make([]string, len(m.channels))
	for i, c := range m.channels {
		ids[i] = c.ID
	}
	return ids
}

func (m *AmbientMixer) Channel(id string) (AmbientChannel, bool) {
	for _, c := range m.channels {
		if c.ID == id {
			return c, true
		}
	}
	return AmbientChannel{}, false
}
