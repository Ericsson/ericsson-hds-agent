package uptime

import (
	"io/ioutil"
	"strings"

	"github.com/Ericsson/ericsson-hds-agent/agent/collectors"
)

const (
	uptime = "Uptime"
	idle   = "Idle"
)

func loader() ([]byte, error) {
	return ioutil.ReadFile("/proc/uptime")
}

func preformatter(data []byte) ([]*collectors.MetricResult, error) {
	metadata := map[string]string{
		uptime: "float The total number of seconds the system has been up",
		idle:   "float  Column is how much of that time the machine has spent idle, in seconds.",
	}

	result := collectors.BuildMetricResult(uptime+" "+idle, strings.TrimSpace(string(data)), "", metadata)
	return []*collectors.MetricResult{result}, nil
}
