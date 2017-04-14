package inventory

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/Ericsson/ericsson-hds-agent/agent/collectors/types"
)

func lsusbResultFormat(sdevs []string) *types.GenericInfo {
	g := types.GenericInfo{}
	g.Entries = make([]types.Entry, 0)
	for _, sdev := range sdevs {
		if sdev == "" {
			continue
		}
		lines := strings.Split(sdev, ": ")
		e := types.Entry{}
		e.Category = lines[0]
		var d types.Detail
		if len(lines) < 1 {
			continue
		}
		fields := strings.Fields(lines[1])
		if len(fields) < 3 {
			continue
		}
		d = types.Detail{Tag: strings.TrimSpace(fields[0]), Value: strings.TrimSpace(fields[1])}
		e.Details = append(e.Details, d)
		d = types.Detail{Tag: "Manufacturer", Value: strings.TrimSpace(strings.Join(fields[2:], " "))}
		e.Details = append(e.Details, d)
		g.Entries = append(g.Entries, e)
	}
	return &g
}

// LsUSBRun returns inventory of all available USB devices
func LsUSBRun() ([]byte, error) {
	lsusb, err := exec.LookPath("lsusb")
	if err != nil {
		return nil, err
	}

	output, err := exec.Command(lsusb).Output()
	if err != nil {
		out := ""
		if output != nil {
			out = string(output)
		}
		return nil, fmt.Errorf("cannot run lsusb: %v,[%s]", err, out)
	}
	sdevs := strings.Split(string(output), "\n")
	if len(sdevs) < 1 {
		return nil, fmt.Errorf("lsusb: insufficient result")
	}
	result := lsusbResultFormat(sdevs)
	rjson, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return rjson, nil
}
