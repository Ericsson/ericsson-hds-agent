package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Ericsson/ericsson-hds-agent/agent/collectors"
	"github.com/Ericsson/ericsson-hds-agent/agent/log"
	"github.com/fsnotify/fsnotify"
)

const (
	typeInventory             = "Inventory"
	typeMetric                = "Metrics"
	modePermExec  os.FileMode = 0111
)

// Walks through a provided directory's files and subdirectories, appending the path of executable files to given fileList
func getExecutableFiles(root string, fileList *[]string) error {
	// Check that root exists
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return err
	}

	// Walk through root's files and subdirectories, appending executable paths to provided fileList
	err := filepath.Walk(root, func(path string, file os.FileInfo, _ error) error {
		if !file.IsDir() && file.Mode()&modePermExec != 0 {
			*fileList = append(*fileList, path)
		}
		return nil
	})

	return err
}

// Get the user collector scripts. colType is either "Inventory" or "Metrics"
func (a *Agent) getUserCollectors(colType string) error {
	var colsPaths []string

	if err := getExecutableFiles(fmt.Sprintf("%s/%s", a.Config.Chdir, colType), &colsPaths); err != nil {
		return err
	}

	for _, col := range colsPaths {
		switch colType {
		case typeInventory:
			if err := a.addInventoryScript(col); err != nil {
				log.Errorf("error when adding inventory collector %s: %v", col, err)
			}
		case typeMetric:
			if err := a.addMetricsScript(col); err != nil {
				log.Errorf("error when adding metrics collector %s: %v", col, err)
			}
		default:
			log.Errorf("invalid user collector type [%s]", colType)
		}
	}

	return nil
}

// Add all user scripts to collector lists. Does not check for duplicates
func (a *Agent) addUserScripts() {
	if err := a.getUserCollectors(typeInventory); err != nil {
		log.Info(err.Error())
	}

	if err := a.getUserCollectors(typeMetric); err != nil {
		log.Info(err.Error())
	}
}

// Watch user script directory for file changes
func (a *Agent) monitorUserScripts(scriptType string) error {
	var (
		addScript    func(string) error
		addDirectory func(string, *fsnotify.Watcher) error
		removeScript func(string)
	)

	switch scriptType {
	case typeInventory:
		addScript = a.addInventoryScript
		addDirectory = a.addInventoryDirectory
		removeScript = a.removeInventoryScript
	case typeMetric:
		addScript = a.addMetricsScript
		addDirectory = a.addMetricsDirectory
		removeScript = a.removeMetricsScript
	default:
		return fmt.Errorf("incorrect script type. Given: %s", scriptType)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	if err := recursiveWatch(filepath.Join(a.Config.Chdir, scriptType), watcher); err != nil {
		return err
	}

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					log.Infof("shutting down %s script watcher...", scriptType)
					return
				}
				log.Infof("%s folder event: %s", scriptType, event.String())
				if event.Op&fsnotify.Create == fsnotify.Create {
					if file, err := os.Stat(event.Name); err != nil {
						log.Infof("error when reading file or directory %s: %s", event.Name, err)
					} else if file.IsDir() {
						err := addDirectory(event.Name, watcher)
						if err != nil {
							log.Errorf("error when watching new directory %s: %v", event.Name, err)
						}
					} else {
						err := addScript(event.Name)
						if err != nil {
							log.Errorf("error when watching new script %s: %v", event.Name, err)
						}
					}
				}
				if event.Op&fsnotify.Remove == fsnotify.Remove || event.Op&fsnotify.Rename == fsnotify.Rename {
					removeScript(event.Name)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					log.Infof("shutting down %s script watcher...", scriptType)
					return
				}
				log.Infof("%s folder error: %s", scriptType, err)
			}
		}
	}()

	return nil
}

// Add a directory of user inventory scripts to the inventory collectors list
func (a *Agent) addInventoryDirectory(path string, watcher *fsnotify.Watcher) error {
	if err := recursiveWatch(path, watcher); err != nil {
		return err
	}

	var newScripts []string
	getExecutableFiles(path, &newScripts)

	for _, script := range newScripts {
		if err := a.addInventoryScript(script); err != nil {
			log.Errorf("error when adding inventory collector %s: %v", script, err)
		}
	}

	return nil
}

// Add a user inventory script to the inventory collectors list
func (a *Agent) addInventoryScript(path string) error {
	if file, err := os.Stat(path); err != nil {
		return err
	} else if !file.IsDir() && file.Mode()&modePermExec != 0 {
		name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		collector := &InventoryCollector{
			collect: generateInvCollectorFunction(path),
			BaseCollector: BaseCollector{
				name:          name,
				collectorType: userScript,
				state:         runningState,
				timeout:       a.CollectorTimeout,
			},
		}
		a.inventoryCollectors.Lock()
		a.inventoryCollectors.List[collector.name] = collector
		a.inventoryCollectors.Unlock()
		log.Infof("added inventory collector %s", name)
	}

	return nil
}

