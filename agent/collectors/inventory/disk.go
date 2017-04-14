package inventory

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

import (
	"path/filepath"

	"bytes"
	"io/ioutil"
	"os"
	"reflect"
	"regexp"
	"strconv"

	"github.com/Ericsson/ericsson-hds-agent/agent/collectors/types"
	"github.com/Ericsson/ericsson-hds-agent/agent/log"
)

const (
	blockDrivesDir         = "/sys/block"
	ueventDevtypePrefix    = "DEVTYPE="
	conventionalSectorSize = 512 // /sys/block/<device>/size is the size in 512-byte chunks (confirmed this by looking at the sysfs source code. It gets the numbers of bytes and divides it by 512 (shifts by 9) -- irrespective of the drive's logical or physical sector size)
)

var ccissRegex = regexp.MustCompile("^cciss[!/]c[0-9]+d[0-9]+(p[0-9]+)?$")

//get a list of the block drives on the machine as a []BlockDrive
func getBlockDrives() ([]BlockDrive, error) {
	drives := make([]BlockDrive, 0)

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
			drive := BlockDrive{
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

// Formats output from smartctl:
func smartctlResultFormat(entryCategory string, diskInfo []string) *types.Entry {
	e := types.Entry{}
	e.Category = entryCategory
	e.Details = make([]types.Detail, 0)
	var d types.Detail
	var index int

	for _, line := range diskInfo {
		// Lines ignored:
		if strings.TrimSpace(line) == "" || strings.Contains(line, "Copyright (C)") || strings.Contains(line, ">> Terminate command early") || strings.Contains(line, "A mandatory SMART command failed:") || strings.Contains(line, "Local Time is:") {
			continue
		}

		// Only keep information from lines with ":":
		if strings.Contains(line, ":") {
			index = strings.Index(line, ":")
			d.Tag = strings.TrimSpace(line[:index])
			d.Value = strings.TrimSpace(line[index+1:])
			e.Details = append(e.Details, d)
		} else {
			continue
		}
	}

	return &e
}

func blockDriveFormat(drive BlockDrive) *types.Entry {
	e := types.Entry{}
	e.Category = "/dev/" + drive.Name
	e.Details = make([]types.Detail, 0)

	driveVal := reflect.ValueOf(drive)
	driveType := reflect.TypeOf(drive)
	for i := 0; i < driveType.NumField(); i++ {
		fieldName := driveType.Field(i).Name
		fieldValue := fmt.Sprintf("%v", driveVal.FieldByName(fieldName).Interface())
		if fieldValue != "" {
			e.Details = append(e.Details, types.Detail{Tag: fieldName, Value: fieldValue})
		}
	}

	return &e
}

//gets list of disks on the machine using smartctl --scan-open
//note that this only checks block drives /dev/sd[a-z], /dev/sd[a-c][a-z], /dev/hd[a-z], and /dev/discs/disc*
func getDisks(smartctlPath string) ([]Disk, error) {
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

	var disks = make([]Disk, 0)
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

		disks = append(disks, Disk{Path: parts[0], Name: filepath.Base(parts[0]), Type: parts[2]})
	}

	if len(disks) < 0 {
		return nil, fmt.Errorf("no disks found")
	}

	return disks, nil
}

//collect Details of SCSI info related to drive
//driveName is e.g. "sda"
func getSCSIDeviceInfo(driveName string) (deviceDetails []types.Detail) {
	var scsiEndDeviceAttributesToCollect = map[string]string{
		"sas_address":       "SAS Address",
		"sas_device_handle": "SAS Device Handle",
	}
	deviceDetails = make([]types.Detail, 0)

	//get device attributes from the device directory
	deviceDir, err := filepath.EvalSymlinks(filepath.Join("/sys/block/", driveName, "/device/"))
	if err != nil {
		log.Errorf("error reading device information for drive %s: %v", driveName, err)
		return deviceDetails
	}

	for filename, label := range scsiEndDeviceAttributesToCollect {
		path := filepath.Join(deviceDir, filename)
		fc, err := ioutil.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		} else if err != nil {
			log.Errorf("Error opening %s: %v", path, err)
		}
		deviceDetails = append(deviceDetails, types.Detail{Tag: label, Value: strings.TrimSpace(string(fc))})
	}

	//find the SCSI host by looking up the directory ancestor chain of the block drive dir for one containing "scsi_host"
	ancestorDir := filepath.Dir(deviceDir)
	for {
		if match, _ := filepath.Match("/sys/devices/", ancestorDir); match || ancestorDir == filepath.Dir(ancestorDir) { // we've gone too far
			break
		}

		hostGlob, err := filepath.Glob(filepath.Join(ancestorDir, "/scsi_host/*"))
		if err != nil {
			log.Errorf("Error globbing %s: %v", filepath.Join(ancestorDir, "/scsi_host/*"), err)
		} else {
			if len(hostGlob) > 0 {
				if len(hostGlob) > 1 {
					log.Errorf("Unexpected: Drive %s has more than one host: %v", driveName, hostGlob)
				}
				for _, hostPath := range hostGlob {
					deviceDetails = append(deviceDetails, types.Detail{Tag: "SCSI Host", Value: filepath.Base(hostPath)})
				}
			}
		}
		ancestorDir = filepath.Dir(ancestorDir)
	}

	return deviceDetails
}

