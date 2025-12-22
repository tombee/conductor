package custom

import (
	"os"
	"strings"
)

// envToMap converts environment variables to a map for template interpolation.
func envToMap() map[string]string {
	env := make(map[string]string)
	for _, e := range os.Environ() {
		pair := strings.SplitN(e, "=", 2)
		if len(pair) == 2 {
			env[pair[0]] = pair[1]
		}
	}
	return env
}