// Remove a user inventory script from the inventory collectors list given the path to its location
func (a *Agent) removeInventoryScript(path string) {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	a.inventoryCollectors.Lock()
	delete(a.inventoryCollectors.List, name)
	a.inventoryCollectors.Unlock()
	log.Infof("removed inventory collector %s", name)
}

// Add a directory of user metric scripts to the metric collectors list
func (a *Agent) addMetricsDirectory(path string, watcher *fsnotify.Watcher) error {
	if err := recursiveWatch(path, watcher); err != nil {
		return err
	}

	var newScripts []string
	getExecutableFiles(path, &newScripts)

	for _, script := range newScripts {
		if err := a.addMetricsScript(script); err != nil {
			log.Errorf("error when adding metrics collector %s: %v", script, err)
		}
	}

	return nil
}

// Add a user metric script to the metric collectors list and start collector
func (a *Agent) addMetricsScript(path string) error {
	if file, err := os.Stat(path); err != nil {
		return err
	} else if file.IsDir() || file.Mode()&modePermExec == 0 {
		return errors.New(path + " is not executable file")
	}
	name := "user." + strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	collector := &MetricCollector{
		frequency: a.MetricFrequency,
		collect:   generateMetricsCollectorFunction(path),
		killCh:    make(chan struct{}),
		BaseCollector: BaseCollector{
			name:          name,
			collectorType: userScript,
			timeout:       a.CollectorTimeout,
			state:         runningState,
		},
	}
	a.metricCollectors.Lock()
	defer a.metricCollectors.Unlock()
	if _, ok := a.metricCollectors.List[collector.name]; ok {
		log.Infof("metric %s collector exist", name)
		return nil
	}
	a.metricCollectors.List[collector.name] = collector
	log.Infof("added metrics collector %s", name)

	// start metric collector
	go a.scheduleMetricCollector(collector)

	return nil
}

// Remove a user metric script from the metric collectors list given the path to
// its location and stop collector
func (a *Agent) removeMetricsScript(path string) {
	name := "user." + strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	a.metricCollectors.Lock()
	defer a.metricCollectors.Unlock()
	if col, ok := a.metricCollectors.List[name]; ok {
		col.state = stopState
		close(col.killCh)
	}
	delete(a.metricCollectors.List, name)
	log.Infof("removed metrics collector %s", name)
}

// Wraps a call to an external executable in a CollectorFunc for inventories
func generateInvCollectorFunction(cmd string) func() ([]byte, error) {
	return func() ([]byte, error) {
		output, err := exec.Command(cmd).Output()
		if err != nil {
			out := ""
			if output != nil {
				out = string(output)
			}
			return nil, fmt.Errorf("run user script [%s] error: %v, output [%s]", cmd, err, out)
		}
		rjson, err := json.Marshal(string(output))
		if err != nil {
			return nil, err
		}
		return rjson, nil
	}
}

// Wraps a call to an external executable in a CollectorFunc for metrics
func generateMetricsCollectorFunction(cmd string) func() ([]*collectors.MetricResult, error) {
	return func() ([]*collectors.MetricResult, error) {
		output, err := exec.Command(cmd).Output()
		if err != nil {
			out := ""
			if output != nil {
				out = string(output)
			}
			return nil, fmt.Errorf("run user script [%s] error: %v, output [%s]", cmd, err, out)
		}

		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(lines) != 2 {
			log.Errorf("Incorrect return format: headers new line data, from script: [%s], output: [%s]", cmd, string(output))
		}
		result := &collectors.MetricResult{
			Header:   lines[0],
			Data:     lines[1],
			Sufix:    "",
			Metadata: nil,
		}
		return []*collectors.MetricResult{result}, nil
	}
}

// Add watchers to a given directory and its sub-directories
func recursiveWatch(root string, watcher *fsnotify.Watcher) error {
	// Sanity checks
	if info, err := os.Stat(root); os.IsNotExist(err) {
		return err
	} else if !info.IsDir() {
		return fmt.Errorf("cannot recurse for watching subdirectories: [%s] is not a directory", root)
	}

	log.Infof("watching %s", root)

	err := filepath.Walk(root, func(path string, file os.FileInfo, _ error) error {
		if file.IsDir() {
			if watcherErr := watcher.Add(path); watcherErr != nil {
				return watcherErr
			}
			log.Infof("added watcher to %s", path)
		}
		return nil
	})

	return err
}
