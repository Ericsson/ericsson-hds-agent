package disk

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Ericsson/ericsson-hds-agent/agent/collectors"
	"github.com/Ericsson/ericsson-hds-agent/agent/collectors/types"
	"github.com/Ericsson/ericsson-hds-agent/agent/log"
)

var procDiskstatsColumns = []string{"readsIssued",
	"readsMerged",
	"sectorsRead",
	"timeReading",
	"writesCompleted",
	"writesMerge",
	"sectorsWritten",
	"timeWriting",
	"ioInProgress",
	"timeDoingIO",
	"weightedTimeDoingIO",
}

const (
	blockDrivesDir         = "/sys/block"
	ueventDevtypePrefix    = "DEVTYPE="
	conventionalSectorSize = 512 // /sys/block/<device>/size is the size in 512-byte chunks
)

func loader() ([]byte, error) {
	return ioutil.ReadFile("/proc/diskstats")
}

func formatProcDiskstats(data string, drivesToInclude map[string]bool) (string, string, map[string]string, error) {
	diskColumns := []string{"readsIssued", "readsMerged", "sectorsRead", "timeReading", "writesCompleted", "writesMerge", "sectorsWritten", "timeWriting", "ioInProgress", "timeDoingIO", "weightedTimeDoingIO"}
	metadataProto := map[string]string{
		"readsIssued":         "int Reads completed successfully",
		"readsMerged":         "int reads merged",
		"sectorsRead":         "int sectors read",
		"timeReading":         "int time spent reading (ms)",
		"writesCompleted":     "int writes completed",
		"writesMerge":         "int writes merged",
		"sectorsWritten":      "int sectors written",
		"timeWriting":         "int time spent writing (ms)",
		"ioInProgress":        "int I/Os currently in progress",
		"timeDoingIO":         "int time spent doing I/Os (ms)",
		"weightedTimeDoingIO": "int weighted time spent doing I/Os (ms)s",
	}
	lines := strings.Split(data, "\n")
	headers := make([]string, 0)
	metrics := make([]string, 0)
	metadata := make(map[string]string)

	driveCount := 0
	for _, line := range lines {
		columns := strings.Fields(line)

		if len(columns) < 4 {
			continue
		}

		var driveName = columns[2]
		if drivesToInclude[driveName] {
			driveCount++
			columnCount := 0
			for i, value := range columns[3:] {
				if i >= len(diskColumns) {
					log.Errorf("more columns than expected in /proc/diskstat for drive %s", driveName)
					break
				}

				headers = append(headers, driveName+"."+diskColumns[i])
				metrics = append(metrics, value)
				if v, ok := metadataProto[diskColumns[i]]; ok {
					metadata[driveName+"."+diskColumns[i]] = v
				}
				columnCount++
			}

			if columnCount < len(diskColumns) {
				log.Errorf("less columns than expected in /proc/diskstat for drive %s", driveName)
			}
		}
	}

	if driveCount == 0 {
		drives := []string{}
		for drive := range drivesToInclude {
			drives = append(drives, drive)
		}
		return "", "", metadata, fmt.Errorf("None of the block drives %v were found in /proc/diskstat", drives)
	}

	return strings.Join(headers, " "), strings.Join(metrics, " "), metadata, nil
}

func preformatter(data []byte) ([]*collectors.MetricResult, error) {
	blockDrives, err := getBlockDrives()
	if err != nil {
		return nil, err
	}

	drivesToInclude := make(map[string]bool)
	for i := range blockDrives {
		if blockDrives[i].Removable { //exclude removable media
			continue
		}
		drivesToInclude[blockDrives[i].Name] = true
	}

	if len(drivesToInclude) == 0 {
		return nil, fmt.Errorf("No block drives found to report on")
	}

	header, metric, metadata, err := formatProcDiskstats(string(data), drivesToInclude)
	if err != nil {
		return nil, err
	}

	result := collectors.BuildMetricResult(header, metric, "", metadata)
	return []*collectors.MetricResult{result}, nil
}

