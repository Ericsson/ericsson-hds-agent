package agent

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/Ericsson/ericsson-hds-agent/agent/collectors"
	"github.com/Ericsson/ericsson-hds-agent/agent/collectors/cpu"
	"github.com/Ericsson/ericsson-hds-agent/agent/collectors/disk"
	"github.com/Ericsson/ericsson-hds-agent/agent/collectors/diskusage"
	"github.com/Ericsson/ericsson-hds-agent/agent/collectors/load"
	"github.com/Ericsson/ericsson-hds-agent/agent/collectors/memory"
	"github.com/Ericsson/ericsson-hds-agent/agent/collectors/net"
	"github.com/Ericsson/ericsson-hds-agent/agent/collectors/sensor"
	"github.com/Ericsson/ericsson-hds-agent/agent/collectors/smart"
	"github.com/Ericsson/ericsson-hds-agent/agent/collectors/uptime"
	"github.com/Ericsson/ericsson-hds-agent/agent/log"
	gometrics "github.com/rcrowley/go-metrics"
)

var metricCollectors = map[string]*collectors.MetricFnWrapper{
	"disk":      &collectors.MetricFnWrapper{RunFn: disk.Run},
	"cpu":       &collectors.MetricFnWrapper{RunFn: cpu.Run},
	"load":      &collectors.MetricFnWrapper{RunFn: load.Run},
	"memory":    &collectors.MetricFnWrapper{RunFn: memory.Run},
	"net":       &collectors.MetricFnWrapper{RunFn: net.Run},
	"uptime":    &collectors.MetricFnWrapper{RunFn: uptime.Run},
	"diskusage": &collectors.MetricFnWrapper{RunFn: diskusage.Run},
	"smart":     &collectors.MetricFnWrapper{RunFn: smart.Run, PrecheckFn: smart.Precheck},
	"sensor":    &collectors.MetricFnWrapper{RunFn: sensor.IpmiSensorRun, PrecheckFn: sensor.IpmiSensorPrecheck},
}

// HeaderStrings returns the formatted string iof a metric header
func (m *metric) HeaderStrings() []string {
	result := []string{}

	for _, mr := range m.Data {
		hname := m.Name + mr.Sufix
		result = append(result, m.formatHeaderString(hname, mr.Header))
	}
	return result
}

func (m *metric) formatHeaderString(mname, header string) string {
	mname = strings.TrimSpace(mname)
	if len(header) > 0 {
		return fmt.Sprintf(`:=:header %s %s %.0f #timestamp %s`, mname, m.NodeID, m.Frequency.Seconds(), header)
	}
	return ""
}

// Formats a metric data string
func (m *metric) formatDatastring(mname, data string) string {
	if len(data) > 0 {
		return fmt.Sprintf(`:=:%s %s %.0f %d %s`, mname, m.NodeID, m.Frequency.Seconds(), m.CollectionTime.Unix(), data)
	}
	return ""
}

// Format returns formatted string of metrics data
func (m *metric) Format() (result string) {

	for _, v := range m.Data {
		mname := m.Name + v.Sufix
		if len(v.Header) == 0 {
			continue
		}
		if v.isNeedSendHeader {
			result += m.formatHeaderString(mname, v.Header) + "\n"
			for name, val := range v.Metadata {
				result += formatMetadataString(mname, m.NodeID, m.Frequency, name, val) + "\n"
			}
		}
		result += m.formatDatastring(mname, v.Data) + "\n"

	}
	result = strings.TrimSuffix(result, "\n")
	return
}

