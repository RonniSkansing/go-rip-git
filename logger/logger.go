package logger

import (
	"fmt"
	"log"
)

// Logger for logging all the things
type Logger struct{}

// FileAdded log file added
func (lr *Logger) FileAdded(s string) {
	log.Println(fmt.Sprintf("+ Added (%s)", s))
}

// FileSkipped log file skipped
func (lr *Logger) FileSkipped(err error) {
	log.Println(fmt.Sprintf("- Skipped (%s)", err))
}
