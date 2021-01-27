package utils

import (
	"log"
	"os"
	"path/filepath"
	"time"
)

// Setting up different types of loggers for the application.
var (
	LogInfo  *log.Logger
	LogError *log.Logger
)

var layout string = "2006-01-02"

// InitLogging initializes logging for the application.
func InitLogging(environment string) {
	if environment == "production" {
		LogInfo = log.New(os.Stdout, "\tINFO:\t", log.LstdFlags|log.LUTC|log.Lshortfile|log.Lmsgprefix)
		LogError = log.New(os.Stderr, "\tERROR:\t", log.LstdFlags|log.LUTC|log.Lshortfile|log.Lmsgprefix)
	} else {
		logFilePath, err := filepath.Abs("./logs")
		if err != nil {
			log.Println("Error opening logs folder. Error:", err)
		}

		// TODO: Rotate logs everyday or based on size limit of the file
		todaysDate := time.Now().UTC().Format(layout)
		logFilePath = logFilePath + "/log_" + todaysDate + ".log"
		logFile, err := os.OpenFile(logFilePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)

		LogInfo = log.New(logFile, "\tINFO:\t", log.LstdFlags|log.LUTC|log.Lshortfile|log.Lmsgprefix)
		LogError = log.New(logFile, "\tERROR:\t", log.LstdFlags|log.LUTC|log.Lshortfile|log.Lmsgprefix)
	}
}
