package inventory

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/Ericsson/ericsson-hds-agent/agent/collectors/types"
	"github.com/Ericsson/ericsson-hds-agent/agent/log"
)

var (
	packageFields = []string{"Version", "Size"}
)

func rpmResultFormat(lines []string) *types.GenericInfo {
	g := types.GenericInfo{}
	g.Entries = make([]types.Entry, 0)
	var d types.Detail
	for _, line := range lines {
		e := types.Entry{}
		columns := strings.Fields(line)
		if len(columns) < 2 {
			continue
		}
		e.Category = strings.Trim(columns[0], " ")
		log.Infof("fields: %v", columns)
		for i, value := range columns[1:] {
			d = types.Detail{Tag: strings.Trim(packageFields[i], " "), Value: strings.Trim(value, " ")}
			e.Details = append(e.Details, d)
		}
		g.Entries = append(g.Entries, e)
	}
	return &g
}

// RpmCollectRun returns inventory of all installed rpm in []byte
func RpmCollectRun() ([]byte, error) {
	rpm, err := exec.LookPath("rpm")
	if err != nil {
		return nil, err
	}

	output, err := exec.Command(rpm, "-qa", "--qf", `%{NAME}\t%{VERSION}\t%{SIZE}\n`).Output()
	if err != nil {
		out := ""
		if output != nil {
			out = string(output)
		}
		return nil, fmt.Errorf("cannot run rpm: %v,[%s]", err, out)
	}

	lines := strings.Split(string(output), "\n")
	result := rpmResultFormat(lines)
	rjson, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return rjson, nil
}
