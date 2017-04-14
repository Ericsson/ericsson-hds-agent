package inventory

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/Ericsson/ericsson-hds-agent/agent/collectors/types"
)

func smbiosResultFormat(lines []string) *types.SMBIOS {
	s := types.SMBIOS{}
	s.Entries = make([]types.Entry, 0)
	e := types.Entry{}
	lineno := 0
	skipping := false
	for _, line := range lines {
		lineno++
		if line == "" {
			continue
		}
		if line == "End Of Table" {
			break
		}
		if strings.HasPrefix(line, "#") ||
			strings.HasPrefix(line, "Table at ") {
			continue
		}
		if line[0] >= '0' && line[0] <= '9' {
			continue
		}
		if strings.HasPrefix(line, "SMBIOS") {
			fields := strings.Fields(line)
			if len(fields) > 1 {
				s.Version = fields[1]
			}
			continue
		}
		if strings.HasPrefix(line, "Handle ") {
			if skipping {
				skipping = false
				continue
			}
			if lineno > 3 && e.Category != "" && e.Details != nil {
				s.Entries = append(s.Entries, e)
			}
			e = types.Entry{}
			e.Details = make([]types.Detail, 0)
			continue
		}
		if skipping {
			continue
		}
		if strings.HasPrefix(line, "OEM-specific Type") {
			skipping = true
			continue
		}
		if line[0] == '\t' && line[1] == '\t' { // skip too much detail
			continue
		}
		if line[0] == '\t' {
			kv := strings.SplitN(line, ":", 2)
			if len(kv) > 1 {
				d := types.Detail{Tag: strings.TrimSpace(kv[0]), Value: strings.TrimSpace(kv[1])}
				e.Details = append(e.Details, d)
			}
		} else {
			e.Category = line
		}
	}
	return &s
}

// SMBIOSRun returns inventory of SMBIOS in []byte, which contains hardware
// components, serial number etc
func SMBIOSRun() ([]byte, error) {
	dmidecode, err := exec.LookPath("dmidecode")
	if err != nil {
		return nil, err
	}

	output, err := exec.Command(dmidecode).Output()
	if err != nil {
		out := ""
		if output != nil {
			out = string(output)
		}
		return nil, fmt.Errorf("cannot run dmidecode, %v %s", err, out)
	}
	lines := strings.Split(string(output), "\n")
	if len(lines) < 3 {
		return nil, fmt.Errorf("dmidecode insufficient result")
	}
	result := smbiosResultFormat(lines)
	rjson, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return rjson, nil
}
