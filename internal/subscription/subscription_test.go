package subscription

import (
	"testing"
)

func TestReplaceHostWithLocalhost(t *testing.T) {
	s := &SubscriptionClient{}

	tests := []struct {
		name     string
		baseURL  string
		subID    string
		expected string
	}{
		{
			name:     "HTTPS with port",
			baseURL:  "https://xxx.example.com:52122/asdasdsafg/",
			subID:    "test-sub-id",
			expected: "https://127.0.0.1:52122/asdasdsafg/test-sub-id",
		},
		{
			name:     "HTTP without port",
			baseURL:  "http://example.com/sub/",
			subID:    "another-id",
			expected: "http://127.0.0.1/sub/another-id",
		},
		{
			name:     "URL without trailing slash",
			baseURL:  "https://domain.com:8080/path",
			subID:    "my-id",
			expected: "https://127.0.0.1:8080/path/my-id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := s.replaceHostWithLocalhost(tt.baseURL, tt.subID)
			if err != nil {
				t.Errorf("replaceHostWithLocalhost() error = %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("replaceHostWithLocalhost() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExtractUniqueSubIDs(t *testing.T) {
	s := &SubscriptionClient{}

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
