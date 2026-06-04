package python

// TODO: Define Python Service capability models and calls.
type CapabilityStatus struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

type CapabilitiesResponse map[string]CapabilityStatus
