package inventory

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

import (
	"github.com/Ericsson/ericsson-hds-agent/agent/collectors/types"
	"github.com/Ericsson/ericsson-hds-agent/agent/log"
)

var (
	dpkgPackageFields = []string{"Version", "Size"}
)

func dpkgResultFormat(lines []string) *types.GenericInfo {
	g := types.GenericInfo{}
	g.Entries = make([]types.Entry, 0)
	var d types.Detail
	for _, line := range lines {
		columns := strings.Fields(line)
		log.Infof("fields: %v", columns)

		if len(columns) < 2 {
			continue
		}
		e := types.Entry{}
		e.Category = strings.Trim(columns[0], " ")
		for i, value := range columns[1:] {
			d = types.Detail{Tag: strings.Trim(packageFields[i], " "), Value: strings.Trim(value, " ")}
			e.Details = append(e.Details, d)
		}
		g.Entries = append(g.Entries, e)
	}
	return &g
}

// DpkgCollectRun returns formatted output of dpkg-query in []byte
func DpkgCollectRun() ([]byte, error) {
	dpkgQuery, err := exec.LookPath("dpkg-query")
	if err != nil {
		return nil, err
	}

	output, err := exec.Command(dpkgQuery, "--show", "--showformat=${Package}\t${Version}\t${Installed-Size}\n").Output()
	if err != nil {
		out := ""
		if output != nil {
			out = string(output)
		}
		return nil, fmt.Errorf("cannot run dpkg: %v,[%s]", err, out)
	}

	lines := strings.Split(string(output), "\n")
	result := dpkgResultFormat(lines)
	rjson, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return rjson, nil
}