// getBlockDrives returns a list of the block drives on the machine as a []BlockDrive
func getBlockDrives() ([]types.BlockDrive, error) {
	var ccissRegex = regexp.MustCompile("^cciss[!/]c[0-9]+d[0-9]+(p[0-9]+)?$")
	drives := make([]types.BlockDrive, 0)

	driveDirs, err := ioutil.ReadDir(blockDrivesDir)
	if err != nil {
		return nil, err
	}

	//walk the block drives in sysfs to collect data on them
	driveExclusion := regexp.MustCompile("^(ram|loop)[0-9]+$")
	for _, fi := range driveDirs {
		if driveExclusion.MatchString(fi.Name()) {
			continue
		}

		//find all nested device dirs and create BlockDrive structs for each one
		driveLink := filepath.Join(blockDrivesDir, fi.Name())
		driveDir, err := filepath.EvalSymlinks(driveLink) // filepath.Walk needs a real dir
		if err != nil {
			log.Errorf("Error evaluating symlinks for %s: %v", driveLink, err)
		}
		filepath.Walk(driveDir, func(path string, info os.FileInfo, err error) error {
			//ignore files
			if !info.IsDir() {
				return nil
			}

			//first, check if dir is a device dir, by checking for a file named 'dev' inside
			childPath := filepath.Join(path, "dev")
			devFileInfo, err := os.Stat(childPath)
			if os.IsNotExist(err) || devFileInfo.IsDir() {
				return filepath.SkipDir
			}

			//now we are confident we have a device dir. Instantiate a BlockDrive and read attributes into it
			driveName := info.Name()
			if ccissRegex.MatchString(driveName) {
				driveName = strings.Replace(driveName, "!", "/", 1)
			}
			drive := types.BlockDrive{
				Name:      driveName,
				SysfsPath: path,
			}

			//read maj:min
			//childPath is still <path>/dev
			fc, err := ioutil.ReadFile(childPath)
			if err == nil {
				drive.MajMin = string(bytes.TrimSpace(fc))
			} else {
				log.Errorf("Error reading %s: %v", childPath, err)
			}
			//read devtype
			childPath = filepath.Join(path, "uevent")
			fc, err = ioutil.ReadFile(childPath)
			if err == nil {
				lines := strings.Split(string(fc), "\n")
				for _, line := range lines {
					if strings.HasPrefix(line, ueventDevtypePrefix) {
						drive.Type = strings.TrimPrefix(line, ueventDevtypePrefix)
						break
					}
				}
			} else if !os.IsNotExist(err) {
				log.Errorf("Error reading %s: %v", childPath, err)
			}
			//read size
			childPath = filepath.Join(path, "size")
			fc, err = ioutil.ReadFile(childPath)
			if err == nil {
				sectors, err := strconv.ParseInt(strings.TrimSpace(string(fc)), 10, 64)
				if err == nil {
					drive.Size = sectors * conventionalSectorSize
				} else {
					log.Errorf("Error parsing %s as integer, from %s: %v", strings.TrimSpace(string(fc)), childPath, err)
				}
			} else if !os.IsNotExist(err) {
				log.Errorf("Error reading %s: %v", childPath, err)
			}
			//read Removable
			childPath = filepath.Join(path, "removable")
			fc, err = ioutil.ReadFile(childPath)
			if err == nil {
				removable, err := strconv.ParseBool(strings.TrimSpace(string(fc)))
				if err == nil {
					drive.Removable = removable
				} else {
					log.Errorf("Error parsing %s as boolean, from %s: %v", strings.TrimSpace(string(fc)), childPath, err)
				}
			} else if !os.IsNotExist(err) {
				log.Errorf("Error reading %s: %v", childPath, err)
			}
			//read ReadOnly
			childPath = filepath.Join(path, "ro")
			fc, err = ioutil.ReadFile(childPath)
			if err == nil {
				readOnly, err := strconv.ParseBool(strings.TrimSpace(string(fc)))
				if err == nil {
					drive.ReadOnly = readOnly
				} else {
					log.Errorf("Error parsing %s as boolean, from %s: %v", strings.TrimSpace(string(fc)), childPath, err)
				}
			} else if !os.IsNotExist(err) {
				log.Errorf("Error reading %s: %v", childPath, err)
			}
			//read Revision
			childPath = filepath.Join(path, "device", "rev")
			fc, err = ioutil.ReadFile(childPath)
			if err == nil {
				drive.Revision = strings.TrimSpace(string(fc))
			} else if !os.IsNotExist(err) {
				log.Errorf("Error reading %s: %v", childPath, err)
			}
			//read Vendor
			childPath = filepath.Join(path, "device", "vendor")
			fc, err = ioutil.ReadFile(childPath)
			if err == nil {
				drive.Vendor = strings.TrimSpace(string(fc))
			} else if !os.IsNotExist(err) {
				log.Errorf("Error reading %s: %v", childPath, err)
			}
			//read Product
			childPath = filepath.Join(path, "device", "model")
			fc, err = ioutil.ReadFile(childPath)
			if err == nil {
				drive.Product = strings.TrimSpace(string(fc))
			} else if !os.IsNotExist(err) {
				log.Errorf("Error reading %s: %v", childPath, err)
			}
			//read LogicalBlockSize
			childPath = filepath.Join(path, "queue", "logical_block_size")
			fc, err = ioutil.ReadFile(childPath)
			if err == nil {
				drive.LogicalBlockSize = strings.TrimSpace(string(fc))
			} else if !os.IsNotExist(err) {
				log.Errorf("Error reading %s: %v", childPath, err)
			}

			//read is hdd or ssd drive type
			childPath = filepath.Join(path, "queue", "rotational")
			fc, err = ioutil.ReadFile(childPath)
			if err == nil {
				if strings.Contains(string(fc), "1") {
					drive.StorageType = "HDD"
				} else {
					drive.StorageType = "SSD"
				}
			} else {
				drive.StorageType = "Unknown"
				if !os.IsNotExist(err) {
					log.Errorf("Error reading %s: %v", childPath, err)

				}
			}
			drives = append(drives, drive)

			return nil
		})
	}

	return drives, nil
}
