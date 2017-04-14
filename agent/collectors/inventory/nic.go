// +build linux

package inventory

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/Ericsson/ericsson-hds-agent/agent/collectors/types"
	"github.com/Ericsson/ericsson-hds-agent/agent/log"
)

func ethtoolParse(inp string) types.Detail {
	for _, line := range strings.Split(inp, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Speed:") {
			spVal := strings.TrimPrefix(line, "Speed:")
			return types.Detail{Tag: "speed", Value: strings.TrimSpace(spVal)}
		}
	}
	return types.Detail{Tag: "speed", Value: ""}
}

func ethtoolParseDashI(inp string) types.Detail {
	for _, line := range strings.Split(inp, "\n") {
		if strings.HasPrefix(line, "firmware-version:") {
			fwVal := strings.TrimPrefix(line, "firmware-version:")
			return types.Detail{Tag: "firmwareRevision", Value: strings.TrimSpace(fwVal)}
		}
	}
	return types.Detail{Tag: "firmwareRevision", Value: ""}
}

func parseIPAddresses(inp string) (addresses []types.Detail) {
	fields := strings.Fields(inp)
	for i, f := range fields {
		if f == "inet" && i+1 < len(fields) {
			addresses = append(addresses, types.Detail{Tag: "ipv4Address", Value: fields[i+1]})
		} else if f == "inet6" && i+1 < len(fields) {
			addresses = append(addresses, types.Detail{Tag: "ipv6Address", Value: fields[i+1]})
		}
	}
	return
}

//parse vlan ID and base interface from /proc/net/vlan/{adapter}
func parseVLANDetails(adapter, fileContent string) (details []types.Detail) {
	details = []types.Detail{}
	vlanIDPrefix := adapter + "  VID: "
	baseInterfacePrefix := "Device: "

	lines := strings.Split(fileContent, "\n")
	for i := range lines {
		if strings.HasPrefix(lines[i], vlanIDPrefix) {
			vlanID := strings.Split(strings.TrimPrefix(lines[i], vlanIDPrefix), "\t")[0]
			details = append(details, types.Detail{Tag: "vlanID", Value: vlanID})
		} else if strings.HasPrefix(lines[i], baseInterfacePrefix) {
			baseInterface := strings.TrimPrefix(lines[i], baseInterfacePrefix)
			details = append(details, types.Detail{Tag: "baseInterface", Value: baseInterface})
		}
	}

	return details
}

// isPhysicalInterface returns true if interface is physical
func isPhysicalInterface(ifname string) (bool, error) {
	fn := "/sys/class/net/" + ifname
	link, err := os.Readlink(fn)
	if err != nil {
		return false, fmt.Errorf("cannot readlink %s", fn)
	}
	if strings.Contains(link, "/virtual/") {
		return false, nil
	}
	return true, nil
}

