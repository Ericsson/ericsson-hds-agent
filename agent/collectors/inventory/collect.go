package inventory

import "github.com/Ericsson/ericsson-hds-agent/agent/collectors"

// Collectors is a list of predefined inventory collectors
var Collectors = map[string]*collectors.CollectorFnWrapper{
	"sysinfo.package.rpm-package":  &collectors.CollectorFnWrapper{RunFn: RpmCollectRun, Dependencies: []string{"rpm"}, Type: "inventory.other"},
	"sysinfo.package.dpkg-package": &collectors.CollectorFnWrapper{RunFn: DpkgCollectRun, Dependencies: []string{"dpkg-query"}, Type: "inventory.other"},
	"sysinfo.disk":                 &collectors.CollectorFnWrapper{RunFn: DiskRun, Dependencies: []string{"smartctl"}, Type: "inventory.all"},
	"sysinfo.pci":                  &collectors.CollectorFnWrapper{RunFn: LsPCIRun, Dependencies: []string{"lspci"}, Type: "inventory.all"},
	"sysinfo.usb":                  &collectors.CollectorFnWrapper{RunFn: LsUSBRun, Dependencies: []string{"lsusb"}, Type: "inventory.all"},
	"sysinfo.nic":                  &collectors.CollectorFnWrapper{RunFn: NicRun, Dependencies: []string{}, Type: "inventory.all"}, //need ethtool to collect all data
	"sysinfo.smbios":               &collectors.CollectorFnWrapper{RunFn: SMBIOSRun, Dependencies: []string{"dmidecode"}, Type: "inventory.all"},
	"sysinfo.bmc.bmc-info":         &collectors.CollectorFnWrapper{RunFn: BmcInfoRun, PrecheckFn: BmcPrecheck, Dependencies: []string{"bmc-info"}, Type: "inventory.all"},
	"sysinfo.bmc.ipmi-tool":        &collectors.CollectorFnWrapper{RunFn: IpmiToolRun, PrecheckFn: BmcPrecheck, Dependencies: []string{"ipmitool"}, Type: "inventory.all"},
	"sysinfo.proc":                 &collectors.CollectorFnWrapper{RunFn: ProcInfoRun, Dependencies: []string{}, Type: "inventory.all"},
	"sysinfo.ecc":                  &collectors.CollectorFnWrapper{RunFn: ECCRun, PrecheckFn: ECCPrecheck, Dependencies: []string{}, Type: "inventory.all"},
}
