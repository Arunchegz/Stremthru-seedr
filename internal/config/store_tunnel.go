package config

type TunnelType string

const (
	TunnelTypeDirect TunnelType = "direct"
	TunnelTypeProxy  TunnelType = "proxy"
)

type storeTunnel struct{}

var StoreTunnel = storeTunnel{}

// Used for API calls
func (storeTunnel) GetTypeForAPI(store string) TunnelType {
	switch store {
	case "alldebrid", "debrider", "debridlink", "easydebrid", "offcloud",
		"premiumize", "realdebrid", "torbox":
		return TunnelTypeProxy

	case "pikpak":
		return TunnelTypeDirect

	case "seedr": // <-- THIS is why Seedr works as cloud
		return TunnelTypeDirect

	default:
		return TunnelTypeDirect
	}
}

// Used for streaming
func (storeTunnel) GetTypeForStream(store string) TunnelType {
	switch store {
	case "alldebrid", "debrider", "debridlink", "easydebrid", "offcloud",
		"premiumize", "realdebrid", "torbox":
		return TunnelTypeProxy

	case "pikpak":
		return TunnelTypeDirect

	case "seedr": // <-- ADD
		return TunnelTypeDirect

	default:
		return TunnelTypeDirect
	}
}