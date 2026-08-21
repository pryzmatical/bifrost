package mcp

import (
	"testing"

	"github.com/maximhq/bifrost/core/schemas"
)

// TestIsEnableable pins the guard that decides whether EnableClient will act
// on an entry.
//
// The regression it exists for: a first enable whose dial fails leaves the
// entry un-disabled (ExecutionConfig.Disabled=false) and Unstable, on purpose
// — a connection checker keeps retrying and the caller keeps its persisted
// disabled=false. A guard testing only State==Disabled would reject every
// subsequent enable of that client with "is not disabled (current state:
// unstable)", wedging it permanently while the admin's UI still shows the
// toggle off.
func TestIsEnableable(t *testing.T) {
	tests := []struct {
		name        string
		state       schemas.MCPConnectionState
		cfgDisabled bool
		nilConfig   bool
		want        bool
	}{
		{
			name:  "cleanly disabled",
			state: schemas.MCPConnectionStateDisabled, cfgDisabled: true, want: true,
		},
		{
			// The wedge, as EnableClient now leaves it: the dial failed, so
			// state went back to Disabled while ExecutionConfig.Disabled
			// stayed false (checker retrying, persisted row still enabled).
			// This has to stay enableable or the retry is rejected forever.
			name:  "disabled state with config already enabled (failed enable)",
			state: schemas.MCPConnectionStateDisabled, cfgDisabled: false, want: true,
		},
		{
			// Guards the inverse of the fix: if some future change parks a
			// failed enable at Unstable again while the config reads enabled,
			// the client is wedged — nothing can enable it and the badge
			// disagrees with the toggle. Kept as an explicit false so that
			// regression has to be an intentional edit to this expectation.
			name:  "unstable with config enabled is not enableable",
			state: schemas.MCPConnectionStateUnstable, cfgDisabled: false, want: false,
		},
		{
			// The DB row was rolled back to disabled while the runtime moved
			// on — enabling must remain possible so the two can reconverge.
			name:  "unstable with config still disabled",
			state: schemas.MCPConnectionStateUnstable, cfgDisabled: true, want: true,
		},
		{
			name:  "healthy client is not enableable",
			state: schemas.MCPConnectionStateHealthy, cfgDisabled: false, want: false,
		},
		{
			name:  "needs_reauth client is not enableable",
			state: schemas.MCPConnectionStateNeedsReauth, cfgDisabled: false, want: false,
		},
		{
			name:  "pending_verification client is not enableable",
			state: schemas.MCPConnectionStatePendingVerification, cfgDisabled: false, want: false,
		},
		{
			// Defensive: a Disabled entry is enableable on state alone, so a
			// missing config must not panic the guard.
			name:  "disabled with nil config",
			state: schemas.MCPConnectionStateDisabled, nilConfig: true, want: true,
		},
		{
			name:  "unstable with nil config is not enableable",
			state: schemas.MCPConnectionStateUnstable, nilConfig: true, want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := &schemas.MCPClientState{State: tt.state}
			if !tt.nilConfig {
				cs.ExecutionConfig = &schemas.MCPClientConfig{Disabled: tt.cfgDisabled}
			}
			if got := isEnableable(cs); got != tt.want {
				t.Errorf("isEnableable(state=%q, cfgDisabled=%v, nilConfig=%v) = %v, want %v",
					tt.state, tt.cfgDisabled, tt.nilConfig, got, tt.want)
			}
		})
	}
}
