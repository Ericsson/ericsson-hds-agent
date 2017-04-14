package log

import (
	"os"
	"sync"
)

type logger struct {
	fSize       int
	logFile     *os.File
	logFileName string
	fLock       sync.Mutex
}
