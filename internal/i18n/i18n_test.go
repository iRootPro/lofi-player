package i18n

import "testing"

func TestResolveSystemLocalePrecedence(t *testing.T) {
	t.Setenv("LANG", "en_US.UTF-8")
	t.Setenv("LC_MESSAGES", "ru_RU.UTF-8")
	t.Setenv("LC_ALL", "")
	if got := Resolve("auto"); got != RU {
		t.Fatalf("Resolve(auto) = %q, want ru", got)
	}
	t.Setenv("LC_ALL", "en_GB.UTF-8")
	if got := Resolve("auto"); got != EN {
		t.Fatalf("LC_ALL must win, got %q", got)
	}
}

func TestResolvePersistedChoiceAndFallback(t *testing.T) {
	t.Setenv("LANG", "ru_RU.UTF-8")
	for input, want := range map[string]Locale{"en": EN, "ru": RU, "unexpected": RU, "": RU} {
		if got := Resolve(input); got != want {
			t.Errorf("Resolve(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestRussianCatalogueHasAllEnglishKeys(t *testing.T) {
	for key := range catalog[EN] {
		if catalog[RU][key] == "" {
			t.Errorf("Russian translation missing key %q", key)
		}
	}
}
