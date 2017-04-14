package smart

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Ericsson/ericsson-hds-agent/agent/collectors"
	"github.com/Ericsson/ericsson-hds-agent/agent/collectors/types"
	"github.com/Ericsson/ericsson-hds-agent/agent/log"
)

const smartDataBegin = "=== START OF READ SMART DATA SECTION ==="

func loader() ([]byte, error) {
	return []byte(""), nil
}

// Precheck validates for presence of dependency smartctl
func Precheck() error {
	if _, err := exec.LookPath("smartctl"); err != nil {
		return err
	}
	//all errors returned by SMARTRun are fatal for the precheck. So rather than duplicate code here, just run it once and check
	_, err := preformatter(nil)
	return err
}

//Process only sata type drive
func formatSmartSATA(data string, disk types.Disk) ([]string, []string, map[string]string) {
	var headers = make([]string, 0)
	var metrics = make([]string, 0)
	var metadata = make(map[string]string)
	//parse the smartctl output, get the raw and normalized values of all smart stats
	smartLines := strings.Split(data, "\n")
	metricsMap := make(map[string]smartStat)
	inSMARTDataSection := false
	isDataTable := false
	for _, smartLine := range smartLines {
		if !inSMARTDataSection {
			if strings.HasPrefix(smartLine, "SMART Disabled") {
				log.Infof("Smart disabled for disk %s [%s]", disk.Path, disk.Type)
				return nil, nil, metadata
			}
			if smartLine == smartDataBegin {
				inSMARTDataSection = true
			}

			continue
		}
		parts := regexp.MustCompile(`\s+`).Split(strings.TrimSpace(string(smartLine)), -1)
		//look for data lines as having 10 or more fields; excluding the header line which starts with "ID#"
		if parts[0] == "ID#" {
			isDataTable = true
			continue
		}
		if len(parts) < 10 && isDataTable {
			isDataTable = false
			break
		}
		if isDataTable && len(parts) >= 10 {
			metricsMap[parts[0]] = smartStat{normalized: parts[3], name: parts[1], raw: strings.Join(parts[9:], "_")}
		}
	}
	var diskID = gentrateDiskID(disk)

	//if no smart stats were available for this drive, skip it
	if len(metricsMap) == 0 {
		log.Infof("No SMART stats available for drive: %s [%s]", disk.Path, disk.Type)
		return nil, nil, metadata
	}

	//order columns by metric number
	var smartKeys = make([]int, len(metricsMap))
	i := 0
	for key := range metricsMap {
		var err error
		smartKeys[i], err = strconv.Atoi(key)
		if err != nil {
			log.Infof("Encountered SMART stat key %q that could not convert to int", key)
			continue
		}
		i++
	}
	sort.Ints(smartKeys)
	for _, smartKey := range smartKeys {
		var smartKeyStr = strconv.Itoa(smartKey)
		headers = append(headers, diskID+".smart"+smartKeyStr+"normalized")
		headers = append(headers, diskID+".smart"+smartKeyStr+"raw")
		metrics = append(metrics, metricsMap[smartKeyStr].normalized)
		metrics = append(metrics, metricsMap[smartKeyStr].raw)
		metadata[diskID+".smart"+smartKeyStr+"raw"] = "string " + metricsMap[smartKeyStr].name
		metadata[diskID+".smart"+smartKeyStr+"normalized"] = "int " + metricsMap[smartKeyStr].name
	}

	return headers, metrics, metadata
}

func gentrateDiskID(disk types.Disk) string {
	//set a disk ID for use in the column names
	var diskID = disk.Name
	//handle special cases where the type is needed
	if strings.Contains(disk.Type, "megaraid") || strings.Contains(disk.Type, "cciss") { //todo anoter types of raid
		diskID = disk.Path + "." + disk.Type
		diskID = strings.Replace(diskID, ",", "_", -1)
	}
	return diskID
}

