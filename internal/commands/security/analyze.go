// Copyright 2025 Tom Barlow
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package security

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/pkg/security"
	"github.com/tombee/conductor/pkg/workflow"
	"gopkg.in/yaml.v3"
)

func newSecurityAnalyzeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analyze <workflow>",
		Short: "Analyze a workflow's security requirements",
		Long: `Analyze a workflow file to determine its security requirements.

This command examines the workflow definition to identify:
- Required filesystem paths (read/write)
- Network hosts that will be contacted
- Commands that will be executed
- Compatibility with each security profile

Use this to understand what permissions a workflow needs before running it,
or to help create a custom security profile.

Example:
  conductor security analyze workflow.yaml
  conductor security analyze workflow.yaml --json`,
		Args: cobra.ExactArgs(1),
		RunE: runSecurityAnalyze,
	}

	return cmd
}

type analyzeOutput struct {
	WorkflowPath string                       `json:"workflow_path"`
	Requirements workflowRequirements         `json:"requirements"`
	Compatibility map[string]compatibilityInfo `json:"compatibility"`
	Recommendations []string                   `json:"recommendations,omitempty"`
}

type workflowRequirements struct {
	Filesystem filesystemRequirements `json:"filesystem"`
	Network    networkRequirements    `json:"network"`
	Commands   commandRequirements    `json:"commands"`
}

type filesystemRequirements struct {
	Reads  []pathRequirement `json:"reads,omitempty"`
	Writes []pathRequirement `json:"writes,omitempty"`
}

type pathRequirement struct {
	Path      string `json:"path"`
	Source    string `json:"source,omitempty"`
	Sensitive bool   `json:"sensitive,omitempty"`
}

type networkRequirements struct {
	Hosts []hostRequirement `json:"hosts,omitempty"`
}

type hostRequirement struct {
	Host   string `json:"host"`
	Source string `json:"source,omitempty"`
	Note   string `json:"note,omitempty"`
}

type commandRequirements struct {
	Commands []commandRequirement `json:"commands,omitempty"`
}

type commandRequirement struct {
	Command string `json:"command"`
	Source  string `json:"source,omitempty"`
	Note    string `json:"note,omitempty"`
}

type compatibilityInfo struct {
	Compatible bool     `json:"compatible"`
	Issues     []string `json:"issues,omitempty"`
}

func runSecurityAnalyze(cmd *cobra.Command, args []string) error {
	workflowPath := args[0]

	// Load workflow
	wf, err := loadWorkflowForAnalysis(workflowPath)
	if err != nil {
		return fmt.Errorf("failed to load workflow: %w", err)
	}

	// Analyze requirements
	reqs := analyzeWorkflowRequirements(wf, workflowPath)

	// Check compatibility with built-in profiles
	compat := checkProfileCompatibility(reqs)

	// Generate recommendations
	recommendations := generateRecommendations(reqs, compat)

	// Build output
	output := analyzeOutput{
		WorkflowPath:    workflowPath,
		Requirements:    reqs,
		Compatibility:   compat,
		Recommendations: recommendations,
	}

	// Output in requested format
	if shared.GetJSON() {
		return outputAnalyzeJSON(output)
	}

	return outputAnalyzeHuman(output)
}

func loadWorkflowForAnalysis(path string) (*workflow.Definition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var wf workflow.Definition
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, err
	}

	return &wf, nil
}

func analyzeWorkflowRequirements(wf *workflow.Definition, workflowPath string) workflowRequirements {
	reqs := workflowRequirements{
		Filesystem: filesystemRequirements{
			Reads:  []pathRequirement{},
			Writes: []pathRequirement{},
		},
		Network: networkRequirements{
			Hosts: []hostRequirement{},
		},
		Commands: commandRequirements{
			Commands: []commandRequirement{},
		},
	}

	// Get workflow directory for relative path resolution
	workflowDir := filepath.Dir(workflowPath)

	// Analyze each step
	for i, step := range wf.Steps {
		stepSource := fmt.Sprintf("step %d (%s)", i+1, step.ID)

		// Analyze builtin connector if specified
		if step.BuiltinConnector != "" {
			analyzeTool(step.BuiltinConnector, step.Inputs, stepSource, workflowDir, &reqs)
		}

		// Analyze inline prompts for potential file/network references
		if step.Prompt != "" {
			analyzePromptText(step.Prompt, stepSource, &reqs)
		}
	}

	// Analyze workflow-level custom functions
	for _, function := range wf.Functions {
		functionSource := fmt.Sprintf("custom function (%s)", function.Name)
		if function.Type == "http" && function.URL != "" {
			if host := extractHost(function.URL); host != "" {
				reqs.Network.Hosts = append(reqs.Network.Hosts, hostRequirement{
					Host:   host,
					Source: functionSource,
				})
			}
		}
	}

	return reqs
}

