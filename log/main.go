package log

import (
	"fmt"
	"os"
)

func Error(format string, i ...interface{}) {
	Log("ERROR", format, i...)
}

func Fatal(format string, i ...interface{}) {
	Error(format, i...)
	os.Exit(1)
}

func Info(format string, i ...interface{}) {
	Log("INFO ", format, i...)
}

func Log(tag, format string, i ...interface{}) {
	fmt.Printf(tag+" "+format+"\n", i...)
}
