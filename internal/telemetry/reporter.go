package telemetry

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"citadel-seeder/internal/logger"
)

// Reporter manages the collection, sampling, and transmission of anonymous telemetry.
type Reporter struct {
	mu          sync.Mutex
	sessionUUID string
	uuidError   error
	optIn       bool
	sending     bool
	sendCancel  context.CancelFunc
	apiEndpoint string
	client      *http.Client
	payload     Payload
	startTime   time.Time
}

const (
	maxTelemetrySamples      = 16
	maxTelemetryErrors       = 32
	maxTelemetryPayloadBytes = 64 << 10
)

// NewReporter creates a new telemetry reporter with a random per-launch ephemeral session UUID.
func NewReporter(optIn bool, apiEndpoint, sentryVersion string) *Reporter {
	uuidStr, uuidErr := newSessionUUID(rand.Reader)

	if apiEndpoint == "" {
		apiEndpoint = "https://jointhectl.com/api/telemetry/sentry"
	}

	return &Reporter{
		sessionUUID: uuidStr,
		uuidError:   uuidErr,
		optIn:       optIn,
		apiEndpoint: apiEndpoint,
		client: &http.Client{
			Timeout:       10 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		payload: Payload{
			SchemaVersion: "1.0.0",
			SessionUUID:   uuidStr,
			Hardware: HardwareConfig{
				SentryVersion: sentryVersion,
			},
			Samples: make([]DynamicMetrics, 0, maxTelemetrySamples),
		},
		startTime: time.Now(),
	}
}

func newSessionUUID(reader io.Reader) (string, error) {
	uuidBytes := make([]byte, 16)
	if _, err := io.ReadFull(reader, uuidBytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(uuidBytes), nil
}

// SetOptIn updates whether the user has authorized telemetry collection.
func (r *Reporter) SetOptIn(enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.optIn = enabled
	if !enabled {
		if r.sendCancel != nil {
			r.sendCancel()
		}
		// Revocation also clears previously collected data from memory.
		version := r.payload.Hardware.SentryVersion
		r.payload.Hardware = HardwareConfig{SentryVersion: version}
		r.payload.Diagnostics = SeedingDiagnostics{}
		r.payload.Samples = nil
	}
}

// IsOptIn reports whether telemetry is enabled.
func (r *Reporter) IsOptIn() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.optIn
}

// SetHardware updates the static hardware baseline.
func (r *Reporter) SetHardware(hw HardwareConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.optIn {
		return
	}
	hw.SentryVersion = r.payload.Hardware.SentryVersion
	r.payload.Hardware = hw
}

// RecordSample records a dynamic checkpoint measurement.
func (r *Reporter) RecordSample(sample DynamicMetrics) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.optIn {
		return
	}
	sample.Timestamp = time.Now()
	if len(r.payload.Samples) == maxTelemetrySamples {
		copy(r.payload.Samples, r.payload.Samples[1:])
		r.payload.Samples[maxTelemetrySamples-1] = sample
	} else {
		r.payload.Samples = append(r.payload.Samples, sample)
	}
}

// SetDiagnostics updates the session outcome and optimization status.
func (r *Reporter) SetDiagnostics(diag SeedingDiagnostics) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.optIn {
		return
	}
	diag.SessionDurationSeconds = int64(time.Since(r.startTime).Seconds())
	if len(diag.OptimizationErrors) > maxTelemetryErrors {
		diag.OptimizationErrors = diag.OptimizationErrors[:maxTelemetryErrors]
	}
	// Never retain or transmit arbitrary OS/game error text: it can contain
	// credentials, home directories, account names, or unexpected identifiers.
	if len(diag.OptimizationErrors) != 0 {
		redacted := make([]string, len(diag.OptimizationErrors))
		for i := range redacted {
			redacted[i] = "REDACTED_ERROR"
		}
		diag.OptimizationErrors = redacted
	}
	r.payload.Diagnostics = diag
}

