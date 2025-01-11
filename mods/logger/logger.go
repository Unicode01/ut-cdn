package logger

import (
	"fmt"
	"time"
)

var (
	LogLevel = 1 //default log level is 1
)

func SetLoggerLevel(level int) {
	LogLevel = level
}

func Log(msg string, level int) {
	if LogLevel > level {
		return
	}
	if level == 1 { //normal
		fmt.Println("[INFO]" + time.Now().Format("2006-01-02 15:04:05") + " " + msg)
	} else if level == 2 { //warning
		fmt.Println("[WARNING]" + time.Now().Format("2006-01-02 15:04:05") + " " + msg)
	} else if level == 3 { //error
		fmt.Println("[ERROR]" + time.Now().Format("2006-01-02 15:04:05") + " " + msg)
	} else { //unknown level
		fmt.Println("[UNKNOWN]" + time.Now().Format("2006-01-02 15:04:05") + " " + msg)
	}
}
