package agent

import (
	"fmt"
	"time"
)

const (
	layout = "Jan 02 15:04:05"
)

// Syslog contains different fileds to format log message into more readable
type Syslog struct {
	Tag, Hostname, Message string
	Timestamp              time.Time
	Facility, Severity     int
}

func (s *Syslog) format() string {
	return fmt.Sprintf("<%d> %s %s %s[]: %s", s.Facility*8+s.Severity, s.Timestamp.Format(layout), s.Hostname, s.Tag, s.Message)
}

func (s *Syslog) formatBytes() []byte {
	return []byte(s.format())
}
