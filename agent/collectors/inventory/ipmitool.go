package inventory

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"

	"github.com/Ericsson/ericsson-hds-agent/agent/collectors/types"
)

func ipmiToolResultFormat(lines []string) *types.GenericInfo {
	g := types.GenericInfo{}
	g.Entries = make([]types.Entry, 0)
	e := types.Entry{}
	e.Category = "bmc"
	var d types.Detail
	for _, line := range lines {
		fields := strings.SplitN(line, ":", 2)
		if len(fields) < 2 {
			continue
		}
		tag := strings.Trim(fields[0], " ")
		value := strings.Trim(fields[1], " ")
		if tag == "" || value == "" {
			continue
		}
		d = types.Detail{Tag: tag, Value: value}

		e.Details = append(e.Details, d)
	}
	g.Entries = append(g.Entries, e)
	return &g
}

func ipmiRunCmds() ([]string, error) {
	fInfoArr, err := ioutil.ReadDir("/dev")
	if err != nil {
		return nil, fmt.Errorf("cannot read /dev: %v", err)
	}
	found := false
	for _, fileInfo := range fInfoArr {
		if strings.HasPrefix(fileInfo.Name(), "ipmi") {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("cannot run ipmitool: no ipmi devices to process in /dev/")
	}

	ipmitool, err := exec.LookPath("ipmitool")
	if err != nil {
		return nil, err
	}

	output, err := exec.Command(ipmitool, "mc", "info").Output()
	if err != nil {
		out := ""
		if output != nil {
			out = string(output)
		}
		return nil, fmt.Errorf("cannot run ipmitool: %v,[%s]", err, out)
	}
	lines := strings.Split(string(output), "\n")

	output, err = exec.Command(ipmitool, "lan", "print").Output()
	if err != nil {
		out := ""
		if output != nil {
			out = string(output)
		}
		return nil, fmt.Errorf("cannot run ipmitool: %v,[%s]", err, out)
	}
	lines = append(lines, strings.Split(string(output), "\n")...)
	return lines, nil
}

// IpmiToolRun returns inventory of Management Controller and LAN channels
func IpmiToolRun() ([]byte, error) {
	lines, err := ipmiRunCmds()
	if err != nil {
		return nil, err
	}
	result := ipmiToolResultFormat(lines)
	rjson, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return rjson, nil
}
