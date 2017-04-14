package inventory

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/Ericsson/ericsson-hds-agent/agent/collectors/types"
	"github.com/Ericsson/ericsson-hds-agent/agent/log"
)

func lspciResultFormat(lines []string) *types.GenericInfo {
	g := types.GenericInfo{}
	g.Entries = make([]types.Entry, 0)
	for _, line := range lines {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		log.Infof("fields: %v", fields)
		e := types.Entry{}
		d := types.Detail{Tag: "Slot", Value: fields[0]}
		e.Details = append(e.Details, d)
		d = types.Detail{}
		d.Tag = "Class"
		for i := 1; i < len(fields); i++ {
			d.Value += fields[i]
			if strings.HasSuffix(fields[i], ":") {
				d.Value = d.Value[:len(d.Value)-1]
				break
			}
		}
		e.Details = append(e.Details, d)
		d = types.Detail{Tag: "Description", Value: strings.Join(fields[2:], " ")}
		e.Details = append(e.Details, d)
		e.Category = "PCIInfo"
		g.Entries = append(g.Entries, e)
	}
	return &g
}

// LsPCIRun returns inventory of all available PCI devices
func LsPCIRun() ([]byte, error) {
	lspci, err := exec.LookPath("lspci")
	if err != nil {
		return nil, err
	}

	output, err := exec.Command(lspci).Output()
	if err != nil {
		out := ""
		if output != nil {
			out = string(output)
		}
		return nil, fmt.Errorf("cannot run lspci: %v,[%s]", err, out)
	}
	lines := strings.Split(string(output), "\n")
	if len(lines) < 1 {
		return nil, fmt.Errorf("lspci: insufficient result")
	}
	result := lspciResultFormat(lines)
	rjson, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return rjson, nil
}
