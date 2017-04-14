package agent

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/Ericsson/ericsson-hds-agent/agent/log"
)

const (
	runningState = "running"
	stopState    = "stopped"
)

/**
* User Scripts
 */

const (
	userScript = "userScript"
	builtIn    = "builtIn"
)

// Precheck validates dependencies needed for collection by collectors
func (u *BaseCollector) Precheck(a *Agent) error {
	var missDeps []string
	for _, dep := range u.dependencies {
		_, err := exec.LookPath(dep)
		if err != nil {
			missDeps = append(missDeps, dep)
		}
	}
	if len(missDeps) > 0 {
		errorDeps := strings.Join(missDeps, ", ")
		er := fmt.Errorf("Collector %s will not run because miss dependency: %s", u.name, errorDeps)
		log.Error(er.Error())
		return er
	}
	if u.precheck == nil {
		return nil
	}
	err := u.precheck()
	if err != nil {
		er := fmt.Errorf("Collector %s will not run because it failed precheck: %v", u.name, err)
		log.Error(er.Error())
		return er
	}
	return nil
}
