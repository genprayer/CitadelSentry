package telemetry

import (
	"errors"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	// regexes to catch potential PII patterns
	macRegex  = regexp.MustCompile(`(?i)\b([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})\b`)
	ipv4Regex = regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`)
	steamId64 = regexp.MustCompile(`\b7656119[0-9]{10}\b`)
	emailRegex = regexp.MustCompile(`(?i)\b[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}\b`)
	longIDRegex = regexp.MustCompile(`\b[0-9]{17,20}\b`)
	windowsUserPathRegex = regexp.MustCompile(`(?i)(?:[A-Z]:)?[\\/](?:Users|Documents and Settings)[\\/][^\\/\s]+`)
	unixUserPathRegex = regexp.MustCompile(`(?:/home|/Users)/[^/\s]+`)
	ipv6Regex = regexp.MustCompile(`(?i)\b(?:[0-9a-f]{1,4}:){2,7}[0-9a-f]{1,4}\b`)
	sessionIDRegex = regexp.MustCompile(`^[0-9a-f]{32}$`)
)

// SanitizeText replaces user directory paths, IP addresses, and identifiers with placeholders.
func SanitizeText(input string) string {
	if input == "" {
		return ""
	}
	result := input

	// Sanitize Windows user directories
	userProfile := os.Getenv("USERPROFILE")
	if userProfile != "" {
		result = strings.ReplaceAll(result, userProfile, "%USERPROFILE%")
	}
	home := os.Getenv("HOME")
	if home != "" {
		result = strings.ReplaceAll(result, home, "$HOME")
	}
	username := os.Getenv("USERNAME")
	if username != "" && len(username) > 2 {
		reUser := regexp.MustCompile(`(?i)(Users[\\/])` + regexp.QuoteMeta(username))
		result = reUser.ReplaceAllString(result, `$1<ANON_USER>`)
	}
	result = windowsUserPathRegex.ReplaceAllString(result, "%USERPROFILE%")
	result = unixUserPathRegex.ReplaceAllString(result, "$HOME")

	// Mask any accidental IP addresses
	result = ipv4Regex.ReplaceAllString(result, "<MASKED_IP>")
	result = ipv6Regex.ReplaceAllString(result, "<MASKED_IP>")

	// Mask MAC addresses
	result = macRegex.ReplaceAllString(result, "<MASKED_MAC>")

	// Mask SteamIDs
	result = steamId64.ReplaceAllString(result, "<MASKED_STEAMID>")
	result = longIDRegex.ReplaceAllString(result, "<MASKED_ID>")
	result = emailRegex.ReplaceAllString(result, "<MASKED_EMAIL>")

	return result
}

// ValidateZeroPII rejects recognized PII patterns in every free-form field.
// Pattern detection cannot prove anonymity: callers must supply coarse hardware
// descriptions and never attach arbitrary user or system log strings.
func ValidateZeroPII(payload *Payload) error {
	if payload == nil {
		return errors.New("telemetry payload is nil")
	}

	if payload.SchemaVersion != "1.0.0" || !sessionIDRegex.MatchString(payload.SessionUUID) {
		return errors.New("invalid telemetry schema or ephemeral session identifier")
	}
	fields := map[string]string{
		"osCaption": payload.Hardware.OSCaption,
		"osBuild": payload.Hardware.OSBuild,
		"osArchitecture": payload.Hardware.OSArchitecture,
		"cpuModel": payload.Hardware.CPUModel,
		"gpuModel": payload.Hardware.GPUModel,
		"gpuVendor": payload.Hardware.GPUVendor,
		"gpuDriverVersion": payload.Hardware.GPUDriverVersion,
		"displayResolution": payload.Hardware.DisplayRes,
		"activePowerPlan": payload.Hardware.ActivePowerPlan,
		"driveType": payload.Hardware.DriveType,
		"squadVersion": payload.Hardware.SquadVersion,
		"sentryVersion": payload.Hardware.SentryVersion,
		"cpuPriorityStatus": payload.Diagnostics.CpuPriorityStatus,
		"ecoQosStatus": payload.Diagnostics.EcoQoSStatus,
		"exitReason": payload.Diagnostics.ExitReason,
	}
	for name, value := range fields {
		if len(value) > 192 || !utf8.ValidString(value) || strings.ContainsAny(value, "\x00\r\n") ||
			macRegex.MatchString(value) || ipv4Regex.MatchString(value) || ipv6Regex.MatchString(value) ||
			steamId64.MatchString(value) || longIDRegex.MatchString(value) || emailRegex.MatchString(value) ||
			windowsUserPathRegex.MatchString(value) || unixUserPathRegex.MatchString(value) {
			return errors.New("telemetry field " + name + " contains unsafe or oversized text")
		}
	}
	if len(payload.Diagnostics.OptimizationErrors) > maxTelemetryErrors || len(payload.Samples) > maxTelemetrySamples {
		return errors.New("telemetry collections exceed their limits")
	}
	for i := range payload.Diagnostics.OptimizationErrors {
		// Detailed errors are never suitable for the remote analytics endpoint.
		payload.Diagnostics.OptimizationErrors[i] = "REDACTED_ERROR"
	}
	for _, sample := range payload.Samples {
		switch sample.Checkpoint {
		case CheckpointT0, CheckpointPostLaunch, Checkpoint5m, Checkpoint15m, Checkpoint30m, Checkpoint60m, CheckpointExit:
		default:
			return errors.New("unrecognized telemetry checkpoint")
		}
	}

	return nil
}
