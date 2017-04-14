package cpu

import (
	"io/ioutil"
	"strings"

	"github.com/Ericsson/ericsson-hds-agent/agent/collectors"
	"github.com/Ericsson/ericsson-hds-agent/agent/log"
)

func loader() ([]byte, error) {
	return ioutil.ReadFile("/proc/stat")
}

func preformatter(data []byte) ([]*collectors.MetricResult, error) {
	// Linux v2.6.33+
	cpuColumns := []string{"user", "nice", "system", "idle", "iowait", "irq", "softirq", "steal", "guest", "guest_nice"}
	metadataProto := map[string]string{
		"user":          "int Time spent in user mode",
		"nice":          "int Time spent in user mode with low priority (nice)",
		"system":        "int Time spent in system mode.",
		"idle":          "int Time spent in the idle task",
		"iowait":        "int Time waiting for I/O to complete",
		"irq":           "int Time servicing interrupts",
		"softirq":       "int Time servicing softirqs",
		"steal":         "int Stolen time, which is the time spent in other operating systems when running in a virtualized environment",
		"guest":         "int Time spent running a virtual CPU",
		"guest_nice":    "int Time spent running a niced guest",
		"intr":          "int Counts of interrupts serviced since boot time",
		"ctxt":          "int Total number of context switches across all CPUs",
		"btime":         "int Time at which the system booted, in seconds since the Unix epoch (January 1, 1970)",
		"processes":     "int Number of processes and threads",
		"procs_running": "int Number of processes currently running on CPUs",
		"procs_blocked": "int Number of processes currently blocked, waiting for I/O to complete",
	}
	metadata := map[string]string{}
	lines := strings.Split(string(data), "\n")
	headers := make([]string, 0)
	metrics := make([]string, 0)
	// Could we handle all of the file's columns?
	allCols := true
	for _, line := range lines {
		columns := strings.Fields(line)
		if len(columns) < 2 {
			continue
		}

		if strings.HasPrefix(columns[0], "cpu") {
			var end int

			for i, value := range columns[1:] {
				if i >= len(cpuColumns) {
					log.Infof("more columns than expected while collecting from /proc/stat")
					break
				}
				headers = append(headers, columns[0]+"."+cpuColumns[i])
				metrics = append(metrics, value)
				if v, ok := metadataProto[cpuColumns[i]]; ok {
					metadata[columns[0]+"."+cpuColumns[i]] = v
				}
				end = i
			}

			if end+1 < len(cpuColumns) {
				allCols = false
			}
		} else {
			headers = append(headers, columns[0])
			metrics = append(metrics, columns[1])
			if v, ok := metadataProto[columns[0]]; ok {
				metadata[columns[0]] = v
			}
		}
	}

	if !allCols {
		log.Infof("fewer columns than expected while collecting from /proc/stat")
	}

	result := collectors.BuildMetricResult(strings.Join(headers, " "), strings.Join(metrics, " "), "", metadata)
	return []*collectors.MetricResult{result}, nil
}