func clonePayload(p Payload) Payload {
	copyPayload := p
	copyPayload.Samples = append([]DynamicMetrics(nil), p.Samples...)
	copyPayload.Diagnostics.OptimizationErrors = append([]string(nil), p.Diagnostics.OptimizationErrors...)
	return copyPayload
}

// GetPreviewPayload returns the current sanitized payload for user inspection in UI.
func (r *Reporter) GetPreviewPayload() (Payload, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	copyPayload := clonePayload(r.payload)
	if err := ValidateZeroPII(&copyPayload); err != nil {
		return Payload{}, err
	}
	return copyPayload, nil
}

// Dispatch sends the report synchronously. Callers must keep it off the seeding
// critical path; errors must be handled as non-fatal by the caller.
func (r *Reporter) Dispatch(ctx context.Context) error {
	r.mu.Lock()
	if !r.optIn {
		r.mu.Unlock()
		return nil // Graceful no-op when opt-out
	}
	if r.uuidError != nil {
		r.mu.Unlock()
		return fmt.Errorf("telemetry unavailable: random session ID generation failed: %w", r.uuidError)
	}
	if r.sending {
		r.mu.Unlock()
		return errors.New("telemetry dispatch already in progress")
	}
	sendCtx, cancel := context.WithCancel(ctx)
	r.sendCancel = cancel
	r.sending = true
	r.payload.Diagnostics.SessionDurationSeconds = int64(time.Since(r.startTime).Seconds())
	copyPayload := clonePayload(r.payload)
	endpoint := r.apiEndpoint
	client := r.client
	r.mu.Unlock()
	defer func() {
		cancel()
		r.mu.Lock()
		r.sendCancel = nil
		r.sending = false
		r.mu.Unlock()
	}()
	if err := validateTelemetryEndpoint(endpoint); err != nil {
		return err
	}

	if err := ValidateZeroPII(&copyPayload); err != nil {
		logger.Warn("Telemetry transmission skipped due to PII validation check", "error", err)
		return err
	}

	data, err := json.Marshal(copyPayload)
	if err != nil {
		return fmt.Errorf("marshal telemetry payload: %w", err)
	}
	if len(data) > maxTelemetryPayloadBytes {
		return fmt.Errorf("telemetry payload exceeds %d bytes", maxTelemetryPayloadBytes)
	}

	// Gzip compression
	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	if _, err := gzWriter.Write(data); err != nil {
		return fmt.Errorf("compress telemetry: %w", err)
	}
	if err := gzWriter.Close(); err != nil {
		return fmt.Errorf("close gzip writer: %w", err)
	}

	if sendCtx.Err() != nil {
		return nil // Consent was revoked or caller canceled before transmission.
	}
	req, err := http.NewRequestWithContext(sendCtx, http.MethodPost, endpoint, &buf)
	if err != nil {
		return fmt.Errorf("create telemetry request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("User-Agent", fmt.Sprintf("CitadelSentry/%s (Telemetry)", copyPayload.Hardware.SentryVersion))

	resp, err := client.Do(req)
	if err != nil {
		// Log warning only; telemetry failure must never interrupt application execution
		logger.Warn("Telemetry report transmission failed (non-fatal)", "error", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logger.Warn("Telemetry endpoint returned error status", "statusCode", resp.StatusCode)
		return errors.New("telemetry server rejected report")
	}

	logger.Info("Anonymous telemetry report delivered successfully", "samples", len(copyPayload.Samples))
	return nil
}

func validateTelemetryEndpoint(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" {
		return errors.New("invalid telemetry endpoint")
	}
	if u.Scheme == "https" {
		return nil
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	if u.Scheme == "http" && (host == "localhost" || (ip != nil && ip.IsLoopback())) {
		return nil // Local-only HTTP permits isolated httptest integration tests.
	}
	return errors.New("telemetry endpoint must use HTTPS")
}
