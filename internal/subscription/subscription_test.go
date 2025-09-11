package subscription

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"xhub-agent/pkg/logger"
)

func TestExtractUniqueSubIDs(t *testing.T) {
	// Create temporary log file for testing
	tmpDir, err := os.MkdirTemp("", "subscription-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	
	logFile := filepath.Join(tmpDir, "test.log")
	testLogger, err := logger.NewLogger(logFile, "debug")
	require.NoError(t, err)
	defer testLogger.Close()

	s := &SubscriptionClient{
		resolvedDomain: "test.example.com",
		logger:         testLogger,
	}

	// Test data
	inbounds := []*InboundInfo{
		{
			ID:     1,
			Enable: true,
			Settings: `{
				"clients": [
					{
						"email": "user1@example.com",
						"subId": "sub-id-1",
						"enable": true
					},
					{
						"email": "user2@example.com", 
						"subId": "sub-id-2",
						"enable": true
					}
				]
			}`,
		},
		{
			ID:     2,
			Enable: true,
			Settings: `{
				"clients": [
					{
						"email": "user3@example.com",
						"subId": "sub-id-1",
						"enable": true
					},
					{
						"email": "user4@example.com",
						"subId": "sub-id-3", 
						"enable": false
					}
				]
			}`,
		},
		{
			ID:     3,
			Enable: false,
			Settings: `{
				"clients": [
					{
						"email": "user5@example.com",
						"subId": "sub-id-4",
						"enable": true
					}
				]
			}`,
		},
	}

	result, err := s.ExtractUniqueSubIDs(inbounds)
	if err != nil {
		t.Errorf("ExtractUniqueSubIDs() error = %v", err)
		return
	}

	// Should only have 2 unique subIDs (sub-id-1 and sub-id-2)
	// sub-id-1 is duplicated, sub-id-3 is disabled, sub-id-4 is in disabled inbound
	expectedCount := 2
	if len(result) != expectedCount {
		t.Errorf("Expected %d unique SubIDs, got %d", expectedCount, len(result))
	}

	// Check if result contains expected subIDs
	subIDExists := make(map[string]bool)
	for _, sub := range result {
		subIDExists[sub.SubID] = true
	}

	if !subIDExists["sub-id-1"] {
		t.Error("Expected sub-id-1 to be in result")
	}
	if !subIDExists["sub-id-2"] {
		t.Error("Expected sub-id-2 to be in result")
	}
	if subIDExists["sub-id-3"] {
		t.Error("sub-id-3 should not be in result (client disabled)")
	}
	if subIDExists["sub-id-4"] {
		t.Error("sub-id-4 should not be in result (inbound disabled)")
	}
}
