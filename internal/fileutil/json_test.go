package fileutil

import "testing"

func TestStrictJSONRejectsDuplicateCaseFoldedKeysAndTrailingValues(t *testing.T) {
	for _, data := range []string{
		`{"original":128,"original":32}`,
		`{"Original":128,"original":32}`,
		`{"key":1,"\u212Aey":2}`,
		`{"nested":{"enabled":true,"enabled":false}}`,
		`{"known":1} {"known":2}`,
	} {
		t.Run(data, func(t *testing.T) {
			var result map[string]any
			if err := DecodeJSONStrict([]byte(data), &result); err == nil {
				t.Fatal("ambiguous JSON was accepted")
			}
		})
	}
}

func TestStrictJSONAcceptsDistinctKeysAndRejectsUnknownStructFields(t *testing.T) {
	var result map[string]any
	if err := DecodeJSONStrict([]byte(`{"alpha":1,"beta":[{"gamma":2}]}`), &result); err != nil {
		t.Fatal(err)
	}
	var typed struct {
		Known int `json:"known"`
	}
	if err := DecodeJSONStrict([]byte(`{"known":1,"unexpected":2}`), &typed); err == nil {
		t.Fatal("unknown struct field was accepted")
	}
}
