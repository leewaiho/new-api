package dto

import (
	"fmt"
	"strings"
)

const (
	AdvancedCustomResponsesToolPolicySourceProtected     = "protected"
	AdvancedCustomResponsesToolPolicySourceModelToolName = "model_tool_name"
	AdvancedCustomResponsesToolPolicySourceModelToolType = "model_tool_type"
	AdvancedCustomResponsesToolPolicySourceRoute         = "route"
	AdvancedCustomResponsesToolPolicySourceSystemDefault = "system_default"

	AdvancedCustomResponsesToolMissingOptionsPreserve         = "preserve"
	AdvancedCustomResponsesToolMissingOptionsPopulateDefaults = "populate_defaults"
)

type AdvancedCustomResponsesToolPolicyResolution struct {
	Policy       string
	Source       string
	MatchedModel string
}

func ResolveAdvancedCustomResponsesToolConflictPolicy(options *AdvancedCustomConverterOptions) string {
	if options == nil {
		return AdvancedCustomResponsesToolConflictPolicyDeduplicate
	}
	policy := strings.TrimSpace(options.ResponsesToolConflictPolicy)
	if policy == "" {
		return AdvancedCustomResponsesToolConflictPolicyDeduplicate
	}
	return policy
}

func ResolveAdvancedCustomResponsesImplicitHostedTools(
	options *AdvancedCustomConverterOptions,
	requestedModel string,
	upstreamModel string,
) []string {
	if options == nil {
		return nil
	}
	values := [][]string{options.ResponsesImplicitHostedTools}
	if override, _, ok := matchAdvancedCustomResponsesToolModelOverride(options, requestedModel, upstreamModel); ok {
		values = append(values, override.ResponsesImplicitHostedTools)
	}
	seen := make(map[string]struct{})
	resolved := make([]string, 0)
	for _, group := range values {
		for _, raw := range group {
			capability := normalizeAdvancedCustomResponsesHostedCapability(raw)
			if capability == "" {
				continue
			}
			if _, exists := seen[capability]; exists {
				continue
			}
			seen[capability] = struct{}{}
			resolved = append(resolved, capability)
		}
	}
	return resolved
}

func normalizeAdvancedCustomResponsesHostedCapability(capability string) string {
	switch strings.TrimSpace(capability) {
	case "image_gen", "image_generation":
		return "image_generation"
	case "web_search", "web_search_preview":
		return "web_search"
	default:
		return strings.TrimSpace(capability)
	}
}

func ResolveAdvancedCustomResponsesWebSearchParameters(
	options *AdvancedCustomConverterOptions,
	requestedModel string,
	upstreamModel string,
) *AdvancedCustomWebSearchParameterCompatibility {
	if options == nil {
		return nil
	}
	if override, _, ok := matchAdvancedCustomResponsesToolModelOverride(options, requestedModel, upstreamModel); ok &&
		override.ResponsesToolParameters != nil && override.ResponsesToolParameters.WebSearch != nil {
		return override.ResponsesToolParameters.WebSearch
	}
	if options.ResponsesToolParameters == nil {
		return nil
	}
	return options.ResponsesToolParameters.WebSearch
}

func ResolveAdvancedCustomResponsesToolPolicy(
	options *AdvancedCustomConverterOptions,
	requestedModel string,
	upstreamModel string,
	toolType string,
	toolName string,
) AdvancedCustomResponsesToolPolicyResolution {
	if normalizeAdvancedCustomResponsesToolType(toolType) == "function" {
		return AdvancedCustomResponsesToolPolicyResolution{
			Policy: AdvancedCustomResponsesToolPolicyPreserve,
			Source: AdvancedCustomResponsesToolPolicySourceProtected,
		}
	}

	if override, matchedModel, ok := matchAdvancedCustomResponsesToolModelOverride(options, requestedModel, upstreamModel); ok {
		for _, namePolicy := range override.ToolNames {
			if advancedCustomResponsesToolNamePolicyMatches(namePolicy, toolType, toolName) {
				return AdvancedCustomResponsesToolPolicyResolution{
					Policy:       strings.TrimSpace(namePolicy.Policy),
					Source:       AdvancedCustomResponsesToolPolicySourceModelToolName,
					MatchedModel: matchedModel,
				}
			}
		}
		if policy, ok := advancedCustomResponsesToolPolicyForType(override.ResponsesTools, toolType); ok {
			return AdvancedCustomResponsesToolPolicyResolution{
				Policy:       policy,
				Source:       AdvancedCustomResponsesToolPolicySourceModelToolType,
				MatchedModel: matchedModel,
			}
		}
	}

	if policy, ok := advancedCustomResponsesToolPolicyForType(nilSafeResponsesTools(options), toolType); ok {
		return AdvancedCustomResponsesToolPolicyResolution{
			Policy: policy,
			Source: AdvancedCustomResponsesToolPolicySourceRoute,
		}
	}
	if policy, ok := advancedCustomLegacyResponsesToolPolicy(options, toolType); ok {
		return AdvancedCustomResponsesToolPolicyResolution{
			Policy: policy,
			Source: AdvancedCustomResponsesToolPolicySourceRoute,
		}
	}
	return AdvancedCustomResponsesToolPolicyResolution{
		Policy: AdvancedCustomResponsesToolPolicyPreserve,
		Source: AdvancedCustomResponsesToolPolicySourceSystemDefault,
	}
}

