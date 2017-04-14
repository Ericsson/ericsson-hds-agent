package uptime

import (
	"github.com/Ericsson/ericsson-hds-agent/agent/collectors"
)

// Run returns uptime Metric results
func Run() ([]*collectors.MetricResult, error) {
	var (
		data []byte
		err  error
	)
	if data, err = loader(); err != nil {
		return nil, err
	}
	return preformatter(data)
}
