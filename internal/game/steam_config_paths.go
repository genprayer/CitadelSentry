package game

import (
	"io"
	"os"
	"path/filepath"
)

const (
	squadSteamAppID      = "393380"
	maxSteamLibraryBytes = 1 << 20
	maxSteamLibraryDepth = 16
)

// steamSquadConfigBases follows registered Steam libraries without scanning disks.
// Arguments allow discovery tests to stay outside the real user's Steam prefix.
func steamSquadConfigBases(home, dataHome string) []string {
	roots := []string{}
	if filepath.IsAbs(dataHome) {
		roots = append(roots, filepath.Join(dataHome, "Steam"))
	}
	roots = append(roots, filepath.Join(home, ".local", "share", "Steam"),
		filepath.Join(home, ".steam", "steam"), filepath.Join(home, ".steam", "root"),
		filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", ".local", "share", "Steam"))
	libraries := []string{}
	scannedRoots := []string{}
	seenRoots, seenLibraries := map[string]bool{}, map[string]bool{}
	for _, root := range roots {
		if !appendSteamPath(&scannedRoots, seenRoots, root) {
			continue
		}
		root = scannedRoots[len(scannedRoots)-1]
		appendSteamPath(&libraries, seenLibraries, root)
		for _, folder := range []string{"steamapps", "config"} {
			for _, library := range readSteamLibraries(filepath.Join(root, folder, "libraryfolders.vdf")) {
				appendSteamPath(&libraries, seenLibraries, library)
			}
		}
	}
	// A previous installation's prefix must not take priority over an installed game.
	installed := []string{}
	for _, library := range libraries {
		if info, err := os.Stat(filepath.Join(library, "steamapps", "appmanifest_"+squadSteamAppID+".acf")); err == nil && info.Mode().IsRegular() {
			installed = append(installed, library)
		}
	}
	if len(installed) > 0 {
		libraries = installed
	}
	bases := []string{}
	seenBases := map[string]bool{}
	for _, library := range libraries {
		users := filepath.Join(library, "steamapps", "compatdata", squadSteamAppID, "pfx", "drive_c", "users")
		userNames := []string{"steamuser"}
		if entries, err := os.ReadDir(users); err == nil {
			for _, entry := range entries {
				if entry.IsDir() && entry.Name() != "steamuser" {
					userNames = append(userNames, entry.Name())
				}
			}
		}
		for _, user := range userNames {
			for _, game := range []string{"SquadGame", "Squad"} {
				appendSteamPath(&bases, seenBases, filepath.Join(users, user, "AppData", "Local", game, "Saved", "Config"))
			}
		}
	}
	return bases
}

func appendSteamPath(paths *[]string, seen map[string]bool, path string) bool {
	if !filepath.IsAbs(path) {
		return false
	}
	canonical := filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(canonical); err == nil {
		canonical = resolved
	}
	if seen[canonical] {
		return false
	}
	seen[canonical] = true
	*paths = append(*paths, canonical)
	return true
}

func readSteamLibraries(path string) []string {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxSteamLibraryBytes {
		return nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(file, maxSteamLibraryBytes+1))
	if err != nil || len(data) > maxSteamLibraryBytes {
		return nil
	}
	paths, err := steamLibraryPaths(data)
	if err != nil {
		return nil
	}
	return paths
}
