package audio

import "strings"

// metadataKeys lists the property names mpv may report a track title under,
// in priority order. Different ICY servers populate different keys, and
// some streams omit ICY entirely and only set media-title.
var metadataKeys = []string{
	"icy-title",
	"icy-name",
	"title",
	"media-title",
}

// ParseMetadata extracts the current track's title and artist from a map of
// ICY (or media-title-derived) metadata properties as reported by mpv.
//
// Most Icecast/Shoutcast streams pack both fields into a single string of
// the form "Artist - Title", so ParseMetadata splits on the first " - "
// occurrence. Streams that send only a title (no dash) yield Title=<the
// whole string>, Artist="". Empty/missing input yields two empty strings.
func ParseMetadata(m map[string]string) (title, artist string) {
	raw := pickFirst(m, metadataKeys)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}

	if a, t, ok := strings.Cut(raw, " - "); ok {
		return strings.TrimSpace(t), strings.TrimSpace(a)
	}
	return raw, ""
}

func pickFirst(m map[string]string, keys []string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok && strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
