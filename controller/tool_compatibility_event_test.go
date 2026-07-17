package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestApplyModelToolCompatibilitySuggestionUsesToolNameWithoutAffectingSharedModels(t *testing.T) {
	options := &dto.AdvancedCustomConverterOptions{
		ResponsesToolModelOverrides: []dto.AdvancedCustomResponsesToolModelOverride{{
			Models: []string{"glm-5.2", "glm-5.3"},
			ResponsesTools: &dto.AdvancedCustomResponsesToolsOptions{
				WebSearch: dto.AdvancedCustomResponsesToolPolicyPreserve,
			},
		}},
	}
	event := &model.ToolCompatibilityEvent{
		ToolType: "web_search", ToolName: "web_search_preview",
		SuggestedPolicy: dto.AdvancedCustomResponsesToolPolicyDrop,
	}
	require.NoError(t, applyModelToolCompatibilitySuggestion(options, "glm-5.2", event))
	require.Len(t, options.ResponsesToolModelOverrides, 2)

	var target, untouched *dto.AdvancedCustomResponsesToolModelOverride
	for i := range options.ResponsesToolModelOverrides {
		override := &options.ResponsesToolModelOverrides[i]
		if len(override.Models) == 1 && override.Models[0] == "glm-5.2" {
			target = override
		}
		if len(override.Models) == 1 && override.Models[0] == "glm-5.3" {
			untouched = override
		}
	}
	require.NotNil(t, target)
	require.NotNil(t, untouched)
	require.Len(t, target.ToolNames, 1)
	require.Equal(t, "web_search_preview", target.ToolNames[0].ToolName)
	require.Equal(t, dto.AdvancedCustomResponsesToolPolicyDrop, target.ToolNames[0].Policy)
	require.Empty(t, untouched.ToolNames)
	require.Equal(t, dto.AdvancedCustomResponsesToolPolicyPreserve, untouched.ResponsesTools.WebSearch)
}

func TestRemoveModelToolCompatibilityOverrideRemovesOnlyTargetedNameAndEmptyOverride(t *testing.T) {
	options := &dto.AdvancedCustomConverterOptions{
		ResponsesToolModelOverrides: []dto.AdvancedCustomResponsesToolModelOverride{{
			Models: []string{"glm-5.2"},
			ToolNames: []dto.AdvancedCustomResponsesToolNamePolicy{{
				ToolType: "image_gen", ToolName: "image_gen", Policy: dto.AdvancedCustomResponsesToolPolicyDrop,
			}},
		}},
	}
	removeModelToolCompatibilityOverride(options, "glm-5.2", "image_gen", "image_gen")
	require.Empty(t, options.ResponsesToolModelOverrides)
}

func TestApplyModelToolCompatibilitySuggestionRejectsFunctionTool(t *testing.T) {
	options := &dto.AdvancedCustomConverterOptions{}
	err := applyModelToolCompatibilitySuggestion(options, "glm-5.2", &model.ToolCompatibilityEvent{
		ToolType: "function", SuggestedPolicy: dto.AdvancedCustomResponsesToolPolicyDrop,
	})
	require.ErrorContains(t, err, "function tools cannot be disabled")
}

func TestApplyModelToolCompatibilitySuggestionRejectsRouteConflictPolicy(t *testing.T) {
	options := &dto.AdvancedCustomConverterOptions{}
	err := applyModelToolCompatibilitySuggestion(options, "glm-5.2", &model.ToolCompatibilityEvent{
		EventType:       model.ToolCompatibilityEventTypeNameConflict,
		SuggestedPolicy: dto.AdvancedCustomResponsesToolConflictPolicyDeduplicate,
	})
	require.ErrorContains(t, err, "explicitly confirmed route-scoped action")
	require.Empty(t, options.ResponsesToolConflictPolicy)
	require.Empty(t, options.ResponsesToolModelOverrides)
}

