// Package logging pkg/logging/logging.go
package logging

import (
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

var Log = logrus.New()

// InitLogger initializes the logging configuration
func InitLogger(logLevel logrus.Level) {
	Log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	Log.SetOutput(os.Stdout)
	Log.SetLevel(logLevel)
}

// SetLogLevel allows setting the log level dynamically
func SetLogLevel(level string) {
	parsedLevel, err := logrus.ParseLevel(level)
	if err != nil {
		Error(fmt.Sprintf("Invalid log level: %s", level), err)
		return
	}
	Log.SetLevel(parsedLevel)
	Info(fmt.Sprintf("Log level set to: %s", level))
}

// Title prints a formatted section title
func Title(title string) {
	border := strings.Repeat("#", len(title)+8)
	fmt.Println(border)
	fmt.Printf("### %s ###\n", title)
	fmt.Println(border)
}

// Ok Print a success message
func Ok(message ...string) {
	if len(message) == 0 {
		fmt.Println(" ok")
	} else {
		fmt.Println(message[0])
	}
}

// Info logs an informational message
func Info(message string) {
	Log.Info(message)
}

// Step logs an informational message
func Step(message string) {
	Log.Info(fmt.Sprintf("\n*====> %s", message))
}

// InfoMessage logs an informational message
func InfoMessage(message string, fields map[string]interface{}) {
	Log.WithFields(fields).Info(message)
}

// Warn logs an error message
func Warn(message string) {
	Log.Warn(fmt.Sprintf("*====> [!]: %s", message))
}

// Error logs an error message
func Error(message string, err error) {
	Log.WithError(err).Error(message)
}

// Fatal logs an error and exits the program
func Fatal(message string, err error) {
	Log.WithError(err).Fatal(message)
}
