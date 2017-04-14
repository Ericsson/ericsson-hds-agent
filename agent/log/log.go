// Package log implements logging with 2 rule:
// a. Information should go to log file (it will be created in /tmp folder).
// b. Error also should go to stderr
package log

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

const maxFileSize int = 1024 * 1024 * 50

var (
	infoLog  logger
	errLog   logger
	execName = filepath.Base(os.Args[0])
)

// Info logs with [info] tag in prefix
func Info(str string) {
	msg := time.Now().Format("2006-01-02 15:04:05.1111") + " [INFO]: " + str + "\n"
	toLogFile(msg)
}

// Infof logs with [info] tag in prefix
// Arguments are handled in the manner of fmt.Sprintf
func Infof(format string, args ...interface{}) {
	Info(fmt.Sprintf(format, args...))
}

// ReadLogFile returns contents of file as string
func ReadLogFile() string {
	infoLog.fLock.Lock()
	defer infoLog.fLock.Unlock()

	if infoLog.logFile != nil {
		data, err := ioutil.ReadFile(infoLog.logFileName)
		if err == nil {
			return string(data)
		}
	}

	return ""
}

func toLogFile(msg string) {
	infoLog.fLock.Lock()
	defer infoLog.fLock.Unlock()

	if infoLog.logFile == nil {
		timeNow := time.Now().Format("-2006-01-02-15:04:05")
		infoLog.logFileName = filepath.Join(os.TempDir(), fmt.Sprintf("%s.INFO", execName+timeNow))
		var err error
		infoLog.logFile, err = os.Create(infoLog.logFileName)
		if err != nil {
			infoLog.logFileName = ""
			return //we can't create file so just skip
		}
	}

	if _, err := infoLog.logFile.WriteString(msg); err != nil || infoLog.fSize > maxFileSize {

		infoLog.logFile.Sync()  //ignore error
		infoLog.logFile.Close() //ignore error
		infoLog.logFile = nil   //try get new file
		infoLog.fSize = 0
	} else {
		infoLog.fSize += len(msg)
	}
}

func toErrFile(msg string) {
	errLog.fLock.Lock()
	defer errLog.fLock.Unlock()

	if errLog.logFile == nil {
		timeNow := time.Now().Format("-2006-01-02-15:04:05")
		errLog.logFileName = filepath.Join(os.TempDir(), fmt.Sprintf("%s.ERROR", execName+timeNow))
		var err error
		errLog.logFile, err = os.Create(errLog.logFileName)
		if err != nil {
			errLog.logFileName = ""
			return //we can't create file so just skip
		}
	}

	if _, err := errLog.logFile.WriteString(msg); err != nil || errLog.fSize > maxFileSize {

		errLog.logFile.Sync()  //ignore error
		errLog.logFile.Close() //ignore error
		errLog.logFile = nil   //try get new file
		errLog.fSize = 0
	} else {
		errLog.fSize += len(msg)
	}
}

// Error logs with [error] tag in prefix
func Error(str string) {
	msg := time.Now().Format("2006-01-02 15:04:05") + " [ERROR]: " + str + "\n"
	os.Stderr.WriteString(msg)
	toErrFile(msg)
}

// Errorf logs with [error] tag in prefix
// Arguments are handled in the manner of fmt.Sprintf
func Errorf(format string, args ...interface{}) {
	Error(fmt.Sprintf(format, args...))
}

// Console logs on console or stdout
func Console(str string) {
	msg := time.Now().Format("2006-01-02 15:04:05") + ": " + str + "\n"
	os.Stdout.WriteString(msg)
}

// Consolef logs on console or stdout
// Arguments are handled in the manner of fmt.Sprintf
func Consolef(format string, args ...interface{}) {
	Console(fmt.Sprintf(format, args...))
}
