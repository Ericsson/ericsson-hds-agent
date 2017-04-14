package agent

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Ericsson/ericsson-hds-agent/agent/log"
)

func handleInventoryCollection(c *InventoryCollector) Inventory {
	resCh := make(chan []byte, 1)
	errCh := make(chan error, 1)
	go func() {
		res, err := c.collect()
		if err != nil {
			errCh <- err
			return
		}
		resCh <- res
	}()
	blobtype := "unknown"
	switch c.collectorType {
	case userScript:
		blobtype = "inventory.user"
	case builtIn:
		blobtype = "inventory.all"
	default:
		blobtype = c.collectorType
	}
	inv := Inventory{Name: c.name, Type: blobtype}
	select {
	case err := <-errCh:
		inv.err = err
	case data := <-resCh:
		inv.Data = json.RawMessage(data)
	case <-time.After(c.timeout):
		inv.Timeout = true
	}
	return inv
}

func (a *Agent) runInvCollectors(sha1cache map[string]string, cType string, cNames []string, forceRun bool) error {
	results := make([]Inventory, 0)
	var keys []string

	a.inventoryCollectors.RLock()
	for k, col := range a.inventoryCollectors.List {
		switch {
		case len(cNames) > 0 && contains(cNames, k):
			keys = append(keys, k)
		case cType == "":
			keys = append(keys, k)
		case cType == col.collectorType:
			keys = append(keys, k)
		}
	}

	sort.Strings(keys)

	for _, k := range keys {
		collector := a.inventoryCollectors.List[k]
		if forceRun || collector.state == runningState {
			inv := handleInventoryCollection(collector)
			if len(cNames) > 0 {
				inv.Type = cType
			}
			results = append(results, inv)
		}
	}
	a.inventoryCollectors.RUnlock()
	return a.ProcessInv(sha1cache, results)
}

// ProcessInv collects, formats, and sends inventory.
func (a *Agent) ProcessInv(sha1cache map[string]string, inventoryResults []Inventory) error {
	log.Info("Processing inventory set")
	types := make(map[string]map[string]*json.RawMessage)

	a.inventoryCollectors.RLock()
	defer a.inventoryCollectors.RUnlock()
	for _, inventory := range inventoryResults {
		invCol := a.inventoryCollectors.List[inventory.Name]
		var inventoryKey string

		// Avoid consolidating the sysinfo.package sub-keys because the system
		// could have more than one package manager installed
		if strings.HasPrefix(inventory.Name, "sysinfo.package") {
			inventoryKey = inventory.Name
		} else if nameparts := strings.Split(inventory.Name, "."); inventory.Type == "inventory.all" && len(nameparts) == 3 {
			inventoryKey = nameparts[0] + "." + nameparts[2]
		} else {
			inventoryKey = inventory.Name
		}

		// Was it a timeout or failure?
		switch {
		// Error while collecting
		case inventory.err != nil:
			log.Errorf("Error collecting inventory %v: %v.", inventory.Name, inventory.err)
			invCol.numErrs++
			if invCol.numErrs >= a.ErrorLimit {
				log.Infof("Inventory Collector %v has reached max number of errors and will not be collected.", inventory.Name)
				invCol.state = stopState
			}

		// Timeout while collecting
		case inventory.Timeout:
			log.Errorf("Timeout when collecting inventory %v.", inventory.Name)
			invCol.numTimeout++
			if invCol.numTimeout >= a.TimeoutLimit {
				log.Infof("Inventory Collector %v has reached max number of timeouts and will not be collected.", inventory.Name)
				invCol.state = stopState
			}

		default:
			//Reset Error and Timeout Counters
			invCol.numTimeout = 0
			invCol.numErrs = 0

			if types[inventory.Type] == nil {
				types[inventory.Type] = make(map[string]*json.RawMessage)
			}
			if _, ok := types[inventory.Type][inventoryKey]; !ok { //Is it really need save first and skip next data for same keys
				rjsonRaw := new(json.RawMessage)
				*rjsonRaw = inventory.Data
				types[inventory.Type][inventoryKey] = rjsonRaw
			}
		}
	}

	if len(types) == 0 {
		return fmt.Errorf("none of the specified collectors cannot be processed, for additional info refer to the agent's log")
	}

	for key, value := range types {
		if len(value) == 0 {
			continue
		}
		final, err := json.Marshal(value)
		if err != nil {
			log.Errorf("Error marshalling JSON object %v", err)
			continue
		}
		h := sha1.New()
		h.Write(final)
		sha1 := fmt.Sprintf("%x", h.Sum(nil))
		if sha1cache[key] == sha1 {
			log.Infof("Sha1 %v is cached for %v, skipping send", sha1, key)
			continue
		}
		sha1cache[key] = sha1
		timestamp := fmt.Sprintf("%d", time.Now().Unix())
		blob := Blob{Type: key, NodeID: a.Config.NodeID, ID: a.ID, Content: final, Digest: sha1, Timestamp: timestamp}
		a.ID++
		a.NonBlockingSend(blob.Format())
	}
	return nil
}

// scheduleInventory collects output from inventory collectors
func (a *Agent) scheduleInventory() {
	var invTicker = &time.Ticker{}
	sha1cache := make(map[string]string)

	log.Infof("starting Inventory collector")

	if a.InvFrequency > 0 {
		invTicker = time.NewTicker(a.InvFrequency)
	} else {
		log.Infof("running inventory collector once (frequency <= 0)")
	}

	go func() {
		for {
			select {
			case <-invTicker.C:
				// run inventory collector now
				a.runInvCollectors(sha1cache, "", nil, false)
			}
		}
	}()
}

func (a *Agent) sendInventory() error {
	sha1cache := make(map[string]string)
	a.runInvCollectors(sha1cache, "", nil, false)
	return nil
}
