package logger

import (
	"fmt"
	"log"
)

// Logger for logging all the things
type Logger struct{}

// Error log a error and exit
func (lr *Logger) Error(err error) {
	log.Fatalln(fmt.Sprintf("Error : %v", err))
}

// Info log info
func (lr *Logger) Info(s string) {
	log.Println(fmt.Sprintf("Info : %s", s))
}

// Entry logs entry
func (lr *Logger) Entry(s string) {
	fmt.Println(s)
}

// FileAdded log file added
func (lr *Logger) FileAdded(s string) {
	log.Println(fmt.Sprintf("+ Added (%s)", s))
}

// FileSkipped log file skipped
func (lr *Logger) FileSkipped(err error) {
	log.Println(fmt.Sprintf("- Skipped (%s)", err))
}
