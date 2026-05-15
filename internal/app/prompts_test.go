package app

import (
	"testing"

	"fyne.io/fyne/v2"
)

func TestParseHotkey(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantKey fyne.KeyName
		wantMod fyne.KeyModifier
		wantErr bool
	}{
		{"cmd shift letter", "Cmd+Shift+P", "P", fyne.KeyModifierShortcutDefault | fyne.KeyModifierShift, false},
		{"ctrl alt digit", "Ctrl+Alt+3", "3", fyne.KeyModifierControl | fyne.KeyModifierAlt, false},
		{"meta alias", "Meta+A", "A", fyne.KeyModifierShortcutDefault, false},
		{"lowercase", "cmd+b", "B", fyne.KeyModifierShortcutDefault, false},
		{"no modifier", "P", "", 0, true},
		{"unknown modifier", "Fn+P", "", 0, true},
		{"multi-letter key", "Cmd+Tab", "", 0, true},
		{"empty", "", "", 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotKey, gotMod, err := parseHotkey(c.in)
			if (err != nil) != c.wantErr {
				t.Fatalf("parseHotkey(%q) err=%v wantErr=%v", c.in, err, c.wantErr)
			}
			if c.wantErr {
				return
			}
			if gotKey != c.wantKey {
				t.Errorf("key got %q want %q", gotKey, c.wantKey)
			}
			if gotMod != c.wantMod {
				t.Errorf("mod got %d want %d", gotMod, c.wantMod)
			}
		})
	}
}
