package locale

import "testing"

func TestToPlay(t *testing.T) {
	for in, want := range map[string]string{
		"it":      "it-IT",
		"es-MX":   "es-419",
		"zh-Hans": "zh-CN",
		"de-DE":   "de-DE",
		"it-IT":   "it-IT",
	} {
		if got := ToPlay(in); got != want {
			t.Errorf("ToPlay(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFromPlayInverse(t *testing.T) {
	for appStore, play := range toPlay {
		if got := ToPlay(appStore); got != play {
			t.Errorf("ToPlay(%q) = %q, want %q", appStore, got, play)
		}
		if got := FromPlay(play); got != appStore {
			t.Errorf("FromPlay(%q) = %q, want %q", play, got, appStore)
		}
	}
	for _, same := range []string{"de-DE", "en-US", "fr-FR", "pt-BR"} {
		if got := FromPlay(same); got != same {
			t.Errorf("FromPlay(%q) = %q", same, got)
		}
	}
}