func nilSafeResponsesTools(options *AdvancedCustomConverterOptions) *AdvancedCustomResponsesToolsOptions {
	if options == nil {
		return nil
	}
	return options.ResponsesTools
}

func matchAdvancedCustomResponsesToolModelOverride(
	options *AdvancedCustomConverterOptions,
	requestedModel string,
	upstreamModel string,
) (AdvancedCustomResponsesToolModelOverride, string, bool) {
	if options == nil {
		return AdvancedCustomResponsesToolModelOverride{}, "", false
	}
	requestedModel = strings.TrimSpace(requestedModel)
	upstreamModel = strings.TrimSpace(upstreamModel)
	if override, ok := findAdvancedCustomResponsesToolModelOverride(options, requestedModel); ok {
		return override, requestedModel, true
	}
	if upstreamModel == requestedModel {
		return AdvancedCustomResponsesToolModelOverride{}, "", false
	}
	if override, ok := findAdvancedCustomResponsesToolModelOverride(options, upstreamModel); ok {
		return override, upstreamModel, true
	}
	return AdvancedCustomResponsesToolModelOverride{}, "", false
}

func findAdvancedCustomResponsesToolModelOverride(options *AdvancedCustomConverterOptions, modelName string) (AdvancedCustomResponsesToolModelOverride, bool) {
	if modelName == "" {
		return AdvancedCustomResponsesToolModelOverride{}, false
	}
	for _, override := range options.ResponsesToolModelOverrides {
		for _, rawModel := range override.Models {
			model := strings.TrimSpace(rawModel)
			if model == modelName {
				return override, true
			}
		}
	}
	return AdvancedCustomResponsesToolModelOverride{}, false
}

func advancedCustomResponsesToolNamePolicyMatches(policy AdvancedCustomResponsesToolNamePolicy, toolType string, toolName string) bool {
	configuredType := strings.TrimSpace(policy.ToolType)
	actualType := strings.TrimSpace(toolType)
	if strings.TrimSpace(policy.ToolName) == "" {
		return IsAdvancedCustomUnnamedNativeResponsesToolType(configuredType) && configuredType == actualType
	}
	if strings.TrimSpace(policy.ToolName) != strings.TrimSpace(toolName) {
		return false
	}
	if configuredType == actualType {
		return true
	}
	configuredNormalized := normalizeAdvancedCustomResponsesToolType(configuredType)
	actualNormalized := normalizeAdvancedCustomResponsesToolType(actualType)
	if configuredNormalized != actualNormalized {
		return false
	}
	if configuredNormalized != "unknown" {
		return true
	}
	return configuredType == "unknown"
}

func normalizeAdvancedCustomResponsesToolType(toolType string) string {
	switch strings.TrimSpace(toolType) {
	case "function":
		return "function"
	case "namespace":
		return "namespace"
	case "custom":
		return "custom"
	case "web_search", "web_search_preview":
		return "web_search"
	case "tool_search":
		return "tool_search"
	case "image_gen", "image_generation":
		return "image_generation"
	default:
		return "unknown"
	}
}

func IsAdvancedCustomUnnamedNativeResponsesToolType(toolType string) bool {
	toolType = strings.TrimSpace(toolType)
	return toolType != "" && toolType != "unknown" && normalizeAdvancedCustomResponsesToolType(toolType) == "unknown"
}

func advancedCustomResponsesToolPolicyForType(options *AdvancedCustomResponsesToolsOptions, toolType string) (string, bool) {
	if options == nil {
		return "", false
	}
	var policy string
	switch normalizeAdvancedCustomResponsesToolType(toolType) {
	case "namespace":
		policy = options.Namespace
	case "custom":
		policy = options.Custom
	case "web_search":
		policy = options.WebSearch
	case "tool_search":
		policy = options.ToolSearch
	case "image_generation":
		policy = options.ImageGeneration
	case "unknown":
		policy = options.Unknown
	}
	policy = strings.TrimSpace(policy)
	return policy, policy != ""
}

