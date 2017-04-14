package collectors

// CollectorRunner returns inventory by calling run function of invetory collector
type CollectorRunner func() ([]byte, error)

// MetricRunner returns metric by calling run function of metric collector
type MetricRunner func() ([]*MetricResult, error)

// CollectorPrecheck is precheck function for metric or inventory collectors
type CollectorPrecheck func() error

// CollectorFnWrapper is wrapper struct for inventory collectors
type CollectorFnWrapper struct {
	RunFn        CollectorRunner
	PrecheckFn   CollectorPrecheck
	Dependencies []string
	Type         string
}

// MetricFnWrapper is wrapper struct for metric collectors
type MetricFnWrapper struct {
	RunFn        MetricRunner
	PrecheckFn   CollectorPrecheck
	Dependencies []string
	Type         string
}

// MetricResult represents a metric (example cpu)
type MetricResult struct {
	Header   string
	Data     string
	Sufix    string
	Metadata map[string]string
}
