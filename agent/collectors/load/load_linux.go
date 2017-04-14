package load

import (
	"io/ioutil"
	"strings"

	"github.com/Ericsson/ericsson-hds-agent/agent/collectors"
)

func loader() ([]byte, error) {
	return ioutil.ReadFile("/proc/loadavg")
}

func preformatter(data []byte) ([]*collectors.MetricResult, error) {

	metadataProto := "float The load average in regard to both the CPU and IO over time"
	metadata := map[string]string{
		"last1min":  metadataProto,
		"last5min":  metadataProto,
		"last15min": metadataProto,
	}
	parts := strings.Fields(string(data))
	result := collectors.BuildMetricResult("last1min last5min last15min", parts[0]+" "+parts[1]+" "+parts[2], "", metadata)
	return []*collectors.MetricResult{result}, nil
}
