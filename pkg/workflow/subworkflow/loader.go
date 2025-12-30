// Package subworkflow provides sub-workflow loading, validation, and caching.
package subworkflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tombee/conductor/pkg/workflow"
)

func init() {
	// Register the subworkflow loader factory with the workflow package
	// This avoids import cycles while allowing the executor to create loaders
	workflow.SetDefaultSubworkflowLoaderFactory(func() workflow.SubworkflowLoader {
		return &loaderAdapter{loader: NewLoader()}
	})
}

// loaderAdapter adapts the Loader to the workflow.SubworkflowLoader interface.
type loaderAdapter struct {
	loader *Loader
}

func (a *loaderAdapter) Load(parentDir string, path string, ctx interface{}) (*workflow.Definition, error) {
	// Convert ctx to *LoadContext if provided
	var loadCtx *LoadContext
	if ctx != nil {
		if lc, ok := ctx.(*LoadContext); ok {
			loadCtx = lc
		}
	}
	return a.loader.Load(parentDir, path, loadCtx)
}

// MaxNestingDepth is the maximum allowed nesting depth for sub-workflows.
// This prevents deeply nested workflows that could cause performance issues.
const MaxNestingDepth = 5

// Loader handles loading and caching of sub-workflow definitions.
// It provides security validation, recursion detection, and definition caching.
type Loader struct {
	// cache stores loaded workflow definitions keyed by absolute path
	cache map[string]*cacheEntry
	// mu protects concurrent access to the cache
	mu sync.RWMutex
}

// cacheEntry stores a cached workflow definition with its modification time.
type cacheEntry struct {
	definition *workflow.Definition
	modTime    time.Time
}

// LoadContext tracks the loading context for recursion and depth detection.
type LoadContext struct {
	// callStack tracks the workflow files currently being loaded (for cycle detection)
	callStack []string
	// depth tracks the current nesting depth
	depth int
}

// NewLoader creates a new sub-workflow loader.
func NewLoader() *Loader {
	return &Loader{
		cache: make(map[string]*cacheEntry),
	}
}

// Load loads a sub-workflow definition from the given path relative to parentDir.
// It performs security validation, recursion detection, and caching.
//
// Parameters:
//   - parentDir: The directory containing the parent workflow file
//   - path: The relative path to the sub-workflow file
//   - ctx: The load context for tracking recursion and depth
//
// Returns the loaded workflow definition or an error.
func (l *Loader) Load(parentDir string, path string, ctx *LoadContext) (*workflow.Definition, error) {
	// Validate the path is safe
	if err := workflow.ValidateWorkflowPath(path); err != nil {
		return nil, fmt.Errorf("invalid workflow path: %w", err)
	}

	// Resolve the absolute path
	absPath := filepath.Join(parentDir, path)
	absPath, err := filepath.Abs(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Get the absolute parent directory for comparison
	absParentDir, err := filepath.Abs(parentDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve parent directory: %w", err)
	}

	// Security check: Ensure resolved path doesn't escape the parent directory
	relPath, err := filepath.Rel(absParentDir, absPath)
	if err != nil || filepath.IsAbs(relPath) || len(relPath) >= 3 && relPath[:3] == ".."+string(filepath.Separator) {
		return nil, fmt.Errorf("workflow path escapes parent directory: %s", path)
	}

	// Check for symlinks within the user-provided path (not the base directory)
	// We only check the components added by the user's path, not the parent directory
	if err := checkNoSymlinksInRelativePath(absParentDir, relPath); err != nil {
		return nil, fmt.Errorf("workflow path contains symlinks: %w", err)
	}

	// Initialize context if nil
	if ctx == nil {
		ctx = &LoadContext{
			callStack: []string{},
			depth:     0,
		}
	}

	// Check depth limit
	if ctx.depth >= MaxNestingDepth {
		return nil, fmt.Errorf("maximum nesting depth (%d) exceeded: %s", MaxNestingDepth, path)
	}

	// Check for recursion (cycle detection)
	for _, stackPath := range ctx.callStack {
		if stackPath == absPath {
			return nil, fmt.Errorf("recursion detected: %s -> %s", formatCallStack(ctx.callStack), path)
		}
	}

	// Try to load from cache
	def, err := l.loadFromCache(absPath)
	if err == nil && def != nil {
		return def, nil
	}

	// Update context for this load
	newCtx := &LoadContext{
		callStack: append(ctx.callStack, absPath),
		depth:     ctx.depth + 1,
	}

	// Load the file
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read workflow file: %w", err)
	}

	// Parse the workflow definition
	def, err = workflow.ParseDefinition(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse workflow: %w", err)
	}

	// Recursively load any sub-workflows referenced by this workflow
	subWorkflowDir := filepath.Dir(absPath)
	for _, step := range def.Steps {
		if step.Type == workflow.StepTypeWorkflow && step.Workflow != "" {
			// Validate that the sub-workflow can be loaded (recursion check)
			_, err := l.Load(subWorkflowDir, step.Workflow, newCtx)
			if err != nil {
				return nil, fmt.Errorf("failed to load sub-workflow %s: %w", step.Workflow, err)
			}
		}
	}

	// Cache the definition
	l.storeInCache(absPath, def)

	return def, nil
}

