package utils

import (
	"log"
	"os"
)

type Logger struct {
	isDev   bool
	isDebug bool
}

var Log *Logger

func init() {
	nodeEnv := os.Getenv("NODE_ENV")
	debug := os.Getenv("DEBUG")

	Log = &Logger{
		isDev:   nodeEnv != "production",
		isDebug: debug == "true",
	}
}

func (l *Logger) Info(format string, v ...any) {
	if l.isDev || l.isDebug {
		log.Printf("[INFO] "+format, v...)
	}
}

func (l *Logger) Warn(format string, v ...any) {
	if l.isDev || l.isDebug {
		log.Printf("[WARN] "+format, v...)
	}
}

func (l *Logger) Error(format string, v ...any) {
	// Always log errors
	log.Printf("[ERROR] "+format, v...)
}

func (l *Logger) Debug(format string, v ...any) {
	if l.isDebug {
		log.Printf("[DEBUG] "+format, v...)
	}
}
