package log

import (
	"io"
	"log"
	"os"
	"runtime"
	"strings"
)

type LLevel int

const (
	INFO LLevel = iota
	DEBUG
	ERROR
)

var l *loggerStruct

type loggerStruct struct {
	level  LLevel
	logger *log.Logger
}

func init() {
	logFile, _ := os.OpenFile("app.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	mw := io.MultiWriter(logFile, os.Stdout)

	l = &loggerStruct{
		level:  LLevel(-1),
		logger: log.New(mw, "", log.LstdFlags),
	}
}

func getCaller() (string, int) {
	_, filename, line, ok := runtime.Caller(2)
	if !ok {
		filename = "???"
	}
	filename = filename[strings.LastIndex(filename, "/")+1:]

	return filename, line
}

func Println(msg string) {
	filename, line := getCaller()
	if INFO < l.level {
		return
	}

	l.logger.Printf("INFO %s:%d %s", filename, line, msg)
}

func Debugln(msg string, level LLevel) {
	filename, line := getCaller()
	if DEBUG < l.level {
		return
	}
	l.logger.Printf("DEBUG %s:%d %s", filename, line, msg)
}

func Panicln(err error) {
	filename, line := getCaller()
	if ERROR < l.level {
		return
	}

	l.logger.Panicf("ERROR %s:%d %v", filename, line, err)
}

func Fatalln(err error) {
	filename, line := getCaller()
	if ERROR < l.level {
		return
	}

	l.logger.Panicf("ERROR %s:%d %v", filename, line, err)
}