func withToolCompatibilityControllerDB(t *testing.T) {
	t.Helper()
	previousDB := model.DB
	previousMemoryCache := common.MemoryCacheEnabled
	db, err := gorm.Open(sqlite.Open("file:controller_tool_compatibility_"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.ToolCompatibilityEvent{}))
	model.DB = db
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		model.DB = previousDB
		common.MemoryCacheEnabled = previousMemoryCache
	})
}

func createToolCompatibilityChannel(t *testing.T, options *dto.AdvancedCustomConverterOptions) *model.Channel {
	t.Helper()
	channel := &model.Channel{
		Type:   constant.ChannelTypeAdvancedCustom,
		Key:    "sk-test",
		Name:   "tool-compatibility-test",
		Status: common.ChannelStatusEnabled,
		Models: "glm-5.2,glm-5.3",
		Group:  "default",
	}
	channel.SetOtherSettings(dto.ChannelOtherSettings{AdvancedCustom: &dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{{
			IncomingPath:     "/v1/responses",
			UpstreamPath:     "/v1/chat/completions",
			Converter:        dto.AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions,
			ConverterOptions: options,
		}},
	}})
	require.NoError(t, model.DB.Create(channel).Error)
	return channel
}

func createToolCompatibilityEvent(t *testing.T, channelID int, eventType, toolType, toolName, suggestion string) *model.ToolCompatibilityEvent {
	t.Helper()
	event, err := model.RecordToolCompatibilityEvent(model.ToolCompatibilityEventInput{
		ChannelId: channelID, Route: "/v1/responses", RequestedModel: "glm-5.2", UpstreamModel: "glm-5.2",
		ToolType: toolType, ToolName: toolName, EventType: eventType,
		CurrentPolicy: dto.AdvancedCustomResponsesToolPolicyPreserve, SuggestedPolicy: suggestion,
	})
	require.NoError(t, err)
	return event
}

type toolCompatibilityMutationEnvelope struct {
	Success bool                                 `json:"success"`
	Message string                               `json:"message"`
	Data    toolCompatibilityEventMutationResult `json:"data"`
}

func runToolCompatibilityMutation(t *testing.T, handler gin.HandlerFunc, eventID int, body string) toolCompatibilityMutationEnvelope {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(eventID)}}
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/tool-compatibility/events/"+strconv.Itoa(eventID), strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	handler(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)
	var envelope toolCompatibilityMutationEnvelope
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	return envelope
}

func loadToolCompatibilityRoute(t *testing.T, channelID int) dto.AdvancedCustomRoute {
	t.Helper()
	var channel model.Channel
	require.NoError(t, model.DB.First(&channel, channelID).Error)
	config := channel.GetOtherSettings().AdvancedCustom
	require.NotNil(t, config)
	require.Len(t, config.Routes, 1)
	return config.Routes[0]
}

