package diskusage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/Ericsson/ericsson-hds-agent/agent/collectors"
	"github.com/Ericsson/ericsson-hds-agent/agent/collectors/types"
	"github.com/Ericsson/ericsson-hds-agent/agent/log"
)

func loader() ([]byte, error) {

	df, err := exec.LookPath("df")
	var usageStats []*types.MountUsageStat
	if err != nil {
		usageStats, err = collectUsages()
	} else {
		usageStats, err = collectUsagesDF(df)
	}

	if len(usageStats) == 0 {
		log.Errorf("No usage information found")
		return nil, errors.New("No usage information found")
	}

	return json.Marshal(usageStats)
}

func preformatter(data []byte) ([]*collectors.MetricResult, error) {
	var usageStats = make([]*types.MountUsageStat, 0)
	err := json.Unmarshal(data, &usageStats)
	if err != nil {
		log.Error(err.Error())
		return nil, err
	}
	var metadata = make(map[string]string)
	var headers = make([]string, 0)
	var metrics = make([]string, 0)
	//for each usageStat, add metric headers and values
	for _, usageStat := range usageStats {
		colPrefix := strings.TrimPrefix(usageStat.Mount.DevicePath, "/dev/") //trim '/dev/' if we can
		headers = append(headers, colPrefix+".BytesTotal", colPrefix+".BytesUsed", colPrefix+".BytesAvailable", colPrefix+".InodesUsed", colPrefix+".InodesFree")
		metrics = append(metrics, strconv.FormatUint(usageStat.Total, 10), strconv.FormatUint(usageStat.Used, 10), strconv.FormatUint(usageStat.Available, 10), strconv.FormatUint(usageStat.InodesUsed, 10), strconv.FormatUint(usageStat.InodesFree, 10))

		metadata[colPrefix+".BytesTotal"] = "int Total number of bytes."
		metadata[colPrefix+".BytesUsed"] = "int Number of used bytes."
		metadata[colPrefix+".BytesAvailable"] = "int Number of available bytes."
		metadata[colPrefix+".InodesUsed"] = "int Number of used inodes."
		metadata[colPrefix+".InodesFree"] = "int Number of available inodes."
	}
	result := collectors.BuildMetricResult(strings.Join(headers, " "), strings.Join(metrics, " "), "", metadata)
	return []*collectors.MetricResult{result}, nil
}

//Collect usages with DF
func collectUsagesDF(df string) ([]*types.MountUsageStat, error) {

	output, err := exec.Command(df, "--output=size,used,avail,iused,iavail,source", "-BK", "-a").Output()
	if output == nil || len(output) == 0 {
		return nil, fmt.Errorf("cannot run df util: %v", err)
	}
	return parseDfOutput(output)
}

//Collect usages without DF util
func collectUsages() ([]*types.MountUsageStat, error) {
	var usageStats = make([]*types.MountUsageStat, 0)
	mounts := getMounts()
	if len(mounts) == 0 {
		return nil, fmt.Errorf("No mounted block drives found")
	}

	for _, mount := range mounts {
		usageStat, err := getMountUsage(mount)
		if err != nil {
			log.Errorf("Error getting disk usage for mount at %s: %v", mount.Mountpoint, err)
			continue
		}
		usageStats = append(usageStats, usageStat)
	}
	return usageStats, nil
}