// Collects, formats, and sends metrics
func (a *Agent) processMetric(metric metric, c *MetricCollector) error {
	metric.NodeID = a.Config.NodeID

	a.metricHeaders.Lock()
	headers := metric.HeaderStrings()
	for i := range metric.Data {
		v := metric.Data[i]
		if len(v.Header) == 0 {
			continue
		}
		v.isNeedSendHeader = !strings.HasSuffix(a.metricHeaders.Map[metric.Name+v.Sufix],
			"#timestamp "+v.Header)
		if v.isNeedSendHeader {
			a.metricHeaders.Map[metric.Name+v.Sufix] = headers[i]
			for k, va := range v.Metadata {
				a.setOneMetadata(metric.Name+v.Sufix, k, va, false)
			}
		}
	}
	a.metricHeaders.Unlock()

	log.Infof("processing metric '%s'", metric.Name)
	var metricBytes []byte
	var err error

	switch {
	case metric.Err != nil:
		err = fmt.Errorf("Error collecting metric %s: %v", metric.Name, metric.Err)
		c.numErrs++
		if c.numErrs >= a.ErrorLimit {
			log.Infof("Metric Collector %s has reached max number of errors and will not be collected.", metric.Name)
			c.state = stopState
		}
	case metric.Timeout:
		err = fmt.Errorf("timeout when collecting metric %s", metric.Name)
		c.numTimeout++
		if c.numTimeout >= a.TimeoutLimit {
			log.Infof("Metric Collector %s has reached max number of timeouts and will not be collected.", metric.Name)
			c.state = stopState
		}
	default:
		//Reset Error and Timeout Counters
		c.numTimeout = 0
		c.numErrs = 0

		// Compile metric data:
		metricBytes = []byte(metric.Format())
	}
	log.Infof("Done processing metric: %s", metric.Name)

	// sending metric data
	if len(metricBytes) > 0 {
		a.NonBlockingSend(metricBytes)
	}

	return err
}

func (a *Agent) runMetricCollector(c *MetricCollector) error {
	metric := handleMetricCollection(c)
	err := a.processMetric(metric, c)
	if err != nil {
		log.Error(err.Error())
		return err
	}
	return nil
}

func (a *Agent) scheduleMetricCollector(c *MetricCollector) {
	var ticker = &time.Ticker{}
	var workerCh chan struct{}

	workerCh = make(chan struct{}, 100)
	go func() {
		for _ = range workerCh {
			// run metric collector now
			_ = a.runMetricCollector(c)
		}
	}()

	log.Infof("starting metric collector %v", c.name)

	if c.state == runningState {
		workerCh <- struct{}{}
	}

	if c.frequency > 0 {
		ticker = time.NewTicker(c.frequency)
		if c.state == stopState {
			ticker.Stop()
		}
	} else {
		log.Infof("ran metric collector '%v' once (frequency <= 0)", c.name)
		// stop as it needs to run max once
		c.state = stopState
	}

	// initlize collector members

	killCh := c.killCh
	for {
		select {
		case <-killCh:
			log.Infof("received kill signal for collector '%s'", c.name)
			ticker.Stop()
			close(workerCh)
			return

		case <-ticker.C:
			if c.state == stopState {
				ticker.Stop()
				log.Infof("stop metric collector '%s'", c.name)
				continue
			}
			workerCh <- struct{}{}
		}
	}
}

// schedules all initial metric collectors
func (a *Agent) scheduleMetrics() {
	a.metricCollectors.RLock()
	for _, col := range a.metricCollectors.List {
		if strings.HasPrefix(col.name, "user.") {
			// ignore as it is already started
			continue
		}
		log.Infof("scheduling metric collector '%v'", col.name)
		go a.scheduleMetricCollector(col)
	}
	a.metricCollectors.RUnlock()
}

func handleMetricCollection(c *MetricCollector) metric {
	resCh := make(chan []*collectors.MetricResult, 1)
	errCh := make(chan error, 1)
	m := metric{Name: c.name}

	//run collector now
	go func() {
		res, err := c.collect()
		if err != nil {
			errCh <- err
			return
		}
		resCh <- res
	}()

	// wait for collector run
	select {
	case err := <-errCh:
		m.Err = err

	case result := <-resCh:
		for i := range result {
			if !isHidden(c.name) {
				result[i].Header = strings.TrimSpace(result[i].Header)
				result[i].Data = strings.TrimSpace(result[i].Data)
			}
			m.Data = append(m.Data, &metricResultCollector{MetricResult: *result[i]})
		}
		m.Frequency = c.frequency
		m.CollectionTime = time.Now()

	case <-time.After(c.timeout):
		m.Timeout = true
	}
	return m
}

func isHidden(name string) bool {
	return strings.HasPrefix(name, "hidden.")
}

