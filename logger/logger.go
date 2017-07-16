// TODO: Fix logging output its messed up

package logger

import (
	"fmt"
	"log"
)

// Logger for logging all the things
type Logger struct{}

// Error log a error
func (lr *Logger) Error(err error, s ...string) {
	log.Fatalln(fmt.Sprintf("Error : %v (%s)", s, err))
}

// Info log info
func (lr *Logger) Info(s ...string) {
	log.Println(fmt.Sprintf("Info : %v", s))

}

// FileAdded log file added
func (lr *Logger) FileAdded(s ...string) {
	log.Println(fmt.Sprintf("+ Added %v", s))
}

// FileSkipped log file skipped
func (lr *Logger) FileSkipped(err error, s ...string) {
	log.Println(fmt.Sprintf("- Skipped %v (%s)", s, err))
}
