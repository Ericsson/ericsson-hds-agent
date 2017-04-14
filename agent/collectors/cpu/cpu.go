package cpu

import (
	"github.com/Ericsson/ericsson-hds-agent/agent/collectors"
)

// Run returns cpu metrics
func Run() ([]*collectors.MetricResult, error) {
	data, err := loader()
	if err != nil {
		return nil, err
	}

	result, err := preformatter(data)
	if err != nil {
		return nil, err
	}
	return result, nil
}
