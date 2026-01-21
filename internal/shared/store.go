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

	// 🔥 Seedr
	"github.com/Arunchegz/Stremthru-seedr/store/seedr"

	"github.com/golang-jwt/jwt/v5"
)

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

// 🚀 Seedr Store Client
var sdStore = seedr.NewStoreClient(&seedr.StoreClientConfig{
	HTTPClient: config.GetHTTPClient(config.StoreTunnel.GetTypeForAPI("seedr")),
	UserAgent:  config.StoreClientUserAgent,
})

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
	case store.StoreNameSeedr: // 👈 Seedr
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
	case store.StoreCodeSeedr: // 👈 Seedr
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

func CreateProxyLink(
	r *http.Request,
	link string,
	headers map[string]string,
	tunnelType config.TunnelType,
	expiresIn time.Duration,
	user, password string,
	shouldEncrypt bool,
	filename string,
) (string, error) {

	var encodedToken string

	if !shouldEncrypt && expiresIn == 0 {
		blob, err := json.Marshal(proxyLinkData{
			User:    user + ":" + password,
			Value:   link,
			Headers: headers,
			TunT:    tunnelType,
		})
		if err != nil {
			return "", err
		}
		encodedToken = "base64." + core.Base64EncodeByte(blob)
	} else {
		linkBlob := link
		if headers != nil {
			for k, v := range headers {
				linkBlob += "\n" + k + ": " + v
			}
		}

		var encLink string
		var encFormat string

		if shouldEncrypt {
			encryptedLink, err := core.Encrypt(password, linkBlob)
			if err != nil {
				return "", err
			}
			encLink = encryptedLink
			encFormat = core.EncryptionFormat
		} else {
			encLink = core.Base64Encode(linkBlob)
			encFormat = "base64"
		}

		claims := core.JWTClaims[proxyLinkTokenData]{
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:  "stremthru",
				Subject: user,
			},
			Data: &proxyLinkTokenData{
				EncLink:    encLink,
				EncFormat:  encFormat,
				TunnelType: tunnelType,
			},
		}

		if expiresIn != 0 {
			claims.RegisteredClaims.ExpiresAt =
				jwt.NewNumericDate(time.Now().Add(expiresIn))
		}

		token, err := core.CreateJWT(password, claims)
		if err != nil {
			return "", err
		}
		encodedToken = token
	}

	pLink := ExtractRequestBaseURL(r).JoinPath("/v0/proxy", encodedToken)

	if filename == "" {
		filename, _, _ = strings.Cut(filepath.Base(link), "?")
	}
	if filename != "" {
		pLink = pLink.JoinPath(filename)
	}

	return pLink.String(), nil
}