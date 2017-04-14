package agent

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/Ericsson/ericsson-hds-agent/agent/collectors"
)

// BaseCollector contains common information for collectors
type BaseCollector struct {
	name          string
	collectorType string
	state         string
	timeout       time.Duration // timeout duration
	numTimeout    int           // number of times timeout
	numErrs       int           // number of times error
	precheck      func() error
	dependencies  []string // 3-rd party dependencies names
}

// MetricCollector contains information of metric collector
type MetricCollector struct {
	BaseCollector
	killCh    chan struct{}
	frequency time.Duration
	collect   func() ([]*collectors.MetricResult, error)
}

// InventoryCollector contains information of inventory collector
type InventoryCollector struct {
	BaseCollector
	collect func() ([]byte, error)
}

// Metadata is wrapper struct for hosttype
type Metadata struct {
	HostType string
}

// Destination contains information of destination server where metric/inventory
// data needs to be sent
type Destination struct {
	dst       string // where to send collector output
	proto     string // what protocol to send with i.e. UDP/TCP/HTTP
	transport string // currently support tcp
}

type metricResultCollector struct {
	collectors.MetricResult
	isNeedSendHeader bool
}

type metric struct {
	Name, NodeID   string
	Data           []*metricResultCollector
	Timeout        bool
	Err            error
	Frequency      time.Duration
	CollectionTime time.Time
}

// Blob defines format of message sent to stdout or sendBuffer
type Blob struct {
	Type           string
	ID             int
	Digest, NodeID string
	Timestamp      string
	Content        json.RawMessage
}

type command struct {
	Name, CmdID, FileURL, RunCmd, NodeID string
	RunArgs                              []string
}

type commandOutput struct {
	NodeID  string   `json:"nodeID"`
	CmdID   string   `json:"cmdID"`
	FileURL string   `json:"fileURL"`
	RunCmd  string   `json:"runCmd"`
	RunArgs []string `json:"runArgs"`
	Status  string   `json:"status"`
	Stdout  string   `json:"stdout"`
	Stderr  string   `json:"stderr"`
}

//Config settings saved in external storage
//
//so we can run hds-agent without command line flags
type Config struct {
	NodeID           string `json:"-"` // ID of host machine
	Chdir            string `json:"chdir"`
	CollectorTimeout int    `json:"collection-timeout"` // number of seconds before a collector times out
	Destination      string `json:"destination"`
	DryRun           bool   `json:"dry-run"`
	Duration         int    `json:"duration"` // How many seconds to run agent for
	Freq             int    `json:"frequency"`
	SkipStr          string `json:"skipStr"`
	Stdout           bool   `json:"stdout"`
	WaitTime         int    `json:"retrywait"` // number of seconds between attempting to reconnect to remote server
}

// Agent represents information of agent like metric, inventory etc
type Agent struct {
	hostname            string                 // hostname of agent's computer
	Skipmap             map[string]struct{}    // collectors to skip
	MetricFrequency     time.Duration          // frequency of metric collection
	InvFrequency        time.Duration          // frequency of inventory collection
	CollectorTimeout    time.Duration          // time to wait on collection before timing out
	metricHeaders       metricHeaderMap        // map of metric headers to send to server
	metricMetadata      metricMetadataMap      // map of metadata lines to send to server
	ID                  int                    // ID of the inventory blob
	inventoryCollectors inventoryCollectorList // inventory collectors
	metricCollectors    metricCollectorList    // metric collectors
	TimeoutLimit        int                    // max number of times a collector can timeout before being skipped
	ErrorLimit          int                    // max number of times a collector can error out before being skipped
	timeoutConnSend     time.Duration          // amount of time before sending data to the server times out
	dialTimeout         time.Duration          // amount of time before a connection attempt times out
	SendCh              chan []byte            // channel for sending data to Server
	WaitGroup           sync.WaitGroup         // wait for collectors to finish
	WaitTime            time.Duration          // number of seconds between attempting to reconnect to remote server
	Config              *Config                // the agent config object
	nodesMtx            sync.RWMutex
	SigChan             chan os.Signal
	Destination         *Destination // currnet destination
}

type metricHeaderMap struct {
	sync.RWMutex                   // protect list if it is being updated
	Map          map[string]string // map of metric headers
}

type metricMetadataMap struct {
	sync.RWMutex                              // protect list if it is being updated
	Map          map[string]map[string]string // map of metric metadata
}

type inventoryCollectorList struct {
	sync.RWMutex                                // protect list if it is being updated dynamically
	List         map[string]*InventoryCollector // list of collectors
}

type metricCollectorList struct {
	sync.RWMutex                             // protect list if it is being updated dynamically
	List         map[string]*MetricCollector // list of collectors
}

// Inventory contains information of inventory of machine
type Inventory struct {
	Name, Type string
	Data       json.RawMessage
	err        error
	Timeout    bool
}

type metricsCollector struct {
	Collector, Header, Values string
}