func getSCSIHostsInfo() (hostEntries []types.Entry) {
	var scsiHostAttributesToCollect = map[string]string{
		"proc_name":        "Module Name",
		"host_sas_address": "Host SAS Address",
		"unique_id":        "Unique ID",
	}
	hostEntries = make([]types.Entry, 0)

	hostDirs, _ := filepath.Glob("/sys/class/scsi_host/*")
	for _, hostDir := range hostDirs {
		entry := types.Entry{Category: "SCSI Host", Details: make([]types.Detail, 0)}

		entry.Details = append(entry.Details, types.Detail{Tag: "Name", Value: filepath.Base(hostDir)})

		for filename, label := range scsiHostAttributesToCollect {
			path := filepath.Join(hostDir, filename)
			fc, err := ioutil.ReadFile(path)
			if os.IsNotExist(err) {
				continue
			} else if err != nil {
				log.Errorf("Error opening %s: %v", path, err)
			}
			entry.Details = append(entry.Details, types.Detail{Tag: label, Value: strings.TrimSpace(string(fc))})
		}

		realHostDir, err := filepath.EvalSymlinks(hostDir)
		if err != nil {
			log.Errorf("Error getting direct path to %s: %v", hostDir, err)
		} else {
			expanderSASAddressFiles, _ := filepath.Glob(filepath.Join(filepath.Dir(filepath.Dir(realHostDir)), "/port*/expander*/sas_device/expander*/sas_address"))
			for _, path := range expanderSASAddressFiles {
				fc, err := ioutil.ReadFile(path)
				if err != nil {
					log.Errorf("Error opening %s: %v", path, err)
				}
				entry.Details = append(entry.Details, types.Detail{Tag: "Expander Address", Value: strings.TrimSpace(string(fc))})
			}
		}

		hostEntries = append(hostEntries, entry)
	}

	return hostEntries
}

// DiskRun returns disk information output
func DiskRun() ([]byte, error) {
	g := types.GenericInfo{}
	g.Entries = make([]types.Entry, 0)

	// collect info on disks found by smartctl
	disks := make([]Disk, 0)
	smartctlPath, err := exec.LookPath("smartctl")
	if err == nil {
		disks, _ = getDisks(smartctlPath)
	}
	for _, disk := range disks {
		// Get disk information:
		output, err := exec.Command(smartctlPath, "-i", disk.Path, "-d", disk.Type).Output()
		if err != nil && output == nil {
			// smartctl failed to collect any information, log the error and continue collecting information from other disks:
			log.Errorf("disk collection on %s failed: %v", disk.Path, err)
			continue
		} // otherwise smartctl collected some information before running into an error, or it completed collection without errors.
		lines := strings.Split(string(output), "\n")

		// Format disk information:
		e := smartctlResultFormat(disk.Path, lines)

		//if it's a simple scsi disk, look for scsi device information and add it to the entry
		if strings.HasPrefix(disk.Name, "sd") {
			deviceDetails := getSCSIDeviceInfo(disk.Name)
			e.Details = append(e.Details, deviceDetails...)
		}

		//add the entry to the section
		g.Entries = append(g.Entries, *e)
	}

	// collect info on disks found by walking Sysfs
	drives, _ := getBlockDrives()
	for _, drive := range drives {
		//ignore partitions and removables
		if drive.Removable || drive.Type == "partition" {
			continue
		}
		found := false
		for _, disk := range disks {
			if drive.Name == disk.Name {
				found = true
				break
			}
		}
		if !found {
			if smartctlPath != "" && ccissRegex.MatchString(drive.Name) { // deal with the special cciss case
				for i := 0; i < 16; i++ {
					devPath := filepath.Join("/dev/", drive.Name)
					output, err := exec.Command(smartctlPath, "-i", devPath, "-d", fmt.Sprintf("cciss,%d", i)).Output()
					if err != nil {
						continue
					}
					lines := strings.Split(string(output), "\n")

					// Format disk information:
					driveEntry := smartctlResultFormat(fmt.Sprintf("%s:%d", devPath, i), lines)
					g.Entries = append(g.Entries, *driveEntry)
				}
			} else { //deal with the general case
				driveEntry := blockDriveFormat(drive)
				g.Entries = append(g.Entries, *driveEntry)
			}
		}
	}

	// collect info on SCSI hosts
	hostEntries := getSCSIHostsInfo()
	g.Entries = append(g.Entries, hostEntries...)

	// Returns the formatted disk information:
	rjson, err := json.Marshal(g)
	if err != nil {
		return nil, err
	}
	return rjson, nil
}