func advancedCustomLegacyResponsesToolPolicy(options *AdvancedCustomConverterOptions, toolType string) (string, bool) {
	if options == nil {
		return "", false
	}
	switch strings.TrimSpace(options.ResponsesToolsMode) {
	case AdvancedCustomResponsesToolsModePreserve:
		return AdvancedCustomResponsesToolPolicyPreserve, true
	case AdvancedCustomResponsesToolsModeCompatFlatten:
		switch normalizeAdvancedCustomResponsesToolType(toolType) {
		case "function":
			return AdvancedCustomResponsesToolPolicyPreserve, true
		case "namespace":
			return AdvancedCustomResponsesToolPolicyFlatten, true
		default:
			return AdvancedCustomResponsesToolPolicyDrop, true
		}
	default:
		return "", false
	}
}

func validateAdvancedCustomResponsesToolConflictPolicy(index int, policy string) error {
	policy = strings.TrimSpace(policy)
	if policy == "" {
		return nil
	}
	switch policy {
	case AdvancedCustomResponsesToolConflictPolicyPreserve,
		AdvancedCustomResponsesToolConflictPolicyDeduplicate,
		AdvancedCustomResponsesToolConflictPolicyReject:
		return nil
	default:
		return fmt.Errorf("advanced_custom.advanced_routes[%d].converter_options.responses_tool_conflict_policy is invalid: %s", index, policy)
	}
}

func validateAdvancedCustomResponsesImplicitHostedTools(index int, field string, values []string) error {
	seen := make(map[string]struct{}, len(values))
	for valueIndex, raw := range values {
		capability := normalizeAdvancedCustomResponsesHostedCapability(raw)
		if capability == "" {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].%s[%d] must not be empty", index, field, valueIndex)
		}
		if _, exists := seen[capability]; exists {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].%s contains duplicate capability: %s", index, field, capability)
		}
		seen[capability] = struct{}{}
	}
	return nil
}

func validateAdvancedCustomResponsesImplicitHostedConflictPolicy(index int, options *AdvancedCustomConverterOptions) error {
	if options == nil || ResolveAdvancedCustomResponsesToolConflictPolicy(options) != AdvancedCustomResponsesToolConflictPolicyPreserve {
		return nil
	}
	if len(options.ResponsesImplicitHostedTools) > 0 {
		return fmt.Errorf("advanced_custom.advanced_routes[%d].converter_options.responses_implicit_hosted_tools requires responses_tool_conflict_policy deduplicate or reject", index)
	}
	for overrideIndex, override := range options.ResponsesToolModelOverrides {
		if len(override.ResponsesImplicitHostedTools) > 0 {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].converter_options.responses_tool_model_overrides[%d].responses_implicit_hosted_tools requires responses_tool_conflict_policy deduplicate or reject", index, overrideIndex)
		}
	}
	return nil
}

func validateAdvancedCustomResponsesToolParameters(index int, field string, parameters *AdvancedCustomResponsesToolParameters) error {
	if parameters == nil {
		return nil
	}
	if parameters.WebSearch == nil {
		return fmt.Errorf("advanced_custom.advanced_routes[%d].%s requires web_search", index, field)
	}
	webSearch := parameters.WebSearch
	mode := strings.TrimSpace(webSearch.WhenNestedOptionsMissing)
	switch mode {
	case AdvancedCustomResponsesToolMissingOptionsPreserve:
		if webSearch.Defaults.Enable != nil || webSearch.Defaults.SearchResult != nil || strings.TrimSpace(webSearch.Defaults.SearchEngine) != "" {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].%s.web_search.defaults requires populate_defaults", index, field)
		}
	case AdvancedCustomResponsesToolMissingOptionsPopulateDefaults:
		if webSearch.Defaults.Enable == nil && webSearch.Defaults.SearchResult == nil && strings.TrimSpace(webSearch.Defaults.SearchEngine) == "" {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].%s.web_search.defaults must not be empty", index, field)
		}
	default:
		return fmt.Errorf("advanced_custom.advanced_routes[%d].%s.web_search.when_nested_options_missing is invalid: %s", index, field, mode)
	}
	return nil
}

