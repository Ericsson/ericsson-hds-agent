package agent

import (
	"time"
)

const (
	invFrequency     = 30 * time.Minute
	collectorTimeout = 30
	timeoutConnSend  = time.Second
	dialTimeout      = time.Second
	failureLimit     = 5

	intrptChSize = 10

	protoTCP = "tcp"
)
