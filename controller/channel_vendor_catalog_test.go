package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupVendorCatalogControllerTestDB(t *testing.T) {
	t.Helper()
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}))
}

func TestEnsureVolcengineCodingPlanVendorCatalog(t *testing.T) {
	setupVendorCatalogControllerTestDB(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/vendors/catalogs/volcengine-codingplan", nil)

	EnsureVolcengineCodingPlanVendorCatalog(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	vendor, err := model.GetVendorByID(1)
	require.NoError(t, err)
	require.Equal(t, "Volcengine CodingPlan", vendor.Name)

	modelNames, err := model.GetEnabledModelNamesByVendorID(vendor.Id)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{
		"MiniMax-M2.7",
		"MiniMax-M3",
		"deepseek-v4-flash",
		"deepseek-v4-pro",
		"doubao-seed-2.0-code",
		"doubao-seed-2.0-lite",
		"doubao-seed-2.0-pro",
		"glm-5.2",
		"gpt-5.4-mini",
		"kimi-k2.7-code",
	}, modelNames)

	// Idempotent second run should not duplicate models.
	recorder = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/vendors/catalogs/volcengine-codingplan", nil)
	EnsureVolcengineCodingPlanVendorCatalog(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	var count int64
	require.NoError(t, model.DB.Model(&model.Model{}).Count(&count).Error)
	require.EqualValues(t, 10, count)
}

func TestChannelVendorCatalogImportAppendAndReplace(t *testing.T) {
	setupVendorCatalogControllerTestDB(t)

	vendor := &model.Vendor{Name: "Volcengine CodingPlan", Status: 1}
	require.NoError(t, vendor.Insert())
	for _, name := range []string{"glm-5.2", "doubao-seed-2.0-pro"} {
		require.NoError(t, (&model.Model{ModelName: name, VendorID: vendor.Id, Status: 1}).Insert())
	}
	priority := int64(10)
	require.NoError(t, model.DB.Create(&model.Channel{
		Id:       100,
		Type:     constant.ChannelTypeAdvancedCustom,
		Key:      "test-key",
		Status:   common.ChannelStatusEnabled,
		Name:     "test-ac",
		Models:   "old-model,glm-5.2",
		Group:    "default",
		Priority: &priority,
	}).Error)
	require.NoError(t, (&model.Channel{Id: 100, Models: "old-model,glm-5.2", Group: "default", Status: common.ChannelStatusEnabled, Priority: &priority}).UpdateAbilities(nil))

	channel, err := model.GetChannelById(100, true)
	require.NoError(t, err)
	result := buildChannelVendorCatalogImportResult(channel, vendor, []string{"glm-5.2", "doubao-seed-2.0-pro"}, channelVendorCatalogImportModeAppend)
	require.Equal(t, []string{"doubao-seed-2.0-pro"}, result.AddModels)
	require.Empty(t, result.RemoveModels)
	require.Equal(t, []string{"old-model", "glm-5.2", "doubao-seed-2.0-pro"}, result.NextModels)

	body := bytes.NewBufferString(`{"channel_id":100,"vendor_id":1,"mode":"replace"}`)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/channel/vendor_models/apply", body)
	ctx.Request.Header.Set("Content-Type", "application/json")

	ApplyChannelVendorCatalogModels(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	updated, err := model.GetChannelById(100, true)
	require.NoError(t, err)
	require.Equal(t, "glm-5.2,doubao-seed-2.0-pro", updated.Models)

	var abilities []model.Ability
	require.NoError(t, model.DB.Where("channel_id = ?", 100).Order("model ASC").Find(&abilities).Error)
	require.Len(t, abilities, 2)
	require.Equal(t, "doubao-seed-2.0-pro", abilities[0].Model)
	require.Equal(t, "glm-5.2", abilities[1].Model)
}
