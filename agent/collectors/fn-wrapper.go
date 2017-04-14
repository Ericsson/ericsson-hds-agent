package collectors

import (
	"encoding/json"
	"strings"

	"github.com/Ericsson/ericsson-hds-agent/agent/log"
)

// BuildMetricResult returns MetricResult from given header, metric etc
func BuildMetricResult(header, metric, sufix string, metadata map[string]string) *MetricResult {
	result := &MetricResult{
		Header:   header,
		Data:     metric,
		Metadata: nil,
		Sufix:    sufix,
	}
	if metadata != nil && len(metadata) > 0 {
		result.Metadata = metadata
	}
	return result
}

// ConvertToString converts MetricResults into string
func ConvertToString(result []*MetricResult) string {
	var out = []string{}
	for _, m := range result {
		out = append(out, "h:"+m.Sufix+" "+m.Header+"\n"+"v:"+m.Sufix+" "+m.Data)
		if m.Metadata != nil && len(m.Metadata) > 0 {
			marshMetadata, err := json.Marshal(m.Metadata)
			if err != nil {
				log.Errorf("Could not marshal metadata: %v", err)
				continue
			}
			out = append(out, "m:"+m.Sufix+" "+string(marshMetadata))
		}
	}

	return strings.Join(out, "\n")
}
