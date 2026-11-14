package game

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSteamLibraryPaths(t *testing.T) {
	for _, item := range []struct {
		name string
		data string
		want []string
		bad  bool
	}{
		{"current", `"libraryfolders" { "0" { "path" "/media/Games Drive/SteamLibrary" "apps" { "393380" "123" } } }`, []string{"/media/Games Drive/SteamLibrary"}, false},
		{"legacy", `"LibraryFolders" { "TimeNextStatsReport" "1" "1" "/mnt/games" }`, []string{"/mnt/games"}, false},
		{"quoted-path", `"libraryfolders" { "0" { "path" "/mnt/a\"b" } }`, []string{`/mnt/a"b`}, false},
		{"unrelated-object", `"other" { "0" { "path" "/mnt/unrelated" } }`, []string{}, false},
		{"relative", `"libraryfolders" { "0" { "path" "relative/path" } }`, []string{}, false},
		{"nested-app-path", `"libraryfolders" { "0" { "apps" { "path" "/mnt/unrelated" } } }`, []string{}, false},
		{"truncated", `"libraryfolders" { "0" { "path" "/mnt/games" }`, nil, true},
		{"unterminated-string", `"libraryfolders" { "0" { "path" "/mnt/games } }`, nil, true},
		{"extra-close", `"libraryfolders" {} }`, nil, true},
		{"too-deep", strings.Repeat(`"x" {`, maxSteamLibraryDepth+1) + strings.Repeat("}", maxSteamLibraryDepth+1), nil, true},
	} {
		t.Run(item.name, func(t *testing.T) {
			paths, err := steamLibraryPaths([]byte(item.data))
			if (err != nil) != item.bad || !reflect.DeepEqual(paths, item.want) {
				t.Fatalf("got %v, %v; expected %v (bad=%t)", paths, err, item.want, item.bad)
			}
		})
	}
}

func TestSteamSquadDiscoveryAcrossLibrariesAndPackages(t *testing.T) {
	for _, layout := range []string{"standard", "external-library", "flatpak", "xdg-data", "legacy-library", "custom-prefix-user"} {
		t.Run(layout, func(t *testing.T) {
			home := t.TempDir()
			steam := filepath.Join(home, ".local", "share", "Steam")
			dataHome := ""
			if layout == "flatpak" {
				steam = filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", ".local", "share", "Steam")
			} else if layout == "xdg-data" {
				dataHome = filepath.Join(home, "custom-data")
				steam = filepath.Join(dataHome, "Steam")
			}
			library := steam
			if layout == "external-library" || layout == "legacy-library" {
				library = filepath.Join(t.TempDir(), "Games Disk", "SteamLibrary")
				data := fmt.Sprintf(`"libraryfolders" { "0" { "path" %q } }`, library)
				if layout == "legacy-library" {
					data = fmt.Sprintf(`"LibraryFolders" { "1" %q }`, library)
				}
				writeSteamFixture(t, filepath.Join(steam, "steamapps", "libraryfolders.vdf"), data)
				// An old prefix must not win over the registered installed game.
				writeSteamFixture(t, filepath.Join(steam, "steamapps", "compatdata", squadSteamAppID, "pfx", "drive_c", "users", "steamuser", "AppData", "Local", "SquadGame", "Saved", "Config", "WindowsClient", "GameUserSettings.ini"), "old")
			}
			writeSteamFixture(t, filepath.Join(library, "steamapps", "appmanifest_393380.acf"), `"AppState" { "appid" "393380" }`)
			user := "steamuser"
			if layout == "custom-prefix-user" {
				user = "player"
			}
			expected := filepath.Join(library, "steamapps", "compatdata", squadSteamAppID, "pfx", "drive_c", "users", user, "AppData", "Local", "SquadGame", "Saved", "Config", "Windows", "GameUserSettings.ini")
			writeSteamFixture(t, expected, "[/Script/Squad.SQGameUserSettings]\nFrameRateLimit=90\n")
			candidates := []string{}
			for _, base := range steamSquadConfigBases(home, dataHome) {
				for _, platform := range []string{"WindowsClient", "Windows", "WindowsNoEditor"} {
					candidates = append(candidates, filepath.Join(base, platform, "GameUserSettings.ini"))
				}
			}
			primary, existing := selectSquadConfigPaths(candidates)
			if primary != expected || len(existing) != 1 {
				t.Fatalf("wrong installed config: %s (%v), expected %s", primary, existing, expected)
			}
		})
	}
}

func TestSteamDiscoveryDeduplicatesSymlinkedLibraries(t *testing.T) {
	home := t.TempDir()
	steam := filepath.Join(home, ".local", "share", "Steam")
	writeSteamFixture(t, filepath.Join(steam, "steamapps", "libraryfolders.vdf"), fmt.Sprintf(`"libraryfolders" { "0" { "path" %q } }`, steam))
	link := filepath.Join(home, ".steam", "steam")
	if err := os.MkdirAll(filepath.Dir(link), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(steam, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	seen := map[string]bool{}
	for _, base := range steamSquadConfigBases(home, "") {
		if seen[base] {
			t.Fatalf("duplicate library base: %s", base)
		}
		seen[base] = true
	}
}

func writeSteamFixture(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestSquadStatusDiscoversConfigCreatedAfterSentryStartup(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Windows", "GameUserSettings.ini")
	manager, err := newConfigManager(filepath.Join(root, "backup"), func() (string, []string) {
		return selectSquadConfigPaths([]string{path})
	})
	if err != nil {
		t.Fatal(err)
	}
	if manager.GetStatus().ConfigFound {
		t.Fatal("config was invented before Squad created it")
	}
	writeSteamFixture(t, path, "[/Script/Squad.SQGameUserSettings]\nFrameRateLimit=90\n")
	status := manager.GetStatus()
	if !status.ConfigFound || status.ConfigPath != path || status.CurrentFpsCap != 90 || status.ErrorMessage != "" {
		t.Fatalf("new game config was not discovered by status refresh: %+v", status)
	}
}