func readAdapter(adapter string) *types.Entry {
	e := types.Entry{}
	e.Category = adapter

	// Read /sys/class/net/{device}/ifindex and iflink
	fc, err := ioutil.ReadFile(path.Join("/sys/class/net", adapter, "ifindex"))
	if err != nil {
		log.Errorf("can't read /sys/class/net/%v/ifindex, %v", adapter, err)
	}
	d := types.Detail{Tag: "ifindex", Value: strings.TrimSpace(string(fc))}
	e.Details = append(e.Details, d)

	// Read /sys/class/net/{device}/iflink
	fc, err = ioutil.ReadFile(path.Join("/sys/class/net", adapter, "iflink"))
	if err != nil {
		log.Errorf("can't read /sys/class/net/%v/iflink, %v", adapter, err)
	}
	d = types.Detail{Tag: "iflink", Value: strings.TrimSpace(string(fc))}
	e.Details = append(e.Details, d)

	// Read /sys/class/net/{device}/address
	fc, err = ioutil.ReadFile(path.Join("/sys/class/net", adapter, "address"))
	if err != nil {
		log.Errorf("can't read /sys/class/net/%v/address, %v", adapter, err)
	}
	d = types.Detail{Tag: "hardwareAddress", Value: strings.TrimSpace(string(fc))}
	e.Details = append(e.Details, d)

	// Read /sys/class/net/{device}/type
	fc, err = ioutil.ReadFile(path.Join("/sys/class/net", adapter, "type"))
	if err != nil {
		log.Errorf("can't read /sys/class/net/%v/address, %v", adapter, err)
	}
	d = types.Detail{Tag: "hardwareType", Value: strings.TrimSpace(string(fc))}
	e.Details = append(e.Details, d)

	// Follow symlink
	// TODO: Improve this
	fstr, err := os.Readlink(path.Join("/sys/class/net", adapter))
	pciBus := strings.TrimPrefix(strings.TrimSpace(fstr), "../../devices/")
	d = types.Detail{Tag: "pciBus", Value: pciBus}
	e.Details = append(e.Details, d)

	// get driver from /sys/class/net/{device}/
	fstr, err = os.Readlink(path.Join("/sys/class/net", adapter, "device/driver"))
	if err == nil {
		d = types.Detail{Tag: "driver", Value: path.Base(fstr)}
		e.Details = append(e.Details, d)
	}

	// Exec ethtool -i {device}
	//if the interface is virtual, just return a warning
	physical, _ := isPhysicalInterface(adapter)
	output, err := exec.Command("ethtool", "-i", string(adapter)).Output()
	if err != nil {
		if physical {
			log.Errorf(fmt.Sprintf("cannot run ethtool -i for interface %s : %v, recieved output [%s]", adapter, err, string(output)))
		}
	} else {
		d := ethtoolParseDashI(string(output))
		e.Details = append(e.Details, d)
		// Exec ethtool {device}
		output, err = exec.Command("ethtool", string(adapter)).Output()
		if err != nil {
			if physical {
				log.Errorf(fmt.Sprintf("cannot run ethtool for interface %s : %v, recieved output [%s]", adapter, err, string(output)))
			}
		} else {
			d = ethtoolParse(string(output))
			e.Details = append(e.Details, d)
		}
	}

	// Exec ip addr show {device}
	output, err = exec.Command("ip", "addr", "show", string(adapter)).Output()
	if err != nil {
		log.Errorf(fmt.Sprintf("cannot run ip addr show: %v,[%s]", err, string(output)))
	}

	e.Details = append(e.Details, parseIPAddresses(string(output))...)

	//check for vlan info
	fc, err = ioutil.ReadFile(path.Join("/proc/net/vlan", adapter))
	if err != nil && !os.IsNotExist(err) {
		log.Errorf("can't read /proc/net/vlan/%v, %v", adapter, err)
	} else if err == nil {
		vlanDetails := parseVLANDetails(adapter, string(fc))
		e.Details = append(e.Details, vlanDetails...)
	}

	//check for bond master
	fstr, err = os.Readlink(path.Join("/sys/class/net", adapter, "master"))
	if err == nil {
		d = types.Detail{Tag: "bondMaster", Value: path.Base(fstr)}
		e.Details = append(e.Details, d)
	}

	//check for bond info
	bondingDir := path.Join("/sys/class/net", adapter, "bonding")
	if fi, err := os.Stat(bondingDir); err == nil && fi.IsDir() {
		//get the bond mode
		modeFile := path.Join(bondingDir, "mode")
		fc, err = ioutil.ReadFile(modeFile)
		if err != nil && !os.IsNotExist(err) {
			log.Errorf("error reading %s: %v", modeFile, err)
		} else if err == nil {
			bondMode := strings.TrimSpace(string(fc))
			e.Details = append(e.Details, types.Detail{Tag: "bondMode", Value: bondMode})
		}

		//get the bond slaves
		slavesFile := path.Join(bondingDir, "slaves")
		fc, err = ioutil.ReadFile(slavesFile)
		if err != nil && !os.IsNotExist(err) {
			log.Errorf("error reading %s: %v", slavesFile, err)
		} else if err == nil {
			if len(fc) > 0 { //the bond might not have any slaves
				slavesJSON, err := json.Marshal(strings.Split(strings.TrimSpace(string(fc)), " "))
				if err != nil {
					log.Errorf("Error marshalling bonding slaves for adapter %v: %v. Content was: %v", adapter, err, string(fc))
				}
				e.Details = append(e.Details, types.Detail{Tag: "slaves", Value: string(slavesJSON)})
			}
		}

		//get the primary bond slave
		primarySlaveFile := path.Join(bondingDir, "primary")
		fc, err = ioutil.ReadFile(primarySlaveFile)
		if err != nil && !os.IsNotExist(err) {
			log.Errorf("error reading %s: %v", primarySlaveFile, err)
		} else if err == nil {
			if len(fc) > 0 { //the bond might not have a primary slave
				primarySlave := strings.TrimSpace(string(fc))
				e.Details = append(e.Details, types.Detail{Tag: "primarySlave", Value: primarySlave})
			}
		}
	} else if err != nil && !os.IsNotExist(err) {
		log.Errorf("Error reading bonding directory %v: %v", bondingDir, err)
	}

	return &e
}

func readAll() (*types.GenericInfo, error) {
	g := types.GenericInfo{}
	g.Entries = make([]types.Entry, 0)
	devices, err := ioutil.ReadDir("/sys/class/net")
	if err != nil {
		log.Errorf("can't read /sys/class/net, %v", err)
		return nil, err
	}

	for _, device := range devices {
		if _, err := ioutil.ReadDir(path.Join(path.Join("/sys/class/net", device.Name()))); err != nil {
			continue
		}

		if string(device.Name()) == "lo" {
			continue
		}

		if e := readAdapter(device.Name()); e != nil {
			if len(e.Details) == 0 {
				continue
			}
			g.Entries = append(g.Entries, *e)
		}
	}
	if len(g.Entries) == 0 {
		return nil, fmt.Errorf("unable to collect any NIC info")
	}
	return &g, nil
}

// NicRun returns inventory of all network interfaces
func NicRun() ([]byte, error) {
	result, err := readAll()
	if err != nil {
		return nil, err
	}
	rjson, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return rjson, nil
}
