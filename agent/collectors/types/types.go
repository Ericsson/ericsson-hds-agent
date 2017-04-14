package types

// A Detail represents Tag and Value pair
type Detail struct {
	Tag   string
	Value string
}

// An Entry contains category type and set of Detail
type Entry struct {
	Category string
	Details  []Detail
}

// A SMBIOS contains version info and set of Entry
type SMBIOS struct {
	Version string
	Entries []Entry
}

// A GenericInfo contains set of Entry
type GenericInfo struct {
	Entries []Entry
}

// MountStat contains information of mounted devices
type MountStat struct {
	DevicePath     string
	Mountpoint     string
	FilesystemType string
	Options        string
}

// MountUsageStat contains statastics of mounted devices
type MountUsageStat struct {
	Mount       *MountStat
	Total       uint64
	Free        uint64
	Available   uint64 //available to unprivileged users
	Used        uint64 //Total - Free
	Unavailable uint64 //Total - Available
	InodesTotal uint64
	InodesFree  uint64
	InodesUsed  uint64
}

// Disk contains information of disk
type Disk struct {
	Path string
	Name string
	Type string
}

// BlockDrive contains information of block drives on the machine
type BlockDrive struct {
	Name             string
	SysfsPath        string
	MajMin           string
	Type             string
	Size             int64 //in bytes
	Removable        bool
	ReadOnly         bool
	Revision         string
	Vendor           string
	Product          string
	LogicalBlockSize string
	StorageType      string
}
