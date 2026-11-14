package game

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"text/scanner"
)

// steamLibraryPaths accepts current and legacy libraryfolders.vdf structures.
// Malformed input never supplies partial paths to the config writer.
func steamLibraryPaths(data []byte) ([]string, error) {
	if len(data) > maxSteamLibraryBytes {
		return nil, fmt.Errorf("Steam library metadata exceeds its size limit")
	}
	var input scanner.Scanner
	input.Init(strings.NewReader(string(data)))
	input.Mode = scanner.ScanStrings | scanner.ScanIdents | scanner.ScanInts | scanner.ScanComments | scanner.SkipComments
	var scanErr error
	input.Error = func(_ *scanner.Scanner, message string) {
		scanErr = fmt.Errorf("invalid Steam library metadata: %s", message)
	}
	read := func(token rune) (string, error) {
		switch token {
		case scanner.String:
			return strconv.Unquote(input.TokenText())
		case scanner.Ident, scanner.Int:
			return input.TokenText(), nil
		default:
			return "", fmt.Errorf("unexpected token in Steam library metadata")
		}
	}
	paths := []string{}
	var object func([]string) error
	object = func(parents []string) error {
		if len(parents) > maxSteamLibraryDepth {
			return fmt.Errorf("Steam library metadata nesting exceeds its limit")
		}
		for {
			token := input.Scan()
			if token == scanner.EOF {
				if len(parents) == 0 {
					return scanErr
				}
				return fmt.Errorf("unterminated Steam library object")
			}
			if token == '}' {
				if len(parents) == 0 {
					return fmt.Errorf("unexpected closing Steam library object")
				}
				return scanErr
			}
			key, err := read(token)
			if err != nil {
				return err
			}
			token = input.Scan()
			if token == '{' {
				if err := object(append(parents, key)); err != nil {
					return err
				}
				continue
			}
			value, err := read(token)
			if err != nil {
				return err
			}
			isAbs := filepath.IsAbs(value) || strings.HasPrefix(value, "/") || strings.HasPrefix(value, "\\")
			if len(parents) == 0 || !strings.EqualFold(parents[0], "libraryfolders") || !isAbs {
				continue
			}
			if len(parents) == 2 && strings.EqualFold(key, "path") {
				if _, err := strconv.ParseUint(parents[1], 10, 32); err == nil {
					paths = append(paths, value)
				}
			} else if len(parents) == 1 {
				if _, err := strconv.ParseUint(key, 10, 32); err == nil {
					paths = append(paths, value)
				}
			}
		}
	}
	if err := object(nil); err != nil {
		return nil, err
	}
	return paths, nil
}
