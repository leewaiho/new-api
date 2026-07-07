package controller

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

const (
	channelVendorCatalogImportModeAppend  = "append"
	channelVendorCatalogImportModeReplace = "replace"
)

type channelVendorCatalogImportRequest struct {
	ChannelID int    `json:"channel_id"`
	VendorID  int    `json:"vendor_id"`
	Mode      string `json:"mode"`
}

type channelVendorCatalogImportResult struct {
	ChannelID     int      `json:"channel_id"`
	ChannelName   string   `json:"channel_name"`
	VendorID      int      `json:"vendor_id"`
	VendorName    string   `json:"vendor_name"`
	Mode          string   `json:"mode"`
	ChannelModels []string `json:"channel_models"`
	CatalogModels []string `json:"catalog_models"`
	AddModels     []string `json:"add_models"`
	RemoveModels  []string `json:"remove_models"`
	KeepModels    []string `json:"keep_models"`
	NextModels    []string `json:"next_models"`
	ModelsChanged bool     `json:"models_changed"`
}

func normalizeVendorCatalogImportMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case channelVendorCatalogImportModeReplace:
		return channelVendorCatalogImportModeReplace
	default:
		return channelVendorCatalogImportModeAppend
	}
}

func buildChannelVendorCatalogImportResult(channel *model.Channel, vendor *model.Vendor, catalogModels []string, mode string) channelVendorCatalogImportResult {
	channelModels := normalizeModelNames(channel.GetModels())
	catalogModels = normalizeModelNames(catalogModels)

	addModels := subtractModelNames(catalogModels, channelModels)
	removeModels := make([]string, 0)
	nextModels := mergeModelNames(channelModels, catalogModels)
	if mode == channelVendorCatalogImportModeReplace {
		removeModels = subtractModelNames(channelModels, catalogModels)
		nextModels = catalogModels
	}
	keepModels := intersectModelNames(channelModels, catalogModels)

	return channelVendorCatalogImportResult{
		ChannelID:     channel.Id,
		ChannelName:   channel.Name,
		VendorID:      vendor.Id,
		VendorName:    vendor.Name,
		Mode:          mode,
		ChannelModels: channelModels,
		CatalogModels: catalogModels,
		AddModels:     addModels,
		RemoveModels:  removeModels,
		KeepModels:    keepModels,
		NextModels:    nextModels,
		ModelsChanged: !slicesEqualString(channelModels, nextModels),
	}
}

