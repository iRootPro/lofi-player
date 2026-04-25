package audio

import "testing"

func TestNewAmbientMixerHasThreeChannelsInFixedOrder(t *testing.T) {
	m := NewAmbientMixer()
	got := m.ChannelIDs()
	want := []string{"rain", "fire", "white_noise"}
	if len(got) != len(want) {
		t.Fatalf("ChannelIDs: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ChannelIDs[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestAmbientChannelMetadata(t *testing.T) {
	m := NewAmbientMixer()
	cases := []struct {
		id    string
		label string
	}{
		{"rain", "rain"},
		{"fire", "fire"},
		{"white_noise", "white noise"},
	}
	for _, c := range cases {
		ch, ok := m.Channel(c.id)
		if !ok {
			t.Fatalf("Channel(%q): not found", c.id)
		}
		if ch.Label != c.label {
			t.Errorf("Channel(%q).Label: got %q, want %q", c.id, ch.Label, c.label)
		}
		if ch.Icon == "" {
			t.Errorf("Channel(%q).Icon: empty", c.id)
		}
	}
}