func parseDfOutput(output []byte) ([]*types.MountUsageStat, error) {
	line := ""
	var err error
	mountsUsage := make(map[string]*types.MountUsageStat)
	parseIntFiled := func(str string) (uint64, error) {
		var multiple uint64
		if strings.HasSuffix(str, "K") {
			multiple = 1024
			str = strings.TrimSuffix(str, "K")
		} else {
			multiple = 1
		}
		if str == "-" {
			str = "0"
		}
		res, err := strconv.ParseInt(str, 10, 64)
		if err != nil {
			log.Errorf("Could not parse line: %s, error: %v", line, err)
			return 0, err
		}
		return uint64(res) * multiple, nil
	}

	var usageStats = make([]*types.MountUsageStat, 0)
	out := string(output)

	lines := strings.Split(out, "\n")
	for _, line = range lines {
		if strings.Contains(line, "1K-blocks") {
			continue //header line
		}

		columns := strings.Fields(line)
		if len(columns) != 6 {
			if len(line) > 0 {
				log.Info("Unexpected line: " + line)
			}
			continue //error line
		}

		usage := &types.MountUsageStat{}
		usage.Total, err = parseIntFiled(columns[0])
		if err != nil {
			continue
		}

		usage.Used, err = parseIntFiled(columns[1])
		if err != nil {
			continue
		}

		usage.Available, err = parseIntFiled(columns[2])
		if err != nil {
			continue
		}
		usage.InodesUsed, err = parseIntFiled(columns[3])
		if err != nil {
			continue
		}
		usage.InodesFree, err = parseIntFiled(columns[4])
		if err != nil {
			continue
		}
		usage.Mount = &types.MountStat{DevicePath: columns[5]}
		mountsUsage[usage.Mount.DevicePath] = usage
	}

	devPaths := make([]string, 0)
	for devPath := range mountsUsage {
		devPaths = append(devPaths, devPath)
	}
	sort.Strings(devPaths)
	for _, devPath := range devPaths {
		usageStats = append(usageStats, mountsUsage[devPath])
	}

	return usageStats, nil
}

// getMounts returns a list of mounts(disk, path, etc) using both mount(8) and /proc/mount
func getMounts() []*types.MountStat {
	mounts := make(map[string]*types.MountStat)

	//read mount(8) output
	out, err := exec.Command("mount").Output()
	if err != nil {
		log.Infof("Error running mount program: %v", err)
	} else {
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			fields := strings.Split(line, " ")
			if len(fields) < 6 {
				continue
			}
			devPath := fields[0]
			if err != nil {
				continue
			}
			mountpoint := fields[2]
			fstype := fields[4]
			options := fields[5]
			//trim parens around the options string
			if len(options) >= 2 && options[0] == '(' && options[len(options)-1] == ')' {
				options = options[1 : len(options)-1]
			}
			mounts[devPath] = &types.MountStat{
				DevicePath:     devPath,
				Mountpoint:     mountpoint,
				FilesystemType: fstype,
				Options:        options,
			}
		}
	}

	//read /proc/mounts
	fc, err := ioutil.ReadFile("/proc/mounts")
	if err != nil {
		log.Errorf("Error reading /proc/mounts: %v", err)
	} else {
		lines := strings.Split(string(fc), "\n")
		for _, line := range lines {
			fields := strings.Split(line, " ")
			if len(fields) < 4 {
				continue
			}

			devPath, err := filepath.EvalSymlinks(fields[0])
			if err != nil {
				continue
			}
			mountpoint := fields[1]
			fstype := fields[2]
			options := fields[3]
			mounts[devPath] = &types.MountStat{
				DevicePath:     devPath,
				Mountpoint:     mountpoint,
				FilesystemType: fstype,
				Options:        options,
			}
		}
	}

	//return a slice of the Mounts sorted by the device path
	mountList := make([]*types.MountStat, 0)
	devPaths := make([]string, 0)
	for devPath := range mounts {
		devPaths = append(devPaths, devPath)
	}
	sort.Strings(devPaths)
	for _, devPath := range devPaths {
		mountList = append(mountList, mounts[devPath])
	}
	return mountList
}

// getMountUsage returns statstics of mounted device
func getMountUsage(mount *types.MountStat) (*types.MountUsageStat, error) {
	stat := syscall.Statfs_t{}
	err := syscall.Statfs(mount.Mountpoint, &stat)
	if err != nil {
		return nil, err
	}
	bsize := stat.Bsize
	diskUsage := &types.MountUsageStat{
		Mount: mount,
		//XXX get the filesystem according to stat?
		Total:       (uint64(stat.Blocks) * uint64(bsize)),
		Free:        (uint64(stat.Bfree) * uint64(bsize)),
		Available:   (uint64(stat.Bavail) * uint64(bsize)),
		InodesTotal: (uint64(stat.Files)),
		InodesFree:  (uint64(stat.Ffree)),
	}
	diskUsage.Used = diskUsage.Total - diskUsage.Free
	diskUsage.Unavailable = diskUsage.Total - diskUsage.Available
	diskUsage.InodesUsed = diskUsage.InodesTotal - diskUsage.InodesFree

	return diskUsage, nil
}
