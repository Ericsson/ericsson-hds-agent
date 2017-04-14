package net

import (
	"github.com/Ericsson/ericsson-hds-agent/agent/collectors"
)

// Run returns network interface metrics
func Run() ([]*collectors.MetricResult, error) {

	data, err := loader()
	if err != nil {
		return nil, err
	}

	return preformatter(data)
}
