package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type toolCompatibilityEventStatusRequest struct {
	Status string `json:"status"`
}

func ListToolCompatibilityEvents(c *gin.Context) {
	channelID, _ := strconv.Atoi(c.Query("channel_id"))
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	events, total, err := model.ListToolCompatibilityEvents(model.ToolCompatibilityEventListOptions{ChannelId: channelID, Route: c.Query("route"), RequestedModel: c.Query("requested_model"), UpstreamModel: c.Query("upstream_model"), ToolType: c.Query("tool_type"), EventType: c.Query("event_type"), ResolutionStatus: c.Query("resolution_status"), Page: page, PageSize: pageSize})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": events, "total": total, "page": max(page, 1), "page_size": min(max(pageSize, 1), 100)})
}

func UpdateToolCompatibilityEventStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var request toolCompatibilityEventStatusRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	event, err := model.UpdateToolCompatibilityEventResolutionStatus(id, request.Status)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": event})
}

func ApplyToolCompatibilityEventSuggestion(c *gin.Context) {
	mutateToolCompatibilityEventConfig(c, false)
}
func RestoreToolCompatibilityEventModelDefault(c *gin.Context) {
	mutateToolCompatibilityEventConfig(c, true)
}

func mutateToolCompatibilityEventConfig(c *gin.Context, restore bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	event, err := model.GetToolCompatibilityEventByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	modelName := strings.TrimSpace(event.RequestedModel)
	if modelName == "" {
		modelName = strings.TrimSpace(event.UpstreamModel)
	}
	if modelName == "" {
		common.ApiError(c, errors.New("event has no model to modify"))
		return
	}

	// Lock the channel row for the short read-modify-write transaction. Applying
	// one event must not overwrite a concurrent edit of the same settings JSON.
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		var channel model.Channel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&channel, event.ChannelId).Error; err != nil {
			return err
		}
		if channel.Type != constant.ChannelTypeAdvancedCustom {
			return errors.New("event channel is not Advanced Custom")
		}
		settings := channel.GetOtherSettings()
		config := settings.AdvancedCustom
		if config == nil {
			return errors.New("advanced_custom settings are missing")
		}
		var route *dto.AdvancedCustomRoute
		for i := range config.Routes {
			if strings.TrimSpace(config.Routes[i].IncomingPath) == strings.TrimSpace(event.Route) {
				route = &config.Routes[i]
				break
			}
		}
		if route == nil {
			return errors.New("event route no longer exists")
		}
		if route.ConverterOptions == nil {
			route.ConverterOptions = &dto.AdvancedCustomConverterOptions{}
		}
		if restore {
			removeModelToolCompatibilityOverride(route.ConverterOptions, modelName, event.ToolType, event.ToolName)
		} else if err := applyModelToolCompatibilitySuggestion(route.ConverterOptions, modelName, event); err != nil {
			return err
		}
		if err := config.Validate(); err != nil {
			return err
		}
		channel.SetOtherSettings(settings)
		return tx.Model(&model.Channel{}).Where("id = ?", channel.Id).Update("settings", channel.OtherSettings).Error
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitChannelCache()
	if !restore {
		_, _ = model.UpdateToolCompatibilityEventResolutionStatus(event.Id, model.ToolCompatibilityResolutionStatusResolved)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"event": event, "model": modelName}})
}

func applyModelToolCompatibilitySuggestion(options *dto.AdvancedCustomConverterOptions, modelName string, event *model.ToolCompatibilityEvent) error {
	policy := strings.TrimSpace(event.SuggestedPolicy)
	if policy == "" {
		return errors.New("event has no actionable suggestion")
	}
	if event.EventType == model.ToolCompatibilityEventTypeNameConflict {
		// Conflict policy is route-scoped in the current Advanced Custom schema.
		options.ResponsesToolConflictPolicy = policy
		return nil
	}
	if event.ToolType == "function" {
		return errors.New("function tools cannot be disabled")
	}
	override := findOrCreateModelToolCompatibilityOverride(options, modelName)
	if toolName := strings.TrimSpace(event.ToolName); toolName != "" {
		override.ToolNames = upsertToolCompatibilityNamePolicy(override.ToolNames, event.ToolType, toolName, policy)
		return nil
	}
	if override.ResponsesTools == nil {
		override.ResponsesTools = &dto.AdvancedCustomResponsesToolsOptions{}
	}
	switch event.ToolType {
	case "namespace":
		override.ResponsesTools.Namespace = policy
	case "custom":
		override.ResponsesTools.Custom = policy
	case "web_search", "web_search_preview":
		override.ResponsesTools.WebSearch = policy
	case "tool_search":
		override.ResponsesTools.ToolSearch = policy
	case "image_gen", "image_generation":
		override.ResponsesTools.ImageGeneration = policy
	default:
		override.ResponsesTools.Unknown = policy
	}
	return nil
}

