package utils

import (
	"log"
	"os"
)

var debugLogging = os.Getenv("MRRSS_DEBUG") != ""

func DebugLog(format string, args ...interface{}) {
	if debugLogging {
		log.Printf(format, args...)
	}
}
