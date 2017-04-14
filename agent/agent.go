package agent

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Ericsson/ericsson-hds-agent/agent/collectors/inventory"
	"github.com/Ericsson/ericsson-hds-agent/agent/log"
)

//Start checks for errors in configuration, initializes nodeid and starts the agent
func Start(a *Agent) {

	//connect to destination
	a.connect()

	log.Info("start collections")
	a.scheduleInventory()
	a.scheduleMetrics()

	a.WaitGroup.Wait()
}

// Stop the agent:
func (a *Agent) Stop() {
	log.Info("Stopping agent")

	a.inventoryCollectors.Lock()
	for _, invCol := range a.inventoryCollectors.List {
		if invCol.state == runningState {
			invCol.state = stopState
		}
	}
	a.inventoryCollectors.Unlock()
	a.metricCollectors.Lock()
	for _, metCol := range a.metricCollectors.List {
		if metCol.state == runningState {
			metCol.state = stopState
		}
	}
	a.metricCollectors.Unlock()

	log.Info("Finished stopping agent")
	a.WaitGroup.Done()
}

// NonBlockingSend sends data to stdout and over channel and is non-blocking
func (a *Agent) NonBlockingSend(data []byte) {
	// Send data to stdout
	if a.Config.Stdout {
		os.Stdout.Write(data)
		os.Stdout.Write([]byte{'\n'})
	}

	if a.Config.Destination != "" || a.Config.DryRun {
		select {
		case a.SendCh <- data:
		default:
			log.Errorf("unable to send data to destination %v", a.Config.Destination)
		}
	}
}

// showHelp outputs the help if given invalid arguments and exits
func showUsage(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "\n")
		fmt.Fprintln(os.Stderr, err, "\n")
	}

	flag.Usage()
}

// Trap interrupts and exit
func handleInterrupt(a *Agent, intrptChSize int) {
	a.SigChan = make(chan os.Signal, intrptChSize)
	signal.Notify(a.SigChan, os.Interrupt, os.Kill, syscall.SIGTERM)
	go func() {
		for sig := range a.SigChan {
			log.Infof("agent received %s signal, exiting", sig)
			os.Exit(1)
		}
	}()
}

