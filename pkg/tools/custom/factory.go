package custom

import (
	"fmt"

	"github.com/tombee/conductor/pkg/tools"
	"github.com/tombee/conductor/pkg/workflow"
)

// NewToolFromDefinition creates a custom tool from a workflow function definition.
// The workflowDir parameter is used to resolve relative script paths.
func NewToolFromDefinition(def workflow.FunctionDefinition, workflowDir string) (tools.Tool, error) {
	switch def.Type {
	case workflow.ToolTypeHTTP:
		return NewHTTPCustomTool(def)
	case workflow.ToolTypeScript:
		return NewScriptCustomTool(def, workflowDir)
	default:
		return nil, fmt.Errorf("unsupported function type: %s", def.Type)
	}
}