func analyzeTool(toolName string, inputs map[string]interface{}, source string, workflowDir string, reqs *workflowRequirements) {
	switch toolName {
	case "file.read", "file_read":
		if path, ok := inputs["path"].(string); ok {
			reqs.Filesystem.Reads = append(reqs.Filesystem.Reads, pathRequirement{
				Path:      resolvePath(path, workflowDir),
				Source:    source,
				Sensitive: isSensitivePath(path),
			})
		}

	case "file.write", "file_write":
		if path, ok := inputs["path"].(string); ok {
			reqs.Filesystem.Writes = append(reqs.Filesystem.Writes, pathRequirement{
				Path:      resolvePath(path, workflowDir),
				Source:    source,
				Sensitive: isSensitivePath(path),
			})
		}

	case "shell", "shell.exec":
		if command, ok := inputs["command"].(string); ok {
			cmd := extractBaseCommand(command)
			note := ""
			if isSensitiveCommand(cmd) {
				note = "potentially dangerous"
			}
			reqs.Commands.Commands = append(reqs.Commands.Commands, commandRequirement{
				Command: cmd,
				Source:  source,
				Note:    note,
			})
		}

	case "http.get", "http.post", "http_get", "http_post":
		if url, ok := inputs["url"].(string); ok {
			if host := extractHost(url); host != "" {
				note := ""
				if strings.Contains(host, "unknown") || !isKnownHost(host) {
					note = "unrecognized host - review before allowing"
				}
				reqs.Network.Hosts = append(reqs.Network.Hosts, hostRequirement{
					Host:   host,
					Source: source,
					Note:   note,
				})
			}
		}
	}
}

func analyzePromptText(prompt string, source string, reqs *workflowRequirements) {
	// Look for URLs in prompt text
	urlPattern := regexp.MustCompile(`https?://([a-zA-Z0-9.-]+\.[a-zA-Z]{2,})`)
	matches := urlPattern.FindAllStringSubmatch(prompt, -1)
	for _, match := range matches {
		if len(match) > 1 {
			host := match[1]
			reqs.Network.Hosts = append(reqs.Network.Hosts, hostRequirement{
				Host:   host,
				Source: source + " (in prompt)",
				Note:   "referenced in prompt text",
			})
		}
	}
}

func resolvePath(path string, workflowDir string) string {
	// Expand home directory
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}

	// Make absolute if relative
	if !filepath.IsAbs(path) {
		return filepath.Join(workflowDir, path)
	}

	return filepath.Clean(path)
}

func isSensitivePath(path string) bool {
	sensitivePaths := []string{
		".ssh", ".aws", ".gnupg", ".config/conductor/credentials",
		".env", "credentials", "secrets", ".npmrc", ".pypirc",
	}

	lowerPath := strings.ToLower(path)
	for _, sensitive := range sensitivePaths {
		if strings.Contains(lowerPath, sensitive) {
			return true
		}
	}
	return false
}

func extractBaseCommand(command string) string {
	// Extract first word as base command
	parts := strings.Fields(command)
	if len(parts) > 0 {
		return parts[0]
	}
	return command
}

func isSensitiveCommand(cmd string) bool {
	sensitive := []string{"sudo", "rm", "chmod", "chown", "curl", "wget", "nc", "netcat"}
	for _, s := range sensitive {
		if cmd == s || strings.HasPrefix(cmd, s+" ") {
			return true
		}
	}
	return false
}