func TestApplyToolCompatibilitySuggestionToSelectedModelReturnsEffectivePolicy(t *testing.T) {
	withToolCompatibilityControllerDB(t)
	channel := createToolCompatibilityChannel(t, &dto.AdvancedCustomConverterOptions{
		ResponsesTools: &dto.AdvancedCustomResponsesToolsOptions{Namespace: dto.AdvancedCustomResponsesToolPolicyFlatten},
	})
	event := createToolCompatibilityEvent(t, channel.Id, model.ToolCompatibilityEventTypeUpstreamUnsupported, "image_gen", "", dto.AdvancedCustomResponsesToolPolicyDrop)

	response := runToolCompatibilityMutation(t, ApplyToolCompatibilityEventSuggestion, event.Id, `{"target_model":"glm-5.3"}`)
	require.True(t, response.Success, response.Message)
	require.Equal(t, "glm-5.3", response.Data.Model)
	require.Equal(t, toolCompatibilityMutationScopeModel, response.Data.Scope)
	require.Equal(t, dto.AdvancedCustomResponsesToolPolicyDrop, response.Data.EffectivePolicy)
	require.Equal(t, dto.AdvancedCustomResponsesToolPolicySourceModelToolType, response.Data.PolicySource)
	require.Equal(t, model.ToolCompatibilityResolutionStatusResolved, response.Data.Event.ResolutionStatus)
	require.NotNil(t, response.Data.RouteConfig)
	require.Equal(t, "/v1/responses", response.Data.RouteConfig.IncomingPath)
	require.Len(t, response.Data.RouteConfig.ConverterOptions.ResponsesToolModelOverrides, 1)
	require.Equal(t, []string{"glm-5.3"}, response.Data.RouteConfig.ConverterOptions.ResponsesToolModelOverrides[0].Models)
	require.Equal(t, dto.AdvancedCustomResponsesToolPolicyDrop, response.Data.RouteConfig.ConverterOptions.ResponsesToolModelOverrides[0].ResponsesTools.ImageGeneration)

	route := loadToolCompatibilityRoute(t, channel.Id)
	require.Len(t, route.ConverterOptions.ResponsesToolModelOverrides, 1)
	require.Equal(t, []string{"glm-5.3"}, route.ConverterOptions.ResponsesToolModelOverrides[0].Models)
	require.Equal(t, dto.AdvancedCustomResponsesToolPolicyDrop, route.ConverterOptions.ResponsesToolModelOverrides[0].ResponsesTools.ImageGeneration)
}

func TestApplyToolCompatibilitySuggestionRejectsStaleModelWithoutChangingStatus(t *testing.T) {
	withToolCompatibilityControllerDB(t)
	channel := createToolCompatibilityChannel(t, &dto.AdvancedCustomConverterOptions{})
	event := createToolCompatibilityEvent(t, channel.Id, model.ToolCompatibilityEventTypeUpstreamUnsupported, "image_gen", "", dto.AdvancedCustomResponsesToolPolicyDrop)
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", channel.Id).Update("models", "glm-5.3").Error)

	response := runToolCompatibilityMutation(t, ApplyToolCompatibilityEventSuggestion, event.Id, `{}`)
	require.False(t, response.Success)
	require.Contains(t, response.Message, `target model "glm-5.2" is not configured`)
	updated, err := model.GetToolCompatibilityEventByID(event.Id)
	require.NoError(t, err)
	require.Equal(t, model.ToolCompatibilityResolutionStatusOpen, updated.ResolutionStatus)
	require.Empty(t, loadToolCompatibilityRoute(t, channel.Id).ConverterOptions.ResponsesToolModelOverrides)
}

func TestApplyRouteNameConflictRequiresConfirmation(t *testing.T) {
	withToolCompatibilityControllerDB(t)
	channel := createToolCompatibilityChannel(t, &dto.AdvancedCustomConverterOptions{})
	event := createToolCompatibilityEvent(t, channel.Id, model.ToolCompatibilityEventTypeNameConflict, "image_gen", "image_gen.imagegen", dto.AdvancedCustomResponsesToolConflictPolicyReject)

	response := runToolCompatibilityMutation(t, ApplyToolCompatibilityEventSuggestion, event.Id, `{"scope":"route"}`)
	require.False(t, response.Success)
	require.Contains(t, response.Message, "confirm_route=true")
	require.Empty(t, loadToolCompatibilityRoute(t, channel.Id).ConverterOptions.ResponsesToolConflictPolicy)

	response = runToolCompatibilityMutation(t, ApplyToolCompatibilityEventSuggestion, event.Id, `{"scope":"route","confirm_route":true}`)
	require.True(t, response.Success, response.Message)
	require.Equal(t, toolCompatibilityMutationScopeRoute, response.Data.Scope)
	require.Equal(t, dto.AdvancedCustomResponsesToolConflictPolicyReject, response.Data.EffectivePolicy)
	require.Equal(t, dto.AdvancedCustomResponsesToolPolicySourceRoute, response.Data.PolicySource)
	require.NotNil(t, response.Data.RouteConfig)
	require.Equal(t, dto.AdvancedCustomResponsesToolConflictPolicyReject, response.Data.RouteConfig.ConverterOptions.ResponsesToolConflictPolicy)
	require.Equal(t, dto.AdvancedCustomResponsesToolConflictPolicyReject, loadToolCompatibilityRoute(t, channel.Id).ConverterOptions.ResponsesToolConflictPolicy)
}

