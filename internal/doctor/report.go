package doctor

import (
	"encoding/json"
	"fmt"
)

// PrintReport outputs the check results.
func PrintReport(checks []Check, asJSON bool) {
	if asJSON {
		data, _ := json.MarshalIndent(map[string]interface{}{
			"ok":     len(checks) == 0,
			"checks": checks,
		}, "", "  ")
		fmt.Println(string(data))
		return
	}
	for _, c := range checks {
		fmt.Printf("[%s] %s: %s\n", c.Status, c.ID, c.Message)
	}
}
