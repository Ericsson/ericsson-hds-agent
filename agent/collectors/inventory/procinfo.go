package inventory

import (
	"encoding/json"
	"io/ioutil"
	"strings"

	"github.com/Ericsson/ericsson-hds-agent/agent/collectors/types"
	"github.com/Ericsson/ericsson-hds-agent/agent/log"
)

const tagAll = ":all"

func filterNames(names []string) []string {
	if names[0] == "all" || names[0] == tagAll {
		names = []string{"version", "partitions", "cpuinfo", "hostname"}
	}
	return names
}

func procInfoReadStructured(names []string) (*types.GenericInfo, error) {
	g := types.GenericInfo{}
	g.Entries = make([]types.Entry, 0)
	//TODO: diskstats, stat, filesystems, mdstat, ...
	names = filterNames(names)
	for _, name := range names {
		switch name {
		case "hostname":
			fc, err := ioutil.ReadFile("/proc/sys/kernel/hostname")
			if err != nil {
				log.Errorf("can't read /proc/sys/kernel/hostname, %v", err)
				return nil, err
			}
			e := types.Entry{}
			e.Category = "kernel"
			d := types.Detail{Tag: "hostname", Value: strings.TrimSpace(string(fc))}
			e.Details = append(e.Details, d)
			g.Entries = append(g.Entries, e)
		case "partitions":
			fc, err := ioutil.ReadFile("/proc/partitions")
			if err != nil {
				log.Errorf("can't read /proc/partitions, %v", err)
				return nil, err
			}
			e := types.Entry{}
			e.Category = "partitions"
			lines := strings.Split(string(fc), "\n")
			lineno := 0
			for _, line := range lines {
				lineno++
				if lineno < 3 {
					continue
				}
				if line == "" {
					continue
				}
				fields := strings.Fields(line)
				if len(fields) > 3 {
					d := types.Detail{Tag: "Disk Device", Value: strings.TrimSpace(fields[3])}
					e.Details = append(e.Details, d)
					d = types.Detail{Tag: "Number of Blocks", Value: strings.TrimSpace(fields[2])}
					e.Details = append(e.Details, d)
				}
			}
			g.Entries = append(g.Entries, e)
		case "version":
			fc, err := ioutil.ReadFile("/proc/version")
			if err != nil {
				log.Errorf("can't read /proc/version, %v", err)
				return nil, err
			}
			e := types.Entry{Category: "OSVersion"}
			fields := strings.Fields(string(fc))
			if len(fields) > 2 {
				d := types.Detail{Tag: fields[0] + " " + fields[1], Value: strings.Join(fields[2:], " ")}
				e.Details = append(e.Details, d)
				g.Entries = append(g.Entries, e)
			}
		case "cpuinfo":
			e := types.Entry{}
			fc, err := ioutil.ReadFile("/proc/cpuinfo")
			if err != nil {
				log.Errorf("can't read /proc/cpuinfo, %v", err)
				return nil, err
			}
			lines := strings.Split(string(fc), "\n")
			lineno := 0
			for _, line := range lines {
				lineno++
				if line == "" {
					continue
				}
				if strings.HasPrefix(line, "processor") {
					if lineno > 1 {
						g.Entries = append(g.Entries, e)
					}
					e = types.Entry{}
					e.Category = "cpuinfo"
				}
				fields := strings.SplitN(line, ":", 2)
				if len(fields) > 1 {
					tag := strings.TrimSpace(fields[0])

					// Skip reporting these values to avoid flickering due to
					// CPU frequency scaling and generating a lot of data
					if tag == "cpu MHz" || tag == "bogomips" {
						continue
					}

					d := types.Detail{Tag: tag, Value: strings.TrimSpace(fields[1])}
					e.Details = append(e.Details, d)
				}
			}
			if len(e.Details) > 0 {
				g.Entries = append(g.Entries, e)
			}
		default:
			log.Errorf("/proc/%s unsupported", name)
		}
	}
	return &g, nil
}

func convertToMetric(result *types.GenericInfo) (string, error) {
	return "", nil // TODO!!!
}

// ProcInfoRun returns inventory of host version, cpuinfo etc
func ProcInfoRun() ([]byte, error) {
	which := []string{"version", "partitions", "cpuinfo", "hostname"}
	result, err := procInfoReadStructured(which)
	if err != nil {
		return nil, err
	}
	rjson, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return rjson, nil
}