func TestRestoreRouteSafeDefaultsPreservesModelOverrides(t *testing.T) {
	withToolCompatibilityControllerDB(t)
	channel := createToolCompatibilityChannel(t, &dto.AdvancedCustomConverterOptions{
		ResponsesToolsMode:          dto.AdvancedCustomResponsesToolsModeCompatFlatten,
		ResponsesToolConflictPolicy: dto.AdvancedCustomResponsesToolConflictPolicyPreserve,
		ResponsesTools: &dto.AdvancedCustomResponsesToolsOptions{
			Namespace:       dto.AdvancedCustomResponsesToolPolicyPreserve,
			Custom:          dto.AdvancedCustomResponsesToolPolicyDrop,
			WebSearch:       dto.AdvancedCustomResponsesToolPolicyDrop,
			ToolSearch:      dto.AdvancedCustomResponsesToolPolicyDrop,
			ImageGeneration: dto.AdvancedCustomResponsesToolPolicyDrop,
			Unknown:         dto.AdvancedCustomResponsesToolPolicyDrop,
		},
		ResponsesDropFields: []string{"metadata"},
		ResponsesToolModelOverrides: []dto.AdvancedCustomResponsesToolModelOverride{{
			Models:         []string{"glm-5.2"},
			ResponsesTools: &dto.AdvancedCustomResponsesToolsOptions{ImageGeneration: dto.AdvancedCustomResponsesToolPolicyDrop},
		}},
	})
	event := createToolCompatibilityEvent(t, channel.Id, model.ToolCompatibilityEventTypeNameConflict, "image_gen", "image_gen.imagegen", dto.AdvancedCustomResponsesToolConflictPolicyReject)

	response := runToolCompatibilityMutation(t, RestoreToolCompatibilityEventModelDefault, event.Id, `{"scope":"route","confirm_route":true}`)
	require.True(t, response.Success, response.Message)
	require.Equal(t, dto.AdvancedCustomResponsesToolConflictPolicyDeduplicate, response.Data.EffectivePolicy)
	require.NotNil(t, response.Data.RouteConfig)
	require.Equal(t, dto.AdvancedCustomResponsesToolConflictPolicyDeduplicate, response.Data.RouteConfig.ConverterOptions.ResponsesToolConflictPolicy)
	require.Equal(t, dto.AdvancedCustomResponsesToolPolicyFlatten, response.Data.RouteConfig.ConverterOptions.ResponsesTools.Namespace)
	require.Empty(t, response.Data.RouteConfig.ConverterOptions.ResponsesTools.WebSearch)
	require.Equal(t, []string{"metadata"}, response.Data.RouteConfig.ConverterOptions.ResponsesDropFields)
	require.Len(t, response.Data.RouteConfig.ConverterOptions.ResponsesToolModelOverrides, 1)
	route := loadToolCompatibilityRoute(t, channel.Id)
	require.Empty(t, route.ConverterOptions.ResponsesToolsMode)
	require.Equal(t, dto.AdvancedCustomResponsesToolConflictPolicyDeduplicate, route.ConverterOptions.ResponsesToolConflictPolicy)
	require.Equal(t, dto.AdvancedCustomResponsesToolPolicyFlatten, route.ConverterOptions.ResponsesTools.Namespace)
	require.Empty(t, route.ConverterOptions.ResponsesTools.Custom)
	require.Empty(t, route.ConverterOptions.ResponsesTools.WebSearch)
	require.Equal(t, []string{"metadata"}, route.ConverterOptions.ResponsesDropFields)
	require.Len(t, route.ConverterOptions.ResponsesToolModelOverrides, 1)
	require.Equal(t, dto.AdvancedCustomResponsesToolPolicyDrop, route.ConverterOptions.ResponsesToolModelOverrides[0].ResponsesTools.ImageGeneration)
}

