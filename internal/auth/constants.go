package auth

import (
	"os"
	"time"
)

const (
	// DefaultBastioBaseURL is the default base URL for the Bastio API
	DefaultBastioBaseURL = "https://api.bastio.com"

	// DefaultBastioGatewayURL is the default URL for the Bastio AI Gateway
	// Note: Don't include /v1 - the Anthropic SDK adds it automatically
	DefaultBastioGatewayURL = "https://api.bastio.com"

	// DefaultBastioWebURL is the default base URL for the Bastio web frontend
	DefaultBastioWebURL = "https://www.bastio.com"

	// CLIVersion is the version of the CLI for device registration
	CLIVersion = "1.0.0"

	// DefaultDeviceFlowTimeout is the maximum time to wait for device authorization
	DefaultDeviceFlowTimeout = 15 * time.Minute

	// DefaultHTTPTimeout is the default timeout for HTTP requests
	DefaultHTTPTimeout = 30 * time.Second
)

// GetBastioBaseURL returns the Bastio API base URL, checking env var first
func GetBastioBaseURL() string {
	if url := os.Getenv("BASTIO_API_URL"); url != "" {
		return url
	}
	return DefaultBastioBaseURL
}

// GetBastioGatewayURL returns the Bastio Gateway URL, checking env var first
func GetBastioGatewayURL() string {
	if url := os.Getenv("BASTIO_GATEWAY_URL"); url != "" {
		return url
	}
	return DefaultBastioGatewayURL
}

// GetBastioWebURL returns the Bastio web frontend URL, checking env var first
func GetBastioWebURL() string {
	if url := os.Getenv("BASTIO_WEB_URL"); url != "" {
		return url
	}
	return DefaultBastioWebURL
}