func extractHost(urlStr string) string {
	// Simple host extraction from URL
	urlStr = strings.TrimPrefix(urlStr, "http://")
	urlStr = strings.TrimPrefix(urlStr, "https://")
	if idx := strings.Index(urlStr, "/"); idx != -1 {
		urlStr = urlStr[:idx]
	}
	if idx := strings.Index(urlStr, ":"); idx != -1 {
		urlStr = urlStr[:idx]
	}
	return urlStr
}

func isKnownHost(host string) bool {
	knownHosts := []string{
		"api.anthropic.com", "api.openai.com", "github.com", "api.github.com",
		"registry.npmjs.org", "pypi.org",
	}
	for _, known := range knownHosts {
		if host == known {
			return true
		}
	}
	return false
}

func checkProfileCompatibility(reqs workflowRequirements) map[string]compatibilityInfo {
	compat := make(map[string]compatibilityInfo)

	profiles := []string{
		security.ProfileUnrestricted,
		security.ProfileStandard,
		security.ProfileStrict,
		security.ProfileAirGapped,
	}

	for _, profileName := range profiles {
		profile, err := security.LoadProfile(profileName, nil)
		if err != nil {
			continue
		}

		issues := checkProfileIssues(profile, reqs)
		compat[profileName] = compatibilityInfo{
			Compatible: len(issues) == 0,
			Issues:     issues,
		}
	}

	return compat
}

func checkProfileIssues(profile *security.SecurityProfile, reqs workflowRequirements) []string {
	var issues []string

	// Check filesystem requirements
	for _, read := range reqs.Filesystem.Reads {
		if !isPathAllowed(read.Path, profile.Filesystem.Read, profile.Filesystem.Deny) {
			issues = append(issues, fmt.Sprintf("filesystem read: %s", read.Path))
		}
	}
	for _, write := range reqs.Filesystem.Writes {
		if !isPathAllowed(write.Path, profile.Filesystem.Write, profile.Filesystem.Deny) {
			issues = append(issues, fmt.Sprintf("filesystem write: %s", write.Path))
		}
	}

	// Check network requirements
	if profile.Network.DenyAll && len(reqs.Network.Hosts) > 0 {
		issues = append(issues, fmt.Sprintf("network: %d hosts required but network disabled", len(reqs.Network.Hosts)))
	} else {
		for _, host := range reqs.Network.Hosts {
			if !isHostAllowed(host.Host, profile.Network.Allow) {
				issues = append(issues, fmt.Sprintf("network: %s", host.Host))
			}
		}
	}

	// Check command requirements
	for _, cmd := range reqs.Commands.Commands {
		if !isCommandAllowed(cmd.Command, profile.Execution.AllowedCommands, profile.Execution.DeniedCommands) {
			issues = append(issues, fmt.Sprintf("command: %s", cmd.Command))
		}
	}

	return issues
}

func isPathAllowed(path string, allowed []string, denied []string) bool {
	// Empty allowlist means unrestricted
	if len(allowed) == 0 && len(denied) == 0 {
		return true
	}

	// Check deny list first
	for _, deny := range denied {
		if matchesPath(path, deny) {
			return false
		}
	}

	// If allowlist is empty, allow by default (unless denied)
	if len(allowed) == 0 {
		return true
	}

	// Check allow list
	for _, allow := range allowed {
		if matchesPath(path, allow) {
			return true
		}
	}

	return false
}

func matchesPath(path, pattern string) bool {
	// Simple prefix match for now
	// Real implementation would handle wildcards and symlinks
	return strings.HasPrefix(path, pattern) || strings.HasPrefix(pattern, path)
}

func isHostAllowed(host string, allowed []string) bool {
	if len(allowed) == 0 {
		return true // Unrestricted
	}

	for _, allow := range allowed {
		if host == allow || strings.HasSuffix(host, "."+allow) {
			return true
		}
	}
	return false
}

func isCommandAllowed(cmd string, allowed []string, denied []string) bool {
	// Check deny list first
	for _, deny := range denied {
		if cmd == deny || strings.HasPrefix(cmd, deny+" ") {
			return false
		}
	}

	// Empty allowlist means unrestricted
	if len(allowed) == 0 {
		return true
	}

	// Check allow list
	for _, allow := range allowed {
		if cmd == allow || strings.HasPrefix(cmd, allow+" ") {
			return true
		}
	}

	return false
}

