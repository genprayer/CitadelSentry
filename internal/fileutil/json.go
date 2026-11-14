package fileutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"
)

const maxJSONNesting = 64

// DecodeJSONStrict rejects unknown struct fields, duplicate keys and trailing values.
func DecodeJSONStrict(data []byte, destination any) error {
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("unexpected trailing JSON data")
	}
	return nil
}

func rejectDuplicateJSONKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var walk func(int) error
	walk = func(depth int) error {
		if depth > maxJSONNesting {
			return fmt.Errorf("JSON nesting exceeds its limit")
		}
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delimiter, isContainer := token.(json.Delim)
		if !isContainer {
			return nil
		}
		switch delimiter {
		case '{':
			seen := make(map[string]bool)
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return err
				}
				key, isString := keyToken.(string)
				if !isString {
					return fmt.Errorf("invalid JSON key")
				}
				folded := strings.Map(func(character rune) rune {
					for next := unicode.SimpleFold(character); next > character; next = unicode.SimpleFold(next) {
						character = next
					}
					return unicode.SimpleFold(character)
				}, key)
				if seen[folded] {
					return fmt.Errorf("duplicate JSON key: %s", key)
				}
				seen[folded] = true
				if err := walk(depth + 1); err != nil {
					return err
				}
			}
		case '[':
			for decoder.More() {
				if err := walk(depth + 1); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("unexpected JSON delimiter")
		}
		_, err = decoder.Token()
		return err
	}
	return walk(0)
}
