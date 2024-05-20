package middleware

import (
	"net/http" // LOGZ.IO GRAFANA CHANGE :: DEV-20823 Add package

	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/setting" // LOGZ.IO GRAFANA CHANGE :: DEV-20823 Add package
	"github.com/unrolled/secure"             // LOGZ.IO GRAFANA CHANGE :: DEV-20823 Add package
	macaron "gopkg.in/macaron.v1"
)

const HeaderNameNoBackendCache = "X-Grafana-NoCache"

func HandleNoCacheHeader() macaron.Handler {
	return func(ctx *models.ReqContext) {
		ctx.SkipCache = ctx.Req.Header.Get(HeaderNameNoBackendCache) == "true"
	}
}

// LOGZ.IO GRAFANA CHANGE :: DEV-20823 New add security hedaers func
func AddSeceureResponseHeaders() macaron.Handler {
	return func(res http.ResponseWriter, req *http.Request, c *macaron.Context) {

		secureMiddleware := secure.New(createSecureOptions())

		nonce, _ := secureMiddleware.ProcessAndReturnNonce(res, req)
		ctx, ok := c.Data["ctx"].(*models.ReqContext)
		if !ok {
			return
		}

		ctx.RequestNonce = nonce
	}
}

func createSecureOptions() secure.Options {
	secureOptions := secure.Options{
		ContentTypeNosniff: setting.ContentTypeProtectionHeader,
		BrowserXssFilter:   setting.XSSProtectionHeader,
		FrameDeny:          !setting.AllowEmbedding,
		ForceSTSHeader:     (setting.Protocol == setting.HTTPSScheme || setting.Protocol == setting.HTTP2Scheme) && setting.StrictTransportSecurity,
	}

	if secureOptions.ForceSTSHeader {
		secureOptions.STSSeconds = int64(setting.StrictTransportSecurityMaxAge)
		secureOptions.STSPreload = setting.StrictTransportSecurityPreload
		secureOptions.STSIncludeSubdomains = setting.StrictTransportSecuritySubDomains
	}

	if setting.ContentSecurityPolicy != "" {
		secureOptions.ContentSecurityPolicy = setting.ContentSecurityPolicy
	}

	return secureOptions
}

// LOGZ.IO GRAFANA CHANGE :: end
