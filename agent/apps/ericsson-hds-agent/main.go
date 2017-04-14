package main

import (
	"flag"
	"os"

	"github.com/Ericsson/ericsson-hds-agent/agent"
)

//this is the default behavior for agent
func main() {
	config := agent.NewDefaultConfig()
	agent.InitFlags(config)
	flag.Parse()
	context := &agent.Agent{Config: config}

	err := context.Initialize()
	if err != nil {
		os.Exit(1)
	}
	agent.Start(context)
}
