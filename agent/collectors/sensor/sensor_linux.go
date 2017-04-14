package sensor

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/Ericsson/ericsson-hds-agent/agent/collectors"
	"github.com/Ericsson/ericsson-hds-agent/agent/log"
)

// IpmiSensorPrecheck validates for presence of dependency ipmitool and dmidecode
func IpmiSensorPrecheck() error {
	_, err := exec.LookPath("ipmitool")
	if err != nil {
		return err
	}

	dmidecode, err := exec.LookPath("dmidecode")
	if err != nil {
		return err
	}

	output, err := exec.Command(dmidecode, "--type", "38").Output()
	if err != nil {
		out := ""
		if output != nil {
			out = string(output)
		}
		return fmt.Errorf("cannot run dmidecode: %v, [%s]", err, out)
	}

	if !bytes.Contains(output, []byte("IPMI Device Information")) {
		return fmt.Errorf("No IPMI device found")
	}

	return nil
}

func formatIpmiSensor(data string) (string, string, map[string]string, error) {
	sensorColumns := []string{"name", "id", "status", "entityId", "value"}
	lines := strings.Split(data, "\n")
	headers := make([]string, 0)
	metrics := make([]string, 0)
	metadata := make(map[string]string)
	for _, line := range lines {
		columns := strings.Split(line, "|")

		if len(columns) < 5 {
			if len(line) > 0 {
				log.Info("Unexpected line: " + line)
			}
			continue //error line
		}

		columnCount := 0
		for i, value := range columns {
			if i >= len(sensorColumns) {
				continue //skip extra fields
			}
			header := strings.TrimSpace(columns[1]) + "." + sensorColumns[i] + "." + strings.TrimSpace(columns[3])
			headers = append(headers, header)
			metadata[header] = "string " + sensorColumns[i] + "of " + columns[0] + " sensor"

			strings.TrimSpace(value)
			val := strings.Replace(strings.TrimSpace(value), " ", "_", -1) //replace all ' ' to _ for correct formating output
			if len(val) == 0 {
				val = "none"
			}
			metrics = append(metrics, val)
			columnCount++
		}

		if columnCount < len(sensorColumns) {
			log.Info("less columns than expected in ipmitool sensor result")
		}
	}

	if len(headers) < 5 {
		return "", "", metadata, errors.New("Could not format ipmitool data")
	}

	return strings.Join(headers, " "), strings.Join(metrics, " "), metadata, nil
}

// IpmiSensorRun returns sensor Metric results
func IpmiSensorRun() ([]*collectors.MetricResult, error) {

	ipmitool, err := exec.LookPath("ipmitool")
	if err != nil {
		return nil, err
	}

	output, err := exec.Command(ipmitool, "sdr", "elist").Output()
	if err != nil {
		out := ""
		if output != nil {
			out = string(output)
		}
		return nil, fmt.Errorf("cannot run ipmitool: %v,[%s]", err, out)
	}
	rawData := string(output)

	header, metric, metadata, err := formatIpmiSensor(rawData)
	if err != nil {
		return nil, err
	}

	result := collectors.BuildMetricResult(header, metric, "", metadata)
	return []*collectors.MetricResult{result}, nil
}
