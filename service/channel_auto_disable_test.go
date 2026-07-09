package service

import (
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestShouldDisableChannelIgnoresTransientRateLimit429(t *testing.T) {
	setupShouldDisableChannelTest(t)

	err := types.NewOpenAIError(
		errors.New("Requests are too frequent. Please reduce your request frequency, wait a short moment, and retry your request."),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusTooManyRequests,
	)

	require.False(t, ShouldDisableChannel(err))
}

func TestShouldDisableChannelKeeps429DisabledWhenIgnoreKeywordsEmpty(t *testing.T) {
	setupShouldDisableChannelTest(t)
	operation_setting.AutomaticDisableIgnoreKeywords = []string{}

	err := types.NewOpenAIError(
		errors.New("Requests are too frequent. Please reduce your request frequency, wait a short moment, and retry your request."),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusTooManyRequests,
	)

	require.True(t, ShouldDisableChannel(err))
}

func TestShouldDisableChannelIgnoreKeywordsAreCaseInsensitive(t *testing.T) {
	setupShouldDisableChannelTest(t)
	operation_setting.AutomaticDisableIgnoreKeywords = []string{
		"REQUESTS ARE TOO FREQUENT",
	}

	err := types.NewOpenAIError(
		errors.New("requests are too frequent. please retry later."),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusTooManyRequests,
	)

	require.False(t, ShouldDisableChannel(err))
}

func TestShouldDisableChannelKeepsQuota429Disabled(t *testing.T) {
	setupShouldDisableChannelTest(t)

	err := types.NewOpenAIError(
		errors.New("You exceeded the weekly usage quota. We recommend upgrading your plan."),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusTooManyRequests,
	)

	require.True(t, ShouldDisableChannel(err))
}

func TestShouldDisableChannelKeeps401Disabled(t *testing.T) {
	setupShouldDisableChannelTest(t)

	err := types.NewOpenAIError(
		errors.New("invalid api key"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusUnauthorized,
	)

	require.True(t, ShouldDisableChannel(err))
}

func setupShouldDisableChannelTest(t *testing.T) {
	t.Helper()

	origEnabled := common.AutomaticDisableChannelEnabled
	origRanges := operation_setting.AutomaticDisableStatusCodeRanges
	origKeywords := operation_setting.AutomaticDisableKeywords
	origIgnoreKeywords := operation_setting.AutomaticDisableIgnoreKeywords

	common.AutomaticDisableChannelEnabled = true
	operation_setting.AutomaticDisableStatusCodeRanges = []operation_setting.StatusCodeRange{
		{Start: http.StatusUnauthorized, End: http.StatusUnauthorized},
		{Start: http.StatusTooManyRequests, End: http.StatusTooManyRequests},
	}
	operation_setting.AutomaticDisableKeywords = []string{
		"exceeded the weekly usage quota",
		"recommend upgrading your plan",
	}
	operation_setting.AutomaticDisableIgnoreKeywords = []string{
		"requests are too frequent",
		"reduce your request frequency",
		"wait a short moment",
	}

	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = origEnabled
		operation_setting.AutomaticDisableStatusCodeRanges = origRanges
		operation_setting.AutomaticDisableKeywords = origKeywords
		operation_setting.AutomaticDisableIgnoreKeywords = origIgnoreKeywords
	})
}