func validateAdvancedCustomResponsesToolModelOverrides(index int, allowFlatten bool, options *AdvancedCustomConverterOptions) error {
	if options == nil || len(options.ResponsesToolModelOverrides) == 0 {
		return nil
	}
	seenModels := make(map[string]struct{})
	for overrideIndex, override := range options.ResponsesToolModelOverrides {
		if len(override.Models) == 0 {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].converter_options.responses_tool_model_overrides[%d].models is required", index, overrideIndex)
		}
		for _, rawModel := range override.Models {
			model := strings.TrimSpace(rawModel)
			if model == "" {
				return fmt.Errorf("advanced_custom.advanced_routes[%d].converter_options.responses_tool_model_overrides[%d].models must not contain empty values", index, overrideIndex)
			}
			if _, exists := seenModels[model]; exists {
				return fmt.Errorf("advanced_custom.advanced_routes[%d].converter_options model is already configured: %s", index, model)
			}
			seenModels[model] = struct{}{}
		}

		if override.ResponsesTools == nil && len(override.ToolNames) == 0 && len(override.ResponsesImplicitHostedTools) == 0 && override.ResponsesToolParameters == nil {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].converter_options.responses_tool_model_overrides[%d] requires a tool policy, implicit hosted tool, or tool parameter rule", index, overrideIndex)
		}
		if err := validateAdvancedCustomResponsesImplicitHostedTools(
			index,
			fmt.Sprintf("converter_options.responses_tool_model_overrides[%d].responses_implicit_hosted_tools", overrideIndex),
			override.ResponsesImplicitHostedTools,
		); err != nil {
			return err
		}
		if override.ResponsesTools != nil {
			fields := []struct {
				name         string
				toolType     string
				policy       string
				allowFlatten bool
			}{
				{name: "namespace", toolType: "namespace", policy: override.ResponsesTools.Namespace, allowFlatten: allowFlatten},
				{name: "custom", toolType: "custom", policy: override.ResponsesTools.Custom},
				{name: "web_search", toolType: "web_search", policy: override.ResponsesTools.WebSearch},
				{name: "tool_search", toolType: "tool_search", policy: override.ResponsesTools.ToolSearch},
				{name: "image_generation", toolType: "image_generation", policy: override.ResponsesTools.ImageGeneration},
				{name: "unknown", toolType: "unknown", policy: override.ResponsesTools.Unknown},
			}
			configured := false
			for _, field := range fields {
				if strings.TrimSpace(field.policy) == "" {
					continue
				}
				configured = true
				if err := validateAdvancedCustomResponsesToolPolicy(index, field.name, field.policy, field.allowFlatten); err != nil {
					return err
				}
				routePolicy := ResolveAdvancedCustomResponsesToolPolicy(options, "", "", field.toolType, "").Policy
				if strings.TrimSpace(field.policy) == routePolicy {
					return fmt.Errorf("advanced_custom.advanced_routes[%d].converter_options.responses_tool_model_overrides[%d].responses_tools.%s must differ from the route policy", index, overrideIndex, field.name)
				}
			}
			if !configured && len(override.ToolNames) == 0 && len(override.ResponsesImplicitHostedTools) == 0 && override.ResponsesToolParameters == nil {
				return fmt.Errorf("advanced_custom.advanced_routes[%d].converter_options.responses_tool_model_overrides[%d] requires a non-empty tool policy, implicit hosted tool, or tool parameter rule", index, overrideIndex)
			}
		}

		if err := validateAdvancedCustomResponsesToolParameters(
			index,
			fmt.Sprintf("converter_options.responses_tool_model_overrides[%d].responses_tool_parameters", overrideIndex),
			override.ResponsesToolParameters,
		); err != nil {
			return err
		}

		seenToolNames := make(map[string]struct{})
		for nameIndex, namePolicy := range override.ToolNames {
			toolType := strings.TrimSpace(namePolicy.ToolType)
			toolName := strings.TrimSpace(namePolicy.ToolName)
			policy := strings.TrimSpace(namePolicy.Policy)
			if toolType == "" || policy == "" || (toolName == "" && !IsAdvancedCustomUnnamedNativeResponsesToolType(toolType)) {
				return fmt.Errorf("advanced_custom.advanced_routes[%d].converter_options.responses_tool_model_overrides[%d].responses_tool_names[%d] requires tool_type, tool_name, and policy", index, overrideIndex, nameIndex)
			}
			if normalizeAdvancedCustomResponsesToolType(toolType) == "function" {
				return fmt.Errorf("advanced_custom.advanced_routes[%d].converter_options function tools are always preserved", index)
			}
			if policy == AdvancedCustomResponsesToolPolicyFlatten {
				return fmt.Errorf("advanced_custom.advanced_routes[%d].converter_options.responses_tool_names[%d].policy does not support flatten", index, nameIndex)
			}
			if err := validateAdvancedCustomResponsesToolPolicy(index, "tool_name", policy, false); err != nil {
				return err
			}
			key := normalizeAdvancedCustomResponsesToolType(toolType) + "\x00" + toolName
			if _, exists := seenToolNames[key]; exists {
				return fmt.Errorf("advanced_custom.advanced_routes[%d].converter_options duplicate responses tool name policy: %s/%s", index, toolType, toolName)
			}
			seenToolNames[key] = struct{}{}
		}
	}
	return nil
}
