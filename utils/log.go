package utils

import (
	log "github.com/sirupsen/logrus"
)

// Logger is the shared logger instance used throughout the project.
var Logger = log.StandardLogger()

// InitLogger configures the shared logger to emit JSON formatted output.
func InitLogger(level string) error {
	parsedLevel, err := log.ParseLevel(level)
	if err != nil {
		return err
	}
	Logger.SetFormatter(&log.JSONFormatter{})
	Logger.SetLevel(parsedLevel)
	return nil
}
