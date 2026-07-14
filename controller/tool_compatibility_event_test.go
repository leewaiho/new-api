package controller

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

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

func TestApplyModelToolCompatibilitySuggestionUsesRouteConflictPolicy(t *testing.T) {
	options := &dto.AdvancedCustomConverterOptions{}
	require.NoError(t, applyModelToolCompatibilitySuggestion(options, "glm-5.2", &model.ToolCompatibilityEvent{
		EventType:       model.ToolCompatibilityEventTypeNameConflict,
		SuggestedPolicy: dto.AdvancedCustomResponsesToolConflictPolicyDeduplicate,
	}))
	require.Equal(t, dto.AdvancedCustomResponsesToolConflictPolicyDeduplicate, options.ResponsesToolConflictPolicy)
	require.Empty(t, options.ResponsesToolModelOverrides)
}

func withToolCompatibilityControllerDB(t *testing.T) {
	t.Helper()
	previous := model.DB
	db, err := gorm.Open(sqlite.Open("file:controller_tool_compatibility_"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ToolCompatibilityEvent{}))
	model.DB = db
	t.Cleanup(func() { model.DB = previous })
}

func TestListAndUpdateToolCompatibilityEventsHandlers(t *testing.T) {
	withToolCompatibilityControllerDB(t)
	event, err := model.RecordToolCompatibilityEvent(model.ToolCompatibilityEventInput{
		ChannelId: 16, Route: "/v1/responses", RequestedModel: "glm-5.2", ToolType: "image_gen",
		EventType: model.ToolCompatibilityEventTypePolicyDrop, CurrentPolicy: "drop",
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/tool-compatibility/events?channel_id=16&page=1&page_size=20", nil)
	ListToolCompatibilityEvents(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "glm-5.2")

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
