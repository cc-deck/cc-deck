package openshell

import (
	"fmt"

	v1 "github.com/rhuss/openshell-sdk-go/openshell/v1"
)

// NewSDKClient creates a new SDK client from a GatewayConfig.
func NewSDKClient(cfg GatewayConfig) (v1.ClientInterface, error) {
	sdkCfg, err := cfg.ToSDKConfig()
	if err != nil {
		return nil, fmt.Errorf("building SDK config: %w", err)
	}
	client, err := v1.NewClient(sdkCfg)
	if err != nil {
		return nil, fmt.Errorf("creating SDK client: %w", err)
	}
	return client, nil
}
