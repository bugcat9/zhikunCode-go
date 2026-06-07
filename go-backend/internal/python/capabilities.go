package python

import (
	"context"
)

// TODO: Define Python Service capability models and calls.
type CapabilityStatus struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

type CapabilitiesResponse map[string]CapabilityStatus

func GetCapabilities(ctx context.Context) (CapabilitiesResponse, error) {
	client := NewClient("")
	return client.Capabilities(ctx)
}
