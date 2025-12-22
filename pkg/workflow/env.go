package workflow

import (
	"os"
	"strings"
)

// LoadEnvironment loads environment variables into a map.
// This map can be added to TemplateContext for {{.env.VAR_NAME}} interpolation.
func LoadEnvironment() map[string]string {
	env := make(map[string]string)
	for _, e := range os.Environ() {
		pair := strings.SplitN(e, "=", 2)
		if len(pair) == 2 {
			env[pair[0]] = pair[1]
		}
	}
	return env
}
