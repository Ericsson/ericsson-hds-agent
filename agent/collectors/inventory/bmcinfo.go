package inventory

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/Ericsson/ericsson-hds-agent/agent/collectors/types"
)

// BmcPrecheck validates the dependency of bmc-info and  dmidecode
func BmcPrecheck() error {
	_, err := exec.LookPath("bmc-info")
	if err != nil {
		return err
	}
	dmidecode, err := exec.LookPath("dmidecode")
	if err != nil {
		return nil // we can't check is data exists but we can try to collect it.
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

func bmcInfoResultFormat(lines []string) *types.GenericInfo {
	g := types.GenericInfo{}
	g.Entries = make([]types.Entry, 0)
	e := types.Entry{}
	e.Category = "bmc"
	var d types.Detail
	for _, line := range lines {
		fields := strings.Split(line, ":")
		if len(fields) < 2 {
			continue
		}
		d = types.Detail{Tag: strings.Trim(fields[0], " "), Value: strings.Trim(fields[1], " ")}
		e.Details = append(e.Details, d)
	}
	g.Entries = append(g.Entries, e)
	return &g
}

// BmcInfoRun returns formatted output of bmc-info in []byte
func BmcInfoRun() ([]byte, error) {
	bmcInfo, err := exec.LookPath("bmc-info")
	if err != nil {
		return nil, err
	}
	output, err := exec.Command(bmcInfo).Output()
	if err != nil {
		out := ""
		if output != nil {
			out = string(output)
		}
		return nil, fmt.Errorf("cannot run bmc-info: %v,[%s]", err, out)
	}
	lines := strings.Split(string(output), "\n")
	if len(lines) < 1 {
		return nil, fmt.Errorf("bmc-info: insufficient result")
	}
	result := bmcInfoResultFormat(lines)
	rjson, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return rjson, nil
}
