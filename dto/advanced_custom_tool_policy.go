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
	for _, override := range options.ResponsesToolModelOverrides {
		for _, rawModel := range override.Models {
			model := strings.TrimSpace(rawModel)
			if model != "" && (model == requestedModel || model == upstreamModel) {
				return override, model, true
			}
		}
	}
	return AdvancedCustomResponsesToolModelOverride{}, "", false
}

func advancedCustomResponsesToolNamePolicyMatches(policy AdvancedCustomResponsesToolNamePolicy, toolType string, toolName string) bool {
	if strings.TrimSpace(policy.ToolName) != strings.TrimSpace(toolName) {
		return false
	}
	configuredType := strings.TrimSpace(policy.ToolType)
	actualType := strings.TrimSpace(toolType)
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

		if override.ResponsesTools == nil && len(override.ToolNames) == 0 {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].converter_options.responses_tool_model_overrides[%d] requires a tool policy", index, overrideIndex)
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
			if !configured && len(override.ToolNames) == 0 {
				return fmt.Errorf("advanced_custom.advanced_routes[%d].converter_options.responses_tool_model_overrides[%d] requires a non-empty tool policy", index, overrideIndex)
			}
		}

		seenToolNames := make(map[string]struct{})
		for nameIndex, namePolicy := range override.ToolNames {
			toolType := strings.TrimSpace(namePolicy.ToolType)
			toolName := strings.TrimSpace(namePolicy.ToolName)
			policy := strings.TrimSpace(namePolicy.Policy)
			if toolType == "" || toolName == "" || policy == "" {
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
