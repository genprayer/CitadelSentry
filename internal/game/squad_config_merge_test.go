package game

import (
	"strings"
	"testing"
)

func TestMergePotatoIniPreservesUserChangesAndRevertsOwnedKeys(t *testing.T) {
	original := []byte("[/Script/Squad.SquadGameUserSettings]\r\nFrameRateLimit=144.000000\r\nCustomOption=Old\r\n")
	patched := patchPotatoIni(original, 1, false)
	current := append(patched, []byte("\r\n[PlayerSettings]\r\nCustomOption=New\r\n")...)
	merged, err := mergePotatoIni(original, patched, current)
	if err != nil {
		t.Fatal(err)
	}
	text := string(merged)
	if !strings.Contains(text, "FrameRateLimit=144.000000") || !strings.Contains(text, "[PlayerSettings]\r\nCustomOption=New") {
		t.Fatalf("merge lost settings: %s", text)
	}
	if strings.Contains(text, "PotatoMode=True") {
		t.Fatal("temporary Sentry marker remained after merge")
	}
}

func TestMergePotatoIniKeepsPlayerOverrideOfFps(t *testing.T) {
	original := []byte("[/Script/Squad.SquadGameUserSettings]\nFrameRateLimit=144.000000\n")
	patched := patchPotatoIni(original, 1, false)
	current := []byte(strings.Replace(string(patched), "FrameRateLimit=1.000000", "FrameRateLimit=90.000000", 1))
	merged, err := mergePotatoIni(original, patched, current)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(merged), "FrameRateLimit=90.000000") {
		t.Fatal("player FPS override was overwritten")
	}
}

func TestMergePotatoIniRestoresRecordedMissingKey(t *testing.T) {
	original := []byte("[/Script/Squad.SquadGameUserSettings]\nFrameRateLimit=144.000000\nCustomOption=Old\n")
	patched := patchPotatoIni(original, 1, false)
	current := []byte("[/Script/Squad.SquadGameUserSettings]\nCustomOption=New\n")
	merged, err := mergePotatoIni(original, patched, current)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(merged), "FrameRateLimit=144.000000") || !strings.Contains(string(merged), "CustomOption=New") {
		t.Fatalf("recorded original key was lost: %s", merged)
	}
}

func TestPotatoPatchDoesNotDuplicateKeysAcrossRepeatedSections(t *testing.T) {
	original := []byte("[/Script/Squad.SquadGameUserSettings]\nFrameRateLimit=144.000000\n[/Script/Squad.SquadGameUserSettings]\nCustomOption=Keep\n")
	patched := patchPotatoIni(original, 1, false)
	if strings.Count(strings.ToLower(string(patched)), "frameratelimit=") != 1 || !strings.Contains(string(patched), "CustomOption=Keep") {
		t.Fatalf("repeated sections caused an invented setting: %s", patched)
	}
}

func TestRestorationPreservesLiteralOriginalValue(t *testing.T) {
	original := []byte("[/Script/Squad.SquadGameUserSettings]\nFrameRateLimit=literal-$1\n")
	patched := []byte("[/Script/Squad.SquadGameUserSettings]\nFrameRateLimit=1.000000\n")
	current := append(append([]byte(nil), patched...), []byte("CustomOption=Keep\n")...)
	merged, err := mergePotatoIni(original, patched, current)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(merged), "FrameRateLimit=literal-$1") {
		t.Fatalf("saved value was interpreted as replacement syntax: %s", merged)
	}
}

func TestMergePotatoIniAcceptsUnrelatedRepeatedEngineKeys(t *testing.T) {
	original := []byte("[SystemSettings]\r\nt.MaxFPS=144.000000\r\n+Profiles=A\r\n+Profiles=B\r\n")
	patched := patchEngineIni(original, 1)
	current := append(patched, []byte("+Profiles=C\r\nNewSetting=Keep\r\n")...)
	merged, err := mergePotatoIni(original, patched, current)
	if err != nil {
		t.Fatalf("repeated unrelated keys blocked restore: %v", err)
	}
	for _, expected := range []string{"t.MaxFPS=144.000000", "+Profiles=A", "+Profiles=B", "+Profiles=C", "NewSetting=Keep"} {
		if !strings.Contains(string(merged), expected) {
			t.Fatalf("missing %s after merge: %s", expected, merged)
		}
	}
}