func generateRecommendations(reqs workflowRequirements, compat map[string]compatibilityInfo) []string {
	var recommendations []string

	// Check for sensitive paths
	for _, read := range reqs.Filesystem.Reads {
		if read.Sensitive {
			recommendations = append(recommendations, fmt.Sprintf("Sensitive path read: %s - ensure this is necessary", read.Path))
		}
	}

	// Check for unrecognized hosts
	for _, host := range reqs.Network.Hosts {
		if host.Note != "" && strings.Contains(host.Note, "unrecognized") {
			recommendations = append(recommendations, fmt.Sprintf("Unrecognized host: %s - verify before allowing", host.Host))
		}
	}

	// Check for dangerous commands
	for _, cmd := range reqs.Commands.Commands {
		if cmd.Note != "" && strings.Contains(cmd.Note, "dangerous") {
			recommendations = append(recommendations, fmt.Sprintf("Potentially dangerous command: %s - review usage", cmd.Command))
		}
	}

	// Suggest appropriate profile
	if compat[security.ProfileStandard].Compatible {
		recommendations = append(recommendations, "Compatible with 'standard' profile (recommended)")
	} else if compat[security.ProfileUnrestricted].Compatible {
		recommendations = append(recommendations, "Requires 'unrestricted' profile - consider refactoring for better security")
	}

	return recommendations
}

func outputAnalyzeJSON(output analyzeOutput) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

func outputAnalyzeHuman(output analyzeOutput) error {
	fmt.Printf("Security Analysis: %s\n", output.WorkflowPath)
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()

	// Requirements
	fmt.Println("Current Requirements:")
	fmt.Println()

	if len(output.Requirements.Filesystem.Reads) > 0 || len(output.Requirements.Filesystem.Writes) > 0 {
		fmt.Println("  Filesystem:")
		if len(output.Requirements.Filesystem.Reads) > 0 {
			fmt.Println("    Reads:")
			for _, r := range output.Requirements.Filesystem.Reads {
				sensitive := ""
				if r.Sensitive {
					sensitive = " (sensitive)"
				}
				fmt.Printf("      - %s%s\n", r.Path, sensitive)
			}
		}
		if len(output.Requirements.Filesystem.Writes) > 0 {
			fmt.Println("    Writes:")
			for _, w := range output.Requirements.Filesystem.Writes {
				fmt.Printf("      - %s\n", w.Path)
			}
		}
		fmt.Println()
	}

	if len(output.Requirements.Network.Hosts) > 0 {
		fmt.Println("  Network:")
		for _, h := range output.Requirements.Network.Hosts {
			note := ""
			if h.Note != "" {
				note = fmt.Sprintf(" (%s)", h.Note)
			}
			fmt.Printf("    - %s%s\n", h.Host, note)
		}
		fmt.Println()
	}

	if len(output.Requirements.Commands.Commands) > 0 {
		fmt.Println("  Commands:")
		for _, c := range output.Requirements.Commands.Commands {
			note := ""
			if c.Note != "" {
				note = fmt.Sprintf(" (%s)", c.Note)
			}
			fmt.Printf("    - %s%s\n", c.Command, note)
		}
		fmt.Println()
	}

	// Compatibility
	fmt.Println("Profile Compatibility:")
	for _, profile := range []string{
		security.ProfileUnrestricted,
		security.ProfileStandard,
		security.ProfileStrict,
		security.ProfileAirGapped,
	} {
		if compat, ok := output.Compatibility[profile]; ok {
			status := "COMPATIBLE"
			if !compat.Compatible {
				status = "INCOMPATIBLE"
				if len(compat.Issues) > 0 {
					status += fmt.Sprintf(" (%d issues)", len(compat.Issues))
				}
			}
			fmt.Printf("  %-15s %s\n", profile+":", status)
			if !compat.Compatible && len(compat.Issues) <= 3 {
				for _, issue := range compat.Issues {
					fmt.Printf("    - %s\n", issue)
				}
			}
		}
	}
	fmt.Println()

	// Recommendations
	if len(output.Recommendations) > 0 {
		fmt.Println("Recommendations:")
		for i, rec := range output.Recommendations {
			fmt.Printf("  %d. %s\n", i+1, rec)
		}
	}

	return nil
}