//Process only sas type drive
func formatSmartSAS(data string, disk types.Disk) ([]string, []string, map[string]string) {
	headerMap := map[string]string{
		"PowerUpHrs":         "float Accumulated power on time",
		"Temperature":        "string Current Drive Temp",
		"StartStopCycles":    "int Accumulated start-stop cycles",
		"LoadUnloadCycles":   "int Accumulated load-unload cycles",
		"ReadCEFast":         "int Read: Errors corrected by ECC, fast",
		"ReadCEDelayed":      "int Read: Errors corrected by ECC, delayed",
		"ReadCERereads":      "int Read: Errors corrected by rereads",
		"ReadTotalCE":        "int Read: Total Errors corrected",
		"ReadCAInvocations":  "int Read: Correction algorithm invocations",
		"ReadProcessedGb":    "int Read: Gigabytes processed [10^9 bytes]",
		"ReadTotalUE":        "int Read: Total Uncorrected Errors",
		"WriteCEFast":        "int Write: Errors corrected by ECC, fast",
		"WriteCEDelayed":     "int Write: Errors corrected by ECC, delayed",
		"WriteCERereads":     "int Write: Errors corrected by rereads",
		"WriteTotalCE":       "int Write: Total Errors corrected",
		"WriteCAInvocations": "int Write: Correction algorithm invocations",
		"WriteProcessedGb":   "int Write: Gigabytes processed [10^9 bytes]",
		"WriteTotalUE":       "int Write: Total Uncorrected Errors",
	}
	var headers = make([]string, 0)
	var metrics = make([]string, 0)
	var metadata = make(map[string]string)
	metricsMap := make(map[string]string)
	dataSection := strings.Split(data, smartDataBegin)
	if len(dataSection) < 2 {
		return headers, metrics, metadata
	}
	section := strings.Split(dataSection[1], "Error counter log")

	processSASmartData(section[0], metricsMap)
	if len(section) > 1 {
		processSASECCTable(section[1], metricsMap)
	}
	var diskID = gentrateDiskID(disk)

	var smartKeys = make([]string, 0)
	for key := range metricsMap {
		smartKeys = append(smartKeys, key)
	}
	sort.Strings(smartKeys)
	for _, smartKey := range smartKeys {
		headers = append(headers, diskID+"."+smartKey)
		metrics = append(metrics, metricsMap[smartKey])
		if meta, ok := headerMap[smartKey]; ok {
			metadata[diskID+"."+smartKey] = meta
		}
	}
	return headers, metrics, metadata
}

func processSASmartData(data string, metricsMap map[string]string) {

	smartLines := strings.Split(data, "\n")
	metricMaskMap := []string{
		"number of hours powered up",
		"Current Drive Temp",
		"Accumulated start-stop cycles",
		"Accumulated load-unload cycles",
	}
	metricHeaderMap := map[string]string{
		"number of hours powered up":     "PowerUpHrs",
		"Current Drive Temp":             "Temperature",
		"Accumulated start-stop cycles":  "StartStopCycles",
		"Accumulated load-unload cycles": "LoadUnloadCycles",
	}

	for _, line := range smartLines {
		for _, name := range metricMaskMap {
			if strings.Contains(line, name) {
				if strings.Contains(line, ":") {
					value := strings.Split(line, ":")[1]
					value = strings.Replace(value, " ", "", -1)
					key := metricHeaderMap[name]
					metricsMap[key] = value
					break
				}
				if strings.Contains(line, "=") {
					value := strings.Split(line, "=")[1]
					value = strings.Replace(value, " ", "", -1)
					key := metricHeaderMap[name]
					metricsMap[key] = value
					break
				}
			}
		}

	}
}

