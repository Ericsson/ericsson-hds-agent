package agent

import (
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"github.com/Ericsson/ericsson-hds-agent/agent/log"
)

// NewDefaultConfig sets the default flags for the Agent so we can support passing no flags from the command line
func NewDefaultConfig() *Config {
	return &Config{
		Chdir:            ".",
		CollectorTimeout: collectorTimeout,
		WaitTime:         10,
	}
}

// InitFlags binds set of flags to variables in Agent config
func InitFlags(c *Config) {
	flag.BoolVar(&c.Stdout, "stdout", c.Stdout, "send to STDOUT")
	flag.StringVar(&c.Chdir, "chdir", c.Chdir, "change the working directory")
	flag.StringVar(&c.SkipStr, "skip", c.SkipStr, "disable preset collectors. i.e: \"-skip=cpu,disk\"")
	flag.IntVar(&c.Freq, "frequency", c.Freq, "collection frequency in seconds. set to >0 to repeat")
	flag.IntVar(&c.CollectorTimeout, "collection-timeout", c.CollectorTimeout, "specify collection timeout in seconds")
	flag.StringVar(&c.Destination, "destination", c.Destination, "send data to server. i.e: \"-destination=tcp:localhost:12345\"")
	flag.BoolVar(&c.DryRun, "dry-run", c.DryRun, "validate environment setting to run collections")
	flag.IntVar(&c.WaitTime, "retrywait", c.WaitTime, "wait time in seconds before reconnect to destination")
	flag.IntVar(&c.Duration, "duration", c.Duration, "number of seconds to run the agent for. 0 for non-stop")
}

// CheckErrs validates values of Config fields
func (c *Config) CheckErrs() error {
	if c.Chdir == "" {
		return fmt.Errorf("invalid command line arguments to -chdir")
	}

	if c.Freq < 0 || ((time.Duration(c.Freq) * time.Second) < 0) {
		return fmt.Errorf("invalid value passed to flag -frequency. Value must be >= 0, but given %v", c.Freq)
	}

	if c.CollectorTimeout <= 0 {
		return fmt.Errorf("invalid value passed to flag -collection-timeout. Value must be > 0, but given %v", c.CollectorTimeout)
	}

	if c.WaitTime <= 0 || ((time.Duration(c.WaitTime) * time.Second) <= 0) {
		return fmt.Errorf("invalid value passed to flag -retrywait. Value must be > 0, but given %v", c.WaitTime)
	}

	if c.Duration < 0 || ((time.Duration(c.Duration) * time.Second) < 0) {
		return fmt.Errorf("invalid value passed to flag -duration. Value must be >= 0, but given %v", c.Duration)
	}

	if c.Stdout == false && c.Destination == "" {
		return fmt.Errorf("provide at least one valid output flag -stdout or -destination")
	}

	return nil
}

var (
	errNodeIDEmpty     = errors.New("empty node.id file")
	errNodeIDMalformed = errors.New("malformed node.id file")
)

// ReadNodeID reads the node.id file when available
func (c *Config) ReadNodeID() (string, error) {
	data, err := ioutil.ReadFile(filepath.Join(c.Chdir, "node.id"))
	if err != nil || len(data) == 0 {
		return "", errNodeIDEmpty
	}

	// Trim all leading and trailing whitespace
	nodeIDStr := strings.TrimSpace(string(data))
	if len(nodeIDStr) == 0 {
		return "", errNodeIDEmpty
	}

	// Make sure that there is no whitespace inside of the nodeid
	fields := strings.Fields(nodeIDStr)
	if len(fields) > 1 {
		return "", errNodeIDMalformed
	}

	return nodeIDStr, nil
}

// WriteNodeID writes the node.id file
func (c *Config) WriteNodeID() error {
	nodeIDFile := "node.id"
	err := ioutil.WriteFile(nodeIDFile, []byte(c.NodeID), 0666)
	if err != nil {
		return fmt.Errorf("cannot write file [%s]: %v", nodeIDFile, err)
	}
	return nil
}

func makeUniqueID() string {
	b := make([]byte, 16)

	if _, err := rand.Read(b); err != nil {
		log.Errorf("can't generate random ID, %v", err)
	}
	return fmt.Sprintf("%x", b[:16])
}

// InitializeNodeID will set nodeID if agent doesn't already have node.id file with valid value
func (c *Config) InitializeNodeID() error {
	nodeID, err := c.ReadNodeID()
	if err != nil && err == errNodeIDMalformed {
		return err
	} else if err == nil {
		c.NodeID = nodeID
		return nil
	}

	nodeID = makeUniqueID()
	log.Infof("created ID %s", nodeID)
	c.NodeID = nodeID
	return c.WriteNodeID()
}