// Initialize initializes the Agent
func (a *Agent) Initialize() error {
	var err error
	var dst *Destination

	config := a.Config

	//check for wrong arguments
	if len(flag.Args()) > 0 {
		err := fmt.Errorf("invalid command line arguments")
		showUsage(err)
		return err
	}

	//handle dry-run
	if config.DryRun {
		config.Destination = ""
		initialFreq := config.Freq
		config.Freq = 0
		config.Stdout = true
		dryRun(a, initialFreq)
	}

	//check parameters are valid
	if err := config.CheckErrs(); err != nil {
		showUsage(err)
		return err
	}

	if path, err := filepath.Abs(config.Chdir); err != nil {
		log.Errorf("resolving directory [%s] error: %v", config.Chdir, err)
		return err
	} else if err = os.Chdir(path); err != nil {
		log.Errorf("can't chdir to %v, %v", config.Chdir, err)
		return err
	} else {
		config.Chdir = path
	}

	if len(config.NodeID) == 0 {
		if err := config.InitializeNodeID(); err != nil {
			fmt.Printf("Error initializing NodeId: %s", err)
			return err
		}
	}

	// Skips collectors:
	a.Skipmap = make(map[string]struct{})
	if config.SkipStr != "" {
		skiplist := strings.Split(config.SkipStr, ",")
	SKIPLOOP:
		for _, skipName := range skiplist {
			if "all" == skipName {
				a.Skipmap[skipName] = struct{}{}
				log.Infof("skipping collector %s", skipName)
				break
			}

			for invScriptName := range inventory.Collectors {
				if invScriptName == skipName {
					a.Skipmap[skipName] = struct{}{}
					log.Infof("skipping collector %s", skipName)
					continue SKIPLOOP
				}
			}

			for metricScriptName := range metricCollectors {
				if metricScriptName == skipName {
					a.Skipmap[skipName] = struct{}{}
					log.Infof("skipping collector %s", skipName)
					continue SKIPLOOP
				}
			}
			err := fmt.Errorf("Collector %s not found", skipName)
			log.Errorf("%s", err)
		}

	}

	// Set destination
	if dst, err = parseDest(config.Destination); err != nil {
		log.Errorf("invalid command line arguments to -destination, %v", err)
		return err
	}
	a.Destination = dst
	a.MetricFrequency = time.Duration(config.Freq) * time.Second
	a.WaitTime = time.Duration(config.WaitTime) * time.Second
	a.CollectorTimeout = time.Duration(config.CollectorTimeout) * time.Second

	go handleInterrupt(a, intrptChSize)

	// Handle timeout on agent:
	a.WaitGroup.Add(1)
	if a.Config.Duration > 0 {
		go func() {
			<-time.After(time.Duration(a.Config.Duration) * time.Second)
			log.Info("Duration complete stopping agent")
			a.Stop()
		}()
	}

	// Configure agent:
	a.InvFrequency = invFrequency
	if a.MetricFrequency.Nanoseconds() == 0 {
		a.InvFrequency = 0 * time.Nanosecond
	}

	a.SendCh = make(chan []byte, len(inventory.Collectors))
	a.TimeoutLimit = failureLimit
	a.ErrorLimit = failureLimit
	a.timeoutConnSend = timeoutConnSend
	a.dialTimeout = dialTimeout

	// Get agent's hostname
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unidentified"
	}
	a.hostname = hostname

	// Core-Scripts
	_, skipAll := a.Skipmap["all"]
	a.metricHeaders.Map = make(map[string]string)

	a.inventoryCollectors.Lock()
	a.inventoryCollectors.List = make(map[string]*InventoryCollector)
	for invScriptName, invCollectorFuncWrapper := range inventory.Collectors {

		newScript := &InventoryCollector{
			collect: invCollectorFuncWrapper.RunFn,
			BaseCollector: BaseCollector{
				name:          invScriptName,
				precheck:      invCollectorFuncWrapper.PrecheckFn,
				dependencies:  invCollectorFuncWrapper.Dependencies,
				collectorType: invCollectorFuncWrapper.Type,
				timeout:       a.CollectorTimeout,
				state:         runningState,
			},
		}

		if _, ok := a.Skipmap[invScriptName]; ok || skipAll || newScript.Precheck(a) != nil {
			newScript.state = stopState
		}
		a.inventoryCollectors.List[invScriptName] = newScript
	}

	a.inventoryCollectors.Unlock()

	a.metricCollectors.Lock()
	a.metricCollectors.List = make(map[string]*MetricCollector)
	for metricScriptName, metricCollectorFuncWrapper := range metricCollectors {
		newScript := &MetricCollector{
			collect:   metricCollectorFuncWrapper.RunFn,
			frequency: a.MetricFrequency,
			BaseCollector: BaseCollector{
				name:          metricScriptName,
				precheck:      metricCollectorFuncWrapper.PrecheckFn,
				dependencies:  metricCollectorFuncWrapper.Dependencies,
				collectorType: builtIn,
				timeout:       a.CollectorTimeout,
				state:         runningState,
			},
			killCh: make(chan struct{}),
		}

		if _, ok := a.Skipmap[metricScriptName]; ok || skipAll || newScript.Precheck(a) != nil {
			newScript.state = stopState
		}
		a.metricCollectors.List[metricScriptName] = newScript
	}
	a.metricCollectors.Unlock()

	// User-scripts
	log.Info("checking for user scripts")
	a.addUserScripts()

	if err := a.monitorUserScripts(typeInventory); err != nil {
		log.Infof("error monitoring inventory scripts: %s", err)
	}

	if err := a.monitorUserScripts(typeMetric); err != nil {
		log.Infof("error monitoring metric scripts: %s", err)
	}

	a.metricMetadata.Map = make(map[string]map[string]string)

	return nil
}