func TestRestoreModelDefaultSplitsSharedOverrideAndPreservesOtherModel(t *testing.T) {
	withToolCompatibilityControllerDB(t)
	channel := createToolCompatibilityChannel(t, &dto.AdvancedCustomConverterOptions{
		ResponsesToolModelOverrides: []dto.AdvancedCustomResponsesToolModelOverride{{
			Models:         []string{"glm-5.2", "glm-5.3"},
			ResponsesTools: &dto.AdvancedCustomResponsesToolsOptions{ImageGeneration: dto.AdvancedCustomResponsesToolPolicyDrop},
		}},
	})
	event := createToolCompatibilityEvent(t, channel.Id, model.ToolCompatibilityEventTypeUpstreamUnsupported, "image_gen", "", dto.AdvancedCustomResponsesToolPolicyDrop)

	response := runToolCompatibilityMutation(t, RestoreToolCompatibilityEventModelDefault, event.Id, `{}`)
	require.True(t, response.Success, response.Message)
	require.Equal(t, dto.AdvancedCustomResponsesToolPolicyPreserve, response.Data.EffectivePolicy)
	require.Equal(t, dto.AdvancedCustomResponsesToolPolicySourceSystemDefault, response.Data.PolicySource)
	require.NotNil(t, response.Data.RouteConfig)
	require.Len(t, response.Data.RouteConfig.ConverterOptions.ResponsesToolModelOverrides, 1)
	require.Equal(t, []string{"glm-5.3"}, response.Data.RouteConfig.ConverterOptions.ResponsesToolModelOverrides[0].Models)
	route := loadToolCompatibilityRoute(t, channel.Id)
	require.Len(t, route.ConverterOptions.ResponsesToolModelOverrides, 1)
	require.Equal(t, []string{"glm-5.3"}, route.ConverterOptions.ResponsesToolModelOverrides[0].Models)
	require.Equal(t, dto.AdvancedCustomResponsesToolPolicyDrop, route.ConverterOptions.ResponsesToolModelOverrides[0].ResponsesTools.ImageGeneration)
}

func TestListAndUpdateToolCompatibilityEventsHandlers(t *testing.T) {
	withToolCompatibilityControllerDB(t)
	channel := createToolCompatibilityChannel(t, nil)
	event, err := model.RecordToolCompatibilityEvent(model.ToolCompatibilityEventInput{
		ChannelId: channel.Id, Route: "/v1/responses", RequestedModel: "glm-5.2", ToolType: "image_gen",
		EventType: model.ToolCompatibilityEventTypePolicyDrop, CurrentPolicy: "drop",
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/tool-compatibility/events?channel_id="+strconv.Itoa(channel.Id)+"&page=1&page_size=20", nil)
	ListToolCompatibilityEvents(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "glm-5.2")
	require.Contains(t, recorder.Body.String(), `"channel_name":"tool-compatibility-test"`)

	recorder = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(event.Id)}}
	ctx.Request = httptest.NewRequest(http.MethodPatch, "/api/tool-compatibility/events/1/status", strings.NewReader(`{"status":"ignored"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	UpdateToolCompatibilityEventStatus(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)
	updated, err := model.GetToolCompatibilityEventByID(event.Id)
	require.NoError(t, err)
	require.Equal(t, model.ToolCompatibilityResolutionStatusIgnored, updated.ResolutionStatus)
}
