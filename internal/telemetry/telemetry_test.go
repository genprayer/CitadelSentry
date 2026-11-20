package telemetry

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSanitizeTextKnownPII(t *testing.T) {
	t.Setenv("USERPROFILE", `C:\Users\JohnDoe`)
	t.Setenv("USERNAME", "JohnDoe")
	raw := `C:\Users\JohnDoe\AppData 192.168.1.50 00:1A:2B:3C:4D:5E 76561198012345678 john@example.com`
	sanitized := SanitizeText(raw)
	for _, secret := range []string{`C:\Users\JohnDoe`, "192.168.1.50", "00:1A:2B:3C:4D:5E", "76561198012345678", "john@example.com"} {
		if strings.Contains(sanitized, secret) {
			t.Errorf("PII was not masked: %q", secret)
		}
	}
}

func TestReporterActualGzipJSONAndOptOut(t *testing.T) {
	var hits atomic.Int32
	received := make(chan Payload, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		hits.Add(1)
		if req.Method != http.MethodPost || req.Header.Get("Content-Encoding") != "gzip" || req.Header.Get("Content-Type") != "application/json" {
			t.Errorf("invalid request headers or method")
		}
		reader, err := gzip.NewReader(req.Body)
		if err != nil {
			t.Errorf("invalid gzip: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer reader.Close()
		body, err := io.ReadAll(io.LimitReader(reader, maxTelemetryPayloadBytes+1))
		if err != nil {
			t.Errorf("read gzip JSON: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var payload Payload
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("invalid JSON: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		received <- payload
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	reporter := NewReporter(false, server.URL, "1.3.2")
	reporter.RecordSample(DynamicMetrics{Checkpoint: CheckpointT0})
	if err := reporter.Dispatch(context.Background()); err != nil || hits.Load() != 0 {
		t.Fatalf("opt-out transmitted telemetry: count=%d err=%v", hits.Load(), err)
	}
	reporter.SetOptIn(true)
	reporter.SetHardware(HardwareConfig{CPUModel: "AMD Ryzen 7", GPUModel: "NVIDIA RTX 4080"})
	reporter.SetDiagnostics(SeedingDiagnostics{OptimizationErrors: []string{`C:\Users\JohnDoe\secret`}})
	reporter.RecordSample(DynamicMetrics{Checkpoint: CheckpointT0, SquadCPUPercent: 4.2})
	if err := reporter.Dispatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 1 {
		t.Fatalf("expected one POST, got %d", hits.Load())
	}
	select {
	case p := <-received:
		if p.Hardware.CPUModel != "AMD Ryzen 7" || len(p.Samples) != 1 || p.Samples[0].SquadCPUPercent != 4.2 {
			t.Fatalf("unexpected decoded JSON payload: %+v", p)
		}
		if len(p.Diagnostics.OptimizationErrors) != 1 || p.Diagnostics.OptimizationErrors[0] != "REDACTED_ERROR" {
			t.Fatalf("raw OS error escaped: %+v", p.Diagnostics.OptimizationErrors)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not receive JSON")
	}
}

func TestDeepCopyCapAndRevocation(t *testing.T) {
	r := NewReporter(true, "https://example.com/api/telemetry", "1.3.2")
	r.SetHardware(HardwareConfig{CPUModel: "Intel Core"})
	r.SetDiagnostics(SeedingDiagnostics{OptimizationErrors: []string{"raw secret"}})
	for i := 0; i < 100; i++ {
		r.RecordSample(DynamicMetrics{Checkpoint: CheckpointT0, SquadCPUPercent: float64(i)})
	}
	p, err := r.GetPreviewPayload()
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Samples) != maxTelemetrySamples || p.Samples[0].SquadCPUPercent != 84 {
		t.Fatalf("sample cap failed: %+v", p.Samples)
	}
	p.Samples[0].SquadCPUPercent = 12345
	p.Diagnostics.OptimizationErrors[0] = "MODIFIED"
	p, err = r.GetPreviewPayload()
	if err != nil || p.Samples[0].SquadCPUPercent != 84 || p.Diagnostics.OptimizationErrors[0] != "REDACTED_ERROR" {
		t.Fatalf("snapshot aliases live buffers: %+v err=%v", p, err)
	}
	r.SetOptIn(false)
	r.SetDiagnostics(SeedingDiagnostics{ExitReason: "should not collect"})
	p, err = r.GetPreviewPayload()
	if err != nil || len(p.Samples) != 0 || p.Hardware.CPUModel != "" || p.Diagnostics.ExitReason != "" {
		t.Fatalf("revocation failed to erase data: %+v err=%v", p, err)
	}
}

func TestValidateEveryHardwareString(t *testing.T) {
	typ := reflect.TypeOf(HardwareConfig{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.Type.Kind() != reflect.String || field.Name == "SentryVersion" {
			continue
		}
		t.Run(field.Name, func(t *testing.T) {
			r := NewReporter(true, "https://example.com", "1.3.2")
			hw := HardwareConfig{}
			reflect.ValueOf(&hw).Elem().FieldByName(field.Name).SetString("leak@example.com")
			r.SetHardware(hw)
			if _, err := r.GetPreviewPayload(); err == nil {
				t.Fatal("PII field was not rejected")
			}
		})
	}
	r := NewReporter(true, "https://example.com", "leak@example.com")
	if _, err := r.GetPreviewPayload(); err == nil {
		t.Fatal("unsafe sentryVersion was accepted")
	}
	for _, status := range []struct {
		name string
		set  func(*SeedingDiagnostics)
	}{
		{"cpuPriorityStatus", func(d *SeedingDiagnostics) { d.CpuPriorityStatus = "leak@example.com" }},
		{"ecoQosStatus", func(d *SeedingDiagnostics) { d.EcoQoSStatus = "leak@example.com" }},
		{"exitReason", func(d *SeedingDiagnostics) { d.ExitReason = "leak@example.com" }},
	} {
		t.Run(status.name, func(t *testing.T) {
			r := NewReporter(true, "https://example.com", "1.3.2")
			d := SeedingDiagnostics{}
			status.set(&d)
			r.SetDiagnostics(d)
			if _, err := r.GetPreviewPayload(); err == nil {
				t.Fatal("unsafe diagnostic status was accepted")
			}
		})
	}
}

func TestEndpointRestrictions(t *testing.T) {
	r := NewReporter(true, "http://example.com/telemetry", "1.3.2")
	if err := r.Dispatch(context.Background()); err == nil {
		t.Fatal("plain HTTP to public server accepted")
	}
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		hits.Add(1)
		w.Header().Set("Location", "http://example.com/telemetry")
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	r = NewReporter(true, server.URL, "1.3.2")
	if err := r.Dispatch(context.Background()); err == nil || hits.Load() != 1 {
		t.Fatalf("redirect followed or accepted: hits=%d err=%v", hits.Load(), err)
	}
}

func TestRevokeCancelsInFlightDispatch(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		close(started)
		select {
		case <-req.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer server.Close()
	r := NewReporter(true, server.URL, "1.3.2")
	done := make(chan error, 1)
	go func() { done <- r.Dispatch(context.Background()) }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("request did not reach server")
	}
	r.SetOptIn(false)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("revocation did not cancel request")
	}
}

func TestConcurrentSnapshotsAndEntropyFailure(t *testing.T) {
	if id, err := newSessionUUID(strings.NewReader("")); err == nil || id != "" {
		t.Fatalf("missing cryptographic entropy accepted: id=%q err=%v", id, err)
	}
	r := NewReporter(true, "https://example.com", "1.3.2")
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for j := 0; j < 100; j++ {
				r.RecordSample(DynamicMetrics{Checkpoint: CheckpointT0})
				_, _ = r.GetPreviewPayload()
			}
		}()
	}
	group.Wait()
}