// ecc table
func processSASECCTable(data string, metricsMap map[string]string) {

	readErroLine := func(prefix, line string) {
		values := strings.Fields(line)
		if len(values) > 1 {
			metricsMap[prefix+"CEFast"] = values[1]
		}
		if len(values) > 2 {
			metricsMap[prefix+"CEDelayed"] = values[2]
		}
		if len(values) > 3 {
			metricsMap[prefix+"CERereads"] = values[3]
		}
		if len(values) > 4 {
			metricsMap[prefix+"TotalCE"] = values[4]
		}
		if len(values) > 5 {
			metricsMap[prefix+"CAInvocations"] = values[5]
		}
		if len(values) > 6 {
			metricsMap[prefix+"ProcessedGb"] = values[6]
		}
		if len(values) > 7 {
			metricsMap[prefix+"TotalUE"] = values[7]
		}
	}

	smartLines := strings.Split(data, "\n")
	for _, line := range smartLines {
		if strings.Contains(line, "SMART Self-test log") {
			break
		}
		if strings.Contains(line, "read:") {
			readErroLine("Read", line)
			continue
		}
		if strings.Contains(line, "write:") {
			readErroLine("Write", line)
			continue
		}
	}

}

//Check is drive SAS or not
func isSAS(data string) bool {
	smartLines := strings.Split(data, "\n")
	for _, s := range smartLines {
		if strings.Contains(s, "Transport protocol:") && strings.Contains(s, "SAS") {
			return true
		}
	}
	return false
}

func preformatter(d []byte) ([]*collectors.MetricResult, error) {

	unknowCount := 0
	sasResult := &collectors.MetricResult{
		Sufix:    "-sas",
		Metadata: map[string]string{},
	}
	ataResult := &collectors.MetricResult{
		Sufix:    "-ata",
		Metadata: map[string]string{},
	}
	smartctlPath, err := exec.LookPath("smartctl")
	if err != nil {
		return nil, err
	}

	disks, err := getDisks(smartctlPath)
	if err != nil {
		return nil, err
	}

	HPSmartArray := isHPSmartArray()

	processHPSmartArray := func(disk types.Disk) {
		for i := 0; true; i++ {
			out := ""
			insideDisk := types.Disk{Path: disk.Path, Name: disk.Name, Type: "cciss," + strconv.Itoa(i)}
			sdata, _ := exec.Command(smartctlPath, "-d", insideDisk.Type, "-Aa", insideDisk.Path).Output()

			if sdata == nil {
				break
			}

			out = string(sdata)
			if !strings.Contains(out, "=== START OF INFORMATION SECTION ===") {
				break
			}

			_, unknowCount = getDiskSerial(string(sdata), unknowCount)

			if isSAS(out) {
				diskHeaders, diskMetrics, diskMetadata := formatSmartSAS(out, insideDisk)
				if len(diskHeaders) > 0 && len(diskMetrics) > 0 {
					sasResult.Header += " " + strings.Join(diskHeaders, " ")
					sasResult.Data += " " + strings.Join(diskMetrics, " ")
					for k, v := range diskMetadata {
						sasResult.Metadata[k] = v
					}
					continue
				}
			} else {
				diskHeaders, diskMetrics, diskMetadata := formatSmartSATA(out, insideDisk)
				if len(diskHeaders) > 0 && len(diskMetrics) > 0 {
					ataResult.Header += " " + strings.Join(diskHeaders, " ")
					ataResult.Data += " " + strings.Join(diskMetrics, " ")
					for k, v := range diskMetadata {
						ataResult.Metadata[k] = v
					}
					continue
				}
			}

			break
		}

	}
	// For each sd disk, collect its information:
	for _, disk := range disks {
		smartData, err := exec.Command(smartctlPath, "-d", disk.Type, "-Aa", disk.Path).Output()
		if err != nil {
			out := ""
			if smartData != nil {
				out = string(smartData)
			}
			log.Infof("Error running `smartctl -Aa %s`: %v,[%s]", disk.Path, err, out)
			// Ignore for now: 	http://linux.die.net/man/8/smartctl -- Return Values of smartctl
		}
		data := string(smartData)

		if HPSmartArray && isLogicalValue(data) {
			processHPSmartArray(disk)
			continue
		}

		_, unknowCount = getDiskSerial(data, unknowCount)
		if isSAS(data) {
			diskHeaders, diskMetrics, diskMetadata := formatSmartSAS(data, disk)
			if len(diskHeaders) > 0 && len(diskMetrics) > 0 {
				sasResult.Header += " " + strings.Join(diskHeaders, " ")
				sasResult.Data += " " + strings.Join(diskMetrics, " ")
				for k, v := range diskMetadata {
					sasResult.Metadata[k] = v
				}
			}
		} else {
			diskHeaders, diskMetrics, diskMetadata := formatSmartSATA(data, disk)
			if len(diskHeaders) > 0 && len(diskMetrics) > 0 {
				ataResult.Header += " " + strings.Join(diskHeaders, " ")
				ataResult.Data += " " + strings.Join(diskMetrics, " ")
				for k, v := range diskMetadata {
					ataResult.Metadata[k] = v
				}
			}
		}
	}

	metricResult := []*collectors.MetricResult{}
	if len(sasResult.Header) > 1 {
		metricResult = append(metricResult, sasResult)
	}
	if len(ataResult.Header) > 1 {
		metricResult = append(metricResult, ataResult)
	}

	if len(metricResult) == 0 {
		return nil, errors.New("no SMART stats available on any drives")
	}
	return metricResult, nil
}

