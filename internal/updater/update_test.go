package updater

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type releaseFixtureTransport func(*http.Request) (*http.Response, error)

func (f releaseFixtureTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestLatestRejectsIncompleteOrAmbiguousReleaseMetadata(t *testing.T) {
	name, err := expectedAsset("v1.2.3")
	if err != nil {
		t.Skip(err)
	}
	valid := map[string]any{"tag_name": "v1.2.3", "draft": false, "prerelease": false, "assets": []any{map[string]any{"name": name, "size": 128, "state": "uploaded"}}}
	cases := map[string]string{}
	for _, field := range []string{"draft", "prerelease", "assets"} {
		for _, isNull := range []bool{false, true} {
			row := map[string]any{}
			for key, value := range valid {
				row[key] = value
			}
			delete(row, field)
			label := "missing-" + field
			if isNull {
				row[field] = nil
				label = "null-" + field
			}
			data, err := json.Marshal(row)
			if err != nil {
				t.Fatal(err)
			}
			cases[label] = string(data)
		}
	}
	for label, assets := range map[string][]any{
		"duplicate-assets": {map[string]any{"name": name, "size": 128, "state": "uploaded"}, map[string]any{"name": name, "size": 128, "state": "uploaded"}},
		"missing-state":    {map[string]any{"name": name, "size": 128}},
		"null-state":       {map[string]any{"name": name, "size": 128, "state": nil}},
		"unfinished-asset": {map[string]any{"name": name, "size": 128, "state": "starter"}},
	} {
		row := map[string]any{}
		for key, value := range valid {
			row[key] = value
		}
		row["assets"] = assets
		data, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		cases[label] = string(data)
	}
	encoded, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	cases["duplicate-draft"] = strings.Replace(string(encoded), `"draft":false`, `"draft":true,"draft":false`, 1)
	cases["case-mismatched-state"] = strings.Replace(string(encoded), `"state"`, `"State"`, 1)
	for label, payload := range cases {
		t.Run(label, func(t *testing.T) {
			manager := NewManager("1.2.2")
			manager.client.Transport = releaseFixtureTransport(func(request *http.Request) (*http.Response, error) {
				if request.URL.String() != releaseAPI {
					t.Fatalf("unexpected download request: %s", request.URL)
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(payload)), Header: make(http.Header)}, nil
			})
			if _, err := manager.Check(context.Background()); err == nil {
				t.Fatal("ambiguous release was presented as a ready stable update")
			}
		})
	}
}

func TestReleaseValidationPreservesExplicitFlagsAndNullableDigest(t *testing.T) {
	for _, payload := range []string{
		`{"tag_name":"v1.2.3","draft":false,"prerelease":false,"assets":[],"futureMetadata":{"value":true}}`,
		`{"tag_name":"v1.2.3","draft":true,"prerelease":false,"assets":[{"name":"optional.bin","size":0,"state":"starter","digest":null}]}`,
		`{"tag_name":"v1.2.3","draft":false,"prerelease":true,"assets":[{"name":"optional.bin","size":128,"state":"uploaded"}]}`,
	} {
		published, err := decodeRelease([]byte(payload))
		if err != nil {
			t.Fatal(err)
		}
		if published.TagName != "v1.2.3" {
			t.Fatal("explicit release version changed")
		}
		if strings.Contains(payload, `"draft":true`) && !published.Draft {
			t.Fatal("draft flag was replaced")
		}
		if strings.Contains(payload, `"prerelease":true`) && !published.Prerelease {
			t.Fatal("prerelease flag was replaced")
		}
	}
}

func TestLatestAcceptsOneUploadedPlatformAsset(t *testing.T) {
	name, err := expectedAsset("v1.2.3")
	if err != nil {
		t.Skip(err)
	}
	payload, err := json.Marshal(map[string]any{
		"tag_name": "v1.2.3", "draft": false, "prerelease": false,
		"assets": []any{map[string]any{"name": name, "size": 128, "state": "uploaded", "digest": nil}},
	})
	if err != nil {
		t.Fatal(err)
	}
	manager := NewManager("1.2.2")
	manager.client.Transport = releaseFixtureTransport(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != releaseAPI {
			t.Fatalf("unexpected request: %s", request.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(string(payload))), Header: make(http.Header)}, nil
	})
	info, err := manager.Check(context.Background())
	if err != nil || !info.Available || info.AssetName != name || info.AssetSize != 128 {
		t.Fatalf("explicit release was rejected: %+v, %v", info, err)
	}
}

func TestParseVersionRejectsAmbiguousLeadingZeros(t *testing.T) {
	for _, version := range []string{"01.2.3", "1.02.3", "v1.2.03"} {
		if _, err := parseVersion(version); err == nil {
			t.Errorf("invalid version %q was normalized into another version", version)
		}
	}
	for _, version := range []string{"0.0.0", "1.2.3", "v1.2.3"} {
		if _, err := parseVersion(version); err != nil {
			t.Errorf("valid version %q: %v", version, err)
		}
	}
}

func TestChecksumForRejectsDuplicateAssetEntries(t *testing.T) {
	name := "application.bin"
	first := strings.Repeat("a", 64)
	second := strings.Repeat("b", 64)
	for _, other := range []string{first, second} {
		data := []byte(first + "  " + name + "\n" + other + " *" + name + "\n")
		if _, err := checksumFor(data, name); err == nil {
			t.Fatal("duplicate checksum entries were resolved by selecting the first value")
		}
	}
}

func TestChecksumForUsesExactlyTheRequestedAsset(t *testing.T) {
	want := strings.Repeat("a", 64)
	data := []byte(strings.Repeat("b", 64) + "  application.bin.old\n" + want + " *application.bin\n")
	got, err := checksumFor(data, "application.bin")
	if err != nil || got != want {
		t.Fatalf("checksum=%q error=%v", got, err)
	}
}