func slicesEqualString(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func loadChannelVendorCatalogImport(channelID int, vendorID int, mode string) (*model.Channel, *model.Vendor, []string, string, error) {
	if channelID <= 0 {
		return nil, nil, nil, "", fmt.Errorf("缺少渠道 ID")
	}
	if vendorID <= 0 {
		return nil, nil, nil, "", fmt.Errorf("缺少供应商 ID")
	}
	channel, err := model.GetChannelById(channelID, true)
	if err != nil {
		return nil, nil, nil, "", err
	}
	vendor, err := model.GetVendorByID(vendorID)
	if err != nil {
		return nil, nil, nil, "", err
	}
	catalogModels, err := model.GetEnabledModelNamesByVendorID(vendorID)
	if err != nil {
		return nil, nil, nil, "", err
	}
	if len(catalogModels) == 0 {
		return nil, nil, nil, "", fmt.Errorf("供应商目录没有启用模型")
	}
	return channel, vendor, catalogModels, normalizeVendorCatalogImportMode(mode), nil
}

func PreviewChannelVendorCatalogModels(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	vendorID, err := strconv.Atoi(c.Query("vendor_id"))
	if err != nil {
		common.ApiErrorMsg(c, "缺少供应商 ID")
		return
	}
	channel, vendor, catalogModels, mode, err := loadChannelVendorCatalogImport(channelID, vendorID, c.Query("mode"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildChannelVendorCatalogImportResult(channel, vendor, catalogModels, mode))
}

func ApplyChannelVendorCatalogModels(c *gin.Context) {
	var req channelVendorCatalogImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	channel, vendor, catalogModels, mode, err := loadChannelVendorCatalogImport(req.ChannelID, req.VendorID, req.Mode)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	result := buildChannelVendorCatalogImportResult(channel, vendor, catalogModels, mode)
	if result.ModelsChanged {
		nextModels := strings.Join(result.NextModels, ",")
		tx := model.DB.Begin()
		if tx.Error != nil {
			common.ApiError(c, tx.Error)
			return
		}
		if err := tx.Model(&model.Channel{}).Where("id = ?", channel.Id).Update("models", nextModels).Error; err != nil {
			tx.Rollback()
			common.ApiError(c, err)
			return
		}
		channel.Models = nextModels
		if err := channel.UpdateAbilities(tx); err != nil {
			tx.Rollback()
			common.ApiError(c, err)
			return
		}
		if err := tx.Commit().Error; err != nil {
			common.ApiError(c, err)
			return
		}
		model.InitChannelCache()
		model.RefreshPricing()
	}

	recordManageAudit(c, "channel.vendor_catalog_import", map[string]interface{}{
		"channel_id": channel.Id,
		"vendor_id":  vendor.Id,
		"mode":       mode,
		"added":      len(result.AddModels),
		"removed":    len(result.RemoveModels),
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    result,
	})
}

type volcengineCodingPlanModelSeed struct {
	ModelName   string
	Description string
	Tags        string
}

var volcengineCodingPlanModels = []volcengineCodingPlanModelSeed{
	{ModelName: "MiniMax-M2.7", Description: "Volcengine CodingPlan MiniMax M2.7", Tags: "codingplan,minimax"},
	{ModelName: "MiniMax-M3", Description: "Volcengine CodingPlan MiniMax M3", Tags: "codingplan,minimax"},
	{ModelName: "deepseek-v4-flash", Description: "Volcengine CodingPlan DeepSeek V4 Flash", Tags: "codingplan,deepseek"},
	{ModelName: "deepseek-v4-pro", Description: "Volcengine CodingPlan DeepSeek V4 Pro", Tags: "codingplan,deepseek"},
	{ModelName: "doubao-seed-2.0-code", Description: "Volcengine CodingPlan Doubao Seed 2.0 Code", Tags: "codingplan,doubao"},
	{ModelName: "doubao-seed-2.0-lite", Description: "Volcengine CodingPlan Doubao Seed 2.0 Lite", Tags: "codingplan,doubao"},
	{ModelName: "doubao-seed-2.0-pro", Description: "Volcengine CodingPlan Doubao Seed 2.0 Pro", Tags: "codingplan,doubao"},
	{ModelName: "glm-5.2", Description: "Volcengine CodingPlan GLM 5.2", Tags: "codingplan,zhipu"},
	{ModelName: "gpt-5.4-mini", Description: "Volcengine CodingPlan GPT 5.4 Mini", Tags: "codingplan,gpt"},
	{ModelName: "kimi-k2.7-code", Description: "Volcengine CodingPlan Kimi K2.7 Code", Tags: "codingplan,kimi"},
}

func EnsureVolcengineCodingPlanVendorCatalog(c *gin.Context) {
	vendor, createdVendor, err := model.EnsureVendorByName("Volcengine CodingPlan", "Volcengine CodingPlan subscription model catalog", "VolcEngine")
	if err != nil {
		common.ApiError(c, err)
		return
	}

	endpointsBytes, err := common.Marshal(map[string]common.EndpointInfo{
		"anthropic":       {Path: "/v1/messages", Method: "POST"},
		"openai":          {Path: "/v1/chat/completions", Method: "POST"},
		"openai-response": {Path: "/v1/responses", Method: "POST"},
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	endpoints := string(endpointsBytes)

	createdModels := make([]string, 0)
	updatedModels := make([]string, 0)
	for _, seed := range volcengineCodingPlanModels {
		created, err := model.EnsureModelCatalogEntry(model.Model{
			ModelName:    seed.ModelName,
			Description:  seed.Description,
			Icon:         "VolcEngine",
			Tags:         seed.Tags,
			VendorID:     vendor.Id,
			Endpoints:    endpoints,
			Status:       1,
			SyncOfficial: 0,
			NameRule:     model.NameRuleExact,
		})
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if created {
			createdModels = append(createdModels, seed.ModelName)
		} else {
			updatedModels = append(updatedModels, seed.ModelName)
		}
	}
	sort.Strings(createdModels)
	sort.Strings(updatedModels)
	model.RefreshPricing()

	common.ApiSuccess(c, gin.H{
		"vendor":         vendor,
		"created_vendor": createdVendor,
		"created_models": createdModels,
		"updated_models": updatedModels,
		"model_count":    len(volcengineCodingPlanModels),
	})
}