//Check is HP Smart array
func isHPSmartArray() bool {
	lspci, err := exec.LookPath("lspci")
	if err != nil {
		return false
	}

	lspciData, _ := exec.Command(lspci).Output()
	if lspciData != nil {
		out := string(lspciData)
		return strings.Contains(out, "Hewlett-Packard Company Smart Array")
	}

	return false
}

//CHeck HP Smart Array Logical value
func isLogicalValue(data string) bool {
	smartLines := strings.Split(data, "\n")
	for _, s := range smartLines {
		if strings.Contains(s, "Product:") && strings.Contains(s, "LOGICAL VOLUME") {
			return true
		}
	}
	return false
}

func getDiskSerial(data string, unknowCount int) (string, int) {
	smartLines := strings.Split(data, "\n")
	for _, s := range smartLines {
		if second := getSecondPart(s, "serial number:", ":"); second != "none" {
			return second, unknowCount
		}
	}
	unknowCount++
	return "unknow" + strconv.Itoa(unknowCount), unknowCount
}

func getSecondPart(line, key, delimeter string) string {
	lowStr := strings.ToLower(line)
	lowKey := strings.ToLower(key)
	if strings.Contains(lowStr, lowKey) {
		values := strings.Split(line, delimeter)
		if len(values) > 1 {
			value := values[1]
			return strings.Replace(value, " ", "", -1)
		}
	}
	return "none"
}

// getDisks returns list of disks on the machine using smartctl --scan-open
// note that this only checks block drives /dev/sd[a-z], /dev/sd[a-c][a-z], /dev/hd[a-z], and /dev/discs/disc*
func getDisks(smartctlPath string) ([]types.Disk, error) {
	// Get a list of sd disks:
	dev, devErr := exec.Command(smartctlPath, "--scan-open").Output()
	if devErr != nil {
		devOut := "command did not generate any output"
		if dev != nil {
			devOut = string(dev)
		}
		return nil, fmt.Errorf("cannot run smartctl: %v, [%s]", devErr, devOut)
	}

	devLines := strings.Split(string(dev), "\n")
	l := len(devLines)

	// Removes the last line if it is an empty line:
	if l > 0 && strings.TrimSpace(devLines[l-1]) == "" {
		devLines = devLines[:l-1]
	}

	// Checks if the host machine has sd disks:
	// Also looks for "glob" in the message, which is indicative of an error message that has slipped past the check for error.
	if l < 1 || strings.Contains(devLines[0], "glob") {
		return nil, fmt.Errorf("no disks found")
	}

	var disks = make([]types.Disk, 0)
	// For each sd disk, collect its information:
	for _, devLine := range devLines {
		if strings.HasPrefix(devLine, "#") {
			continue
		}
		parts := strings.Split(devLine, " ")
		if len(parts) < 3 {
			log.Errorf("Unexpected smartcl --scan device line: %s", devLine)
			continue
		}

		disks = append(disks, types.Disk{Path: parts[0], Name: filepath.Base(parts[0]), Type: parts[2]})
	}

	if len(disks) < 0 {
		return nil, fmt.Errorf("no disks found")
	}

	return disks, nil
}
