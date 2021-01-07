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

// Init initializes logging for the application.
func Init() {
	logFilePath, err := filepath.Abs("./logs")
	if err != nil {
		log.Println("Error opening logs folder. Error:", err)
	}

	// TODO: Rotate logs everyday or based on size limit of the file
	currentTimeInUTC := time.Now().UTC().Format(layout)

	myfilepath := logFilePath + "/log_" + currentTimeInUTC + ".log"
	logFile, err := os.OpenFile(myfilepath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)

	LogInfo = log.New(logFile, "\tINFO:\t", log.Ldate|log.Ltime|log.LUTC|log.Lshortfile|log.Lmsgprefix)
	LogError = log.New(logFile, "\tERROR:\t", log.Ldate|log.Ltime|log.LUTC|log.Lshortfile|log.Lmsgprefix)
}
