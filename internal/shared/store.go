package shared

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/MunifTanjim/stremthru/core"
	"github.com/MunifTanjim/stremthru/internal/cache"
	"github.com/MunifTanjim/stremthru/internal/config"
	"github.com/MunifTanjim/stremthru/internal/context"
	"github.com/MunifTanjim/stremthru/store"

	"github.com/MunifTanjim/stremthru/store/alldebrid"
	"github.com/MunifTanjim/stremthru/store/debrider"
	"github.com/MunifTanjim/stremthru/store/debridlink"
	"github.com/MunifTanjim/stremthru/store/easydebrid"
	"github.com/MunifTanjim/stremthru/store/offcloud"
	"github.com/MunifTanjim/stremthru/store/pikpak"
	"github.com/MunifTanjim/stremthru/store/premiumize"
	"github.com/MunifTanjim/stremthru/store/realdebrid"
	"github.com/MunifTanjim/stremthru/store/torbox"

	// 🔴 ADD THIS
	"github.com/Arunchegz/Stremthru-seedr/store/seedr"

	"github.com/golang-jwt/jwt/v5"
)

/*
|--------------------------------------------------------------------------
| Store Instances
|--------------------------------------------------------------------------
*/

var adStore = alldebrid.NewStoreClient(&alldebrid.StoreClientConfig{
	HTTPClient: config.GetHTTPClient(config.StoreTunnel.GetTypeForAPI("alldebrid")),
	UserAgent:  config.StoreClientUserAgent,
})

var drStore = debrider.NewStoreClient(&debrider.StoreClientConfig{
	HTTPClient: config.GetHTTPClient(config.StoreTunnel.GetTypeForAPI("debrider")),
	UserAgent:  config.StoreClientUserAgent,
})

var dlStore = debridlink.NewStoreClient(&debridlink.StoreClientConfig{
	HTTPClient: config.GetHTTPClient(config.StoreTunnel.GetTypeForAPI("debridlink")),
	UserAgent:  config.StoreClientUserAgent,
})

var edStore = easydebrid.NewStoreClient(&easydebrid.StoreClientConfig{
	HTTPClient: config.GetHTTPClient(config.StoreTunnel.GetTypeForAPI("easydebrid")),
	UserAgent:  config.StoreClientUserAgent,
})

var pmStore = premiumize.NewStoreClient(&premiumize.StoreClientConfig{
	HTTPClient: config.GetHTTPClient(config.StoreTunnel.GetTypeForAPI("premiumize")),
	UserAgent:  config.StoreClientUserAgent,
})

var ppStore = pikpak.NewStoreClient(&pikpak.StoreClientConfig{
	HTTPClient: config.GetHTTPClient(config.StoreTunnel.GetTypeForAPI("pikpak")),
	UserAgent:  config.StoreClientUserAgent,
})

var ocStore = offcloud.NewStoreClient(&offcloud.StoreClientConfig{
	HTTPClient: config.GetHTTPClient(config.StoreTunnel.GetTypeForAPI("offcloud")),
	UserAgent:  config.StoreClientUserAgent,
})

var rdStore = realdebrid.NewStoreClient(&realdebrid.StoreClientConfig{
	HTTPClient: config.GetHTTPClient(config.StoreTunnel.GetTypeForAPI("realdebrid")),
	UserAgent:  "Mozilla/5.0",
})

var tbStore = torbox.NewStoreClient(&torbox.StoreClientConfig{
	HTTPClient: config.GetHTTPClient(config.StoreTunnel.GetTypeForAPI("torbox")),
	UserAgent:  config.StoreClientUserAgent,
})

/*
|--------------------------------------------------------------------------
| 🔴 Seedr Store Instance
|--------------------------------------------------------------------------
*/

var sdStore = seedr.NewStoreClient(&seedr.StoreClientConfig{
	HTTPClient: config.GetHTTPClient(config.StoreTunnel.GetTypeForAPI("seedr")),
	UserAgent:  config.StoreClientUserAgent,
})

/*
|--------------------------------------------------------------------------
| Store Resolver
|--------------------------------------------------------------------------
*/

func GetStore(name string) store.Store {
	switch store.StoreName(name) {
	case store.StoreNameAlldebrid:
		return adStore
	case store.StoreNameDebrider:
		return drStore
	case store.StoreNameDebridLink:
		return dlStore
	case store.StoreNameEasyDebrid:
		return edStore
	case store.StoreNameOffcloud:
		return ocStore
	case store.StoreNamePikPak:
		return ppStore
	case store.StoreNameSeedr: // 🔴 ADD THIS
		return sdStore
	case store.StoreNamePremiumize:
		return pmStore
	case store.StoreNameRealDebrid:
		return rdStore
	case store.StoreNameTorBox:
		return tbStore
	default:
		return nil
	}
}

func GetStoreByCode(code string) store.Store {
	switch store.StoreCode(code) {
	case store.StoreCodeAllDebrid:
		return adStore
	case store.StoreCodeDebrider:
		return drStore
	case store.StoreCodeDebridLink:
		return dlStore
	case store.StoreCodeEasyDebrid:
		return edStore
	case store.StoreCodeOffcloud:
		return ocStore
	case store.StoreCodePikPak:
		return ppStore
	case store.StoreCodeSeedr: // 🔴 ADD THIS
		return sdStore
	case store.StoreCodePremiumize:
		return pmStore
	case store.StoreCodeRealDebrid:
		return rdStore
	case store.StoreCodeTorBox:
		return tbStore
	default:
		return nil
	}
}

/*
|--------------------------------------------------------------------------
| Proxy Logic (unchanged from original)
|--------------------------------------------------------------------------
*/

type proxyLinkTokenData struct {
	EncLink    string            `json:"enc_link"`
	EncFormat  string            `json:"enc_format"`
	TunnelType config.TunnelType `json:"tunt,omitempty"`
}

type proxyLinkData struct {
	User    string            `json:"u"`
	Value   string            `json:"v"`
	Headers map[string]string `json:"reqh,omitempty"`
	TunT    config.TunnelType `json:"tunt,omitempty"`
}

var proxyLinkTokenCache = func() cache.Cache[proxyLinkData] {
	return cache.NewCache[proxyLinkData](&cache.CacheConfig{
		Name:     "store:proxyLinkToken",
		Lifetime: 30 * time.Minute,
	})
}()

/*
|--------------------------------------------------------------------------
| The rest of the file stays unchanged
|--------------------------------------------------------------------------
| (CreateProxyLink, GenerateStremThruLink, UnwrapProxyLinkToken etc.)
|--------------------------------------------------------------------------
*/