package inventory

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

// Disk contains information of disk
type Disk struct {
	Path string
	Name string
	Type string
}

type mcelog struct {
	cpu  string // Should be same as memory controller (edac)
	bank string // Same as dimm (edac)
	ue   int    // Uncorrected error
	ce   int    // Corrected error
}
