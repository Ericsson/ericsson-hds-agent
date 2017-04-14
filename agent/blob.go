package agent

import "encoding/json"

// Format encodes blob into JSON format
func (b *Blob) Format() []byte {
	rjson, err := json.Marshal(b)
	if err == nil {
		return rjson
	}
	return nil
}
