package utils

import (
	"log"
	"os"
	"strconv"
	"strings"
)

// ParseBoolEnv parses the conventional boolean values accepted by Go's
// strconv.ParseBool. Empty and invalid values are treated as defaultValue.
// Keeping this in one place prevents values such as "false" from being
// accidentally interpreted as enabled merely because they are non-empty.
func ParseBoolEnv(value string, defaultValue bool) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultValue
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return parsed
}

// DebugLoggingEnabled reports whether detailed debug logging is enabled.
func DebugLoggingEnabled() bool {
	return ParseBoolEnv(os.Getenv("MRRSS_DEBUG"), false)
}

// AccessLoggingEnabled reports whether HTTP access logging is enabled.
// Access logging is intentionally opt-in because high-frequency polling can
// otherwise generate a large amount of persistent log data.
func AccessLoggingEnabled() bool {
	return ParseBoolEnv(os.Getenv("MRRSS_ACCESS_LOG"), false)
}

var debugLogging = DebugLoggingEnabled()

func init() {
	value := strings.TrimSpace(os.Getenv("MRRSS_DEBUG"))
	if value == "" {
		return
	}
	if _, err := strconv.ParseBool(value); err != nil {
		log.Printf("invalid MRRSS_DEBUG value %q; debug logging disabled", value)
	}
}

func DebugLog(format string, args ...interface{}) {
	if debugLogging {
		log.Printf(format, args...)
	}
}