func findOrCreateModelToolCompatibilityOverride(options *dto.AdvancedCustomConverterOptions, modelName string) *dto.AdvancedCustomResponsesToolModelOverride {
	modelName = strings.TrimSpace(modelName)
	for i := range options.ResponsesToolModelOverrides {
		override := &options.ResponsesToolModelOverrides[i]
		for _, existing := range override.Models {
			if strings.TrimSpace(existing) != modelName {
				continue
			}
			if len(override.Models) == 1 {
				return override
			}
			// Admin actions default to the event model only. Split a UI-created shared
			// override before mutating it so other models retain their previous policy.
			clone := cloneModelToolCompatibilityOverride(*override)
			clone.Models = []string{modelName}
			override.Models = removeModelName(override.Models, modelName)
			options.ResponsesToolModelOverrides = append(options.ResponsesToolModelOverrides, clone)
			return &options.ResponsesToolModelOverrides[len(options.ResponsesToolModelOverrides)-1]
		}
	}
	options.ResponsesToolModelOverrides = append(options.ResponsesToolModelOverrides, dto.AdvancedCustomResponsesToolModelOverride{Models: []string{modelName}})
	return &options.ResponsesToolModelOverrides[len(options.ResponsesToolModelOverrides)-1]
}

func cloneModelToolCompatibilityOverride(value dto.AdvancedCustomResponsesToolModelOverride) dto.AdvancedCustomResponsesToolModelOverride {
	clone := dto.AdvancedCustomResponsesToolModelOverride{Models: append([]string(nil), value.Models...)}
	if value.ResponsesTools != nil {
		tools := *value.ResponsesTools
		clone.ResponsesTools = &tools
	}
	clone.ToolNames = append([]dto.AdvancedCustomResponsesToolNamePolicy(nil), value.ToolNames...)
	return clone
}

func removeModelName(models []string, target string) []string {
	out := make([]string, 0, len(models))
	for _, name := range models {
		if strings.TrimSpace(name) != target {
			out = append(out, name)
		}
	}
	return out
}

func upsertToolCompatibilityNamePolicy(policies []dto.AdvancedCustomResponsesToolNamePolicy, toolType string, toolName string, policy string) []dto.AdvancedCustomResponsesToolNamePolicy {
	for i := range policies {
		if strings.TrimSpace(policies[i].ToolType) == strings.TrimSpace(toolType) && strings.TrimSpace(policies[i].ToolName) == strings.TrimSpace(toolName) {
			policies[i].Policy = policy
			return policies
		}
	}
	return append(policies, dto.AdvancedCustomResponsesToolNamePolicy{ToolType: strings.TrimSpace(toolType), ToolName: strings.TrimSpace(toolName), Policy: policy})
}

func removeModelToolCompatibilityOverride(options *dto.AdvancedCustomConverterOptions, modelName, toolType, toolName string) {
	modelName = strings.TrimSpace(modelName)
	for i := 0; i < len(options.ResponsesToolModelOverrides); i++ {
		override := &options.ResponsesToolModelOverrides[i]
		matched := false
		for _, name := range override.Models {
			if strings.TrimSpace(name) == modelName {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		if len(override.Models) > 1 {
			override = findOrCreateModelToolCompatibilityOverride(options, modelName)
			i = len(options.ResponsesToolModelOverrides) - 1
		}
		if name := strings.TrimSpace(toolName); name != "" {
			filtered := override.ToolNames[:0]
			for _, policy := range override.ToolNames {
				if strings.TrimSpace(policy.ToolType) == strings.TrimSpace(toolType) && strings.TrimSpace(policy.ToolName) == name {
					continue
				}
				filtered = append(filtered, policy)
			}
			override.ToolNames = filtered
		} else if override.ResponsesTools != nil {
			switch toolType {
			case "namespace":
				override.ResponsesTools.Namespace = ""
			case "custom":
				override.ResponsesTools.Custom = ""
			case "web_search", "web_search_preview":
				override.ResponsesTools.WebSearch = ""
			case "tool_search":
				override.ResponsesTools.ToolSearch = ""
			case "image_gen", "image_generation":
				override.ResponsesTools.ImageGeneration = ""
			default:
				override.ResponsesTools.Unknown = ""
			}
		}
		if modelToolCompatibilityOverrideEmpty(*override) {
			options.ResponsesToolModelOverrides = append(options.ResponsesToolModelOverrides[:i], options.ResponsesToolModelOverrides[i+1:]...)
		}
		return
	}
}

func modelToolCompatibilityOverrideEmpty(override dto.AdvancedCustomResponsesToolModelOverride) bool {
	if len(override.ToolNames) > 0 {
		return false
	}
	if override.ResponsesTools == nil {
		return true
	}
	tools := override.ResponsesTools
	return strings.TrimSpace(tools.Namespace) == "" && strings.TrimSpace(tools.Custom) == "" && strings.TrimSpace(tools.WebSearch) == "" && strings.TrimSpace(tools.ToolSearch) == "" && strings.TrimSpace(tools.ImageGeneration) == "" && strings.TrimSpace(tools.Unknown) == ""
}