// loadFromCache attempts to load a workflow definition from cache.
// Returns nil if not cached or if the file has been modified since caching.
func (l *Loader) loadFromCache(absPath string) (*workflow.Definition, error) {
	l.mu.RLock()
	entry, exists := l.cache[absPath]
	l.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("not in cache")
	}

	// Check if file has been modified
	info, err := os.Stat(absPath)
	if err != nil {
		// File no longer exists or can't be stat'd, invalidate cache
		l.mu.Lock()
		delete(l.cache, absPath)
		l.mu.Unlock()
		return nil, fmt.Errorf("cached file no longer accessible")
	}

	// If modification time changed, invalidate cache
	if !info.ModTime().Equal(entry.modTime) {
		l.mu.Lock()
		delete(l.cache, absPath)
		l.mu.Unlock()
		return nil, fmt.Errorf("cached file has been modified")
	}

	return entry.definition, nil
}

// storeInCache stores a workflow definition in the cache.
func (l *Loader) storeInCache(absPath string, def *workflow.Definition) {
	info, err := os.Stat(absPath)
	if err != nil {
		// Can't determine modtime, don't cache
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.cache[absPath] = &cacheEntry{
		definition: def,
		modTime:    info.ModTime(),
	}
}

// checkNoSymlinksInRelativePath verifies that no component within the relative path is a symlink.
// This prevents symlink attacks that could bypass path validation.
// It only checks components within relPath, not the baseDir itself.
func checkNoSymlinksInRelativePath(baseDir, relPath string) error {
	// If relPath is ".", there's nothing to check
	if relPath == "." {
		return nil
	}

	// Split the relative path into components
	components := strings.Split(filepath.Clean(relPath), string(filepath.Separator))

	// Check each component incrementally
	current := baseDir
	for _, component := range components {
		if component == "" || component == "." {
			continue
		}

		current = filepath.Join(current, component)

		// Check if this component is a symlink
		info, err := os.Lstat(current)
		if err != nil {
			// If file doesn't exist yet, that's okay (we'll catch it later)
			if os.IsNotExist(err) {
				continue
			}
			return err
		}

		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("path contains symlink: %s", current)
		}
	}

	return nil
}

// formatCallStack formats the call stack for error messages.
func formatCallStack(stack []string) string {
	if len(stack) == 0 {
		return ""
	}

	result := filepath.Base(stack[0])
	for i := 1; i < len(stack); i++ {
		result += " -> " + filepath.Base(stack[i])
	}
	return result
}
