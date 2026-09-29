package utils

import (
	"fmt"
	"log"
	"os"
)

func NewLogger(prefix string) *log.Logger {
	modifiedPrefix := fmt.Sprintf("[%s] ", prefix)
	var Logger = log.New(
		os.Stdout,
		modifiedPrefix,
		log.Ldate|log.Ltime|log.Lshortfile,
	)

	return Logger
}

var DebugLogger *log.Logger = NewLogger("DEBUG")
