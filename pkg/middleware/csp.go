package middleware

import (
	// 	"crypto/rand"
	// 	"encoding/base64"
	// 	"fmt"
	// 	"io"
	"net/http"
	// 	"regexp"
	// 	"strings"

	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/unrolled/secure" // LOGZ.IO GRAFANA CHANGE :: DEV-20823 Add package
	macaron "gopkg.in/macaron.v1"
)

// LOGZ.IO GRAFANA CHANGE :: DEV-20823 New add security hedaers func
// AddCSPHeader adds the Content Security Policy header.
func AddCSPHeader(cfg *setting.Cfg, logger log.Logger) macaron.Handler {
	return func(res http.ResponseWriter, req *http.Request, c *macaron.Context) {
		secureMiddleware := secure.New(createSecureOptions(cfg))

		nonce, _ := secureMiddleware.ProcessAndReturnNonce(res, req)
		ctx, ok := c.Data["ctx"].(*models.ReqContext)
		if !ok {
			panic("Failed to convert context into models.ReqContext")
		}

		ctx.RequestNonce = nonce
		logger.Debug("Successfully generated CSP nonce", "nonce", nonce)
	}
	// 	return func(w http.ResponseWriter, req *http.Request, c *macaron.Context) {
	// 		if !cfg.CSPEnabled {
	// 			return
	// 		}
	//
	// 		logger.Debug("Adding CSP header to response", "cfg", fmt.Sprintf("%p", cfg))
	//
	// 		ctx, ok := c.Data["ctx"].(*models.ReqContext)
	// 		if !ok {
	// 			panic("Failed to convert context into models.ReqContext")
	// 		}
	//
	// 		if cfg.CSPTemplate == "" {
	// 			logger.Debug("CSP template not configured, so returning 500")
	// 			ctx.JsonApiErr(500, "CSP template has to be configured", nil)
	// 			return
	// 		}
	//
	// 		var buf [16]byte
	// 		if _, err := io.ReadFull(rand.Reader, buf[:]); err != nil {
	// 			logger.Error("Failed to generate CSP nonce", "err", err)
	// 			ctx.JsonApiErr(500, "Failed to generate CSP nonce", err)
	// 		}
	//
	// 		nonce := base64.RawStdEncoding.EncodeToString(buf[:])
	// 		val := strings.ReplaceAll(cfg.CSPTemplate, "$NONCE", fmt.Sprintf("'nonce-%s'", nonce))
	//
	// 		re := regexp.MustCompile(`^\w+:(//)?`)
	// 		rootPath := re.ReplaceAllString(cfg.AppURL, "")
	// 		val = strings.ReplaceAll(val, "$ROOT_PATH", rootPath)
	// 		w.Header().Set("Content-Security-Policy", val)
	// 		ctx.RequestNonce = nonce
	// 		logger.Debug("Successfully generated CSP nonce", "nonce", nonce)
	// 	}
}

func createSecureOptions(cfg *setting.Cfg) secure.Options {
	secureOptions := secure.Options{
		ContentTypeNosniff: cfg.ContentTypeProtectionHeader,
		BrowserXssFilter:   cfg.XSSProtectionHeader,
		FrameDeny:          !cfg.AllowEmbedding,
		ForceSTSHeader:     (cfg.Protocol == setting.HTTPSScheme || cfg.Protocol == setting.HTTP2Scheme) && cfg.StrictTransportSecurity,
	}

	if secureOptions.ForceSTSHeader {
		secureOptions.STSSeconds = int64(cfg.StrictTransportSecurityMaxAge)
		secureOptions.STSPreload = cfg.StrictTransportSecurityPreload
		secureOptions.STSIncludeSubdomains = cfg.StrictTransportSecuritySubDomains
	}

	if cfg.CSPTemplate != "" {
		secureOptions.ContentSecurityPolicy = cfg.CSPTemplate
	}

	return secureOptions
}

// LOGZ.IO GRAFANA CHANGE :: DEV-20823 END