// Generates a function with the Script.function func signature that returns statistical summary:
func createHistogramCollector(h gometrics.Histogram) func() ([]*collectors.MetricResult, error) {
	return func() ([]*collectors.MetricResult, error) {
		hsnap := h.Snapshot()
		qs := hsnap.Percentiles([]float64{0.25, 0.5, 0.75})
		result := &collectors.MetricResult{
			Header:   "count sum mean stddev min max q1 median q3",
			Metadata: nil,
			Data:     fmt.Sprintf("%v %v %v %v %v %v %v %v %v", hsnap.Count(), hsnap.Sum(), hsnap.Mean(), hsnap.StdDev(), hsnap.Min(), hsnap.Max(), qs[0], qs[1], qs[2]),
		}
		return []*collectors.MetricResult{result}, nil
	}
}

func (a *Agent) metadataString(metric, metadataKey, metadataValue string) (string, error) {
	pureMetricName := metric
	if strings.HasPrefix(metric, "smart") {
		pureMetricName = "smart"
	}

	a.metricCollectors.RLock()
	collector := a.metricCollectors.List[pureMetricName]
	a.metricCollectors.RUnlock()

	return formatMetadataString(metric, a.Config.NodeID, collector.frequency, metadataKey, metadataValue), nil
}

func formatMetadataString(metricName, nodeID string, frequency time.Duration, metadataKey, metadataValue string) string {
	return fmt.Sprintf(`:=:metadata %s %s %.0f %s %s`, metricName, nodeID, frequency.Seconds(), metadataKey, metadataValue)
}

//Sets a metadata name/value pair for a single metric or for ":all" metrics. Validates the name and metric parameter.
//The boolean paramatere `updateServer` controls whether to notify server of the change (false during initial load)
//Returns a boolean indicating whether a metadata value was changed.
func (a *Agent) setMetadata(metric, name, value string, updateServer bool) (changed bool, err error) {
	log.Infof("setting metadata %s %s %s", metric, name, value)
	//validate name parameter is non-empty and contains no whitespace characters
	if name == "" {
		return false, errors.New("Error setting metadata: empty `name` parameter")
	}
	for _, rune := range name {
		if unicode.IsSpace(rune) {
			return false, fmt.Errorf("Error setting metadata: `name` parameter (%q) contains whitespace character", name)
		}
	}
	if strings.ContainsRune(value, '\n') {
		return false, fmt.Errorf("Error setting metadata: `value` paraemeter (%q) contains newline", value)
	}

	//lock the metadata map then set for :all metrics or an individual metric
	a.metricCollectors.RLock()
	defer a.metricCollectors.RUnlock()
	if metric == ":all" {

		for metric := range a.metricCollectors.List {
			singleChange := a.setOneMetadata(metric, name, value, updateServer)
			if singleChange {
				changed = true
			}
		}
	} else {
		//make sure the metric exists before setting
		if _, ok := a.metricCollectors.List[metric]; !ok {
			return false, fmt.Errorf("Error seting metadata: metric %q is neither a recognized metric nor \":all\". ", metric)
		}
		changed = a.setOneMetadata(metric, name, value, updateServer)
	}

	return changed, nil
}

//Sets metadata for a single, already-validated metric and name
//Returns a boolean indicating whether the metadata value was changed.
func (a *Agent) setOneMetadata(metric, name, value string, updateServer bool) (changed bool) {
	a.metricMetadata.Lock()
	defer a.metricMetadata.Unlock()
	//get the metadata map for that metric, creating it if it's not yet present
	metadata := a.metricMetadata.Map[metric]
	if metadata == nil {
		a.metricMetadata.Map[metric] = make(map[string]string)
		metadata = a.metricMetadata.Map[metric]
	}
	oldValue := metadata[name]
	//if value to set is the empty string, delete the key from the map, else set
	if value == "" {
		delete(metadata, name)
	} else {
		metadata[name] = value
	}
	changed = oldValue != value
	if changed {
		if updateServer {
			data, err := a.metadataString(metric, name, value)
			if err != nil {
				log.Error(err.Error())
				return changed
			}
			a.NonBlockingSend([]byte(data))
		}
	}
	return changed
}
