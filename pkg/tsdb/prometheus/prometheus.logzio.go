// LOGZ.IO GRAFANA CHANGE :: (ALERTS) DEV-16492 Support external alert evaluation

package prometheus

import (
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/tsdb"
	"github.com/prometheus/client_golang/api"
	apiv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"net/http"
)

type logzIoAuthTransport struct {
	Transport     http.RoundTripper
	logzIoHeaders *models.LogzIoHeaders
}

func (lat logzIoAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if key, value := lat.logzIoHeaders.GetAuthHeader(); value != "" {
		req.Header.Set(key, value)
	}

	return lat.Transport.RoundTrip(req)
}

func (e *PrometheusExecutor) getLogzioAuthClient(dsInfo *models.DataSource, tsdbQuery *tsdb.TsdbQuery) (apiv1.API, error) {
	cfg := api.Config{
		Address:      dsInfo.Url,
		RoundTripper: e.Transport,
	}

	cfg.RoundTripper = logzIoAuthTransport{
		Transport:     e.Transport,
		logzIoHeaders: tsdbQuery.LogzIoHeaders,
	}

	client, err := api.NewClient(cfg)
	if err != nil {
		return nil, err
	}

	return apiv1.NewAPI(client), nil
}

// LOGZ.IO GRAFANA CHANGE :: end
