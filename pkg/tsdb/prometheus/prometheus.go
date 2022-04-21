package prometheus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/grafana/grafana/pkg/services/ngalert/eval"
	"regexp"

	"github.com/grafana/grafana/pkg/tsdb/prometheus/promclient"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana/pkg/infra/httpclient"
	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/infra/tracing"
	"github.com/grafana/grafana/pkg/tsdb/intervalv2"
	apiv1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

var (
	plog         = log.New("tsdb.prometheus")
	legendFormat = regexp.MustCompile(`\{\{\s*(.+?)\s*\}\}`)
	safeRes      = 11000
)

type Service struct {
	intervalCalculator intervalv2.Calculator
	im                 instancemgmt.InstanceManager
	tracer             tracing.Tracer
}

func ProvideService(httpClientProvider httpclient.Provider, tracer tracing.Tracer) *Service {
	plog.Debug("initializing")
	// LOGZ.IO GRAFANA CHANGE :: DEV-31493 Override datasource URL on alert evaluation
	ip := &eval.LogzioInstanceProvider{
		Delegate: datasource.NewInstanceProvider(newInstanceSettings(httpClientProvider)),
	}
	im := instancemgmt.New(ip)
	// LOGZ.IO GRAFANA CHANGE :: end
	return &Service{
		intervalCalculator: intervalv2.NewCalculator(),
		im:                 im, // LOGZ.IO GRAFANA CHANGE :: DEV-31493 Override datasource URL on alert evaluation
		tracer:             tracer,
	}
}

func newInstanceSettings(httpClientProvider httpclient.Provider) datasource.InstanceFactoryFunc {
	return func(settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
		var jsonData promclient.JsonData
		err := json.Unmarshal(settings.JSONData, &jsonData)
		if err != nil {
			return nil, fmt.Errorf("error reading settings: %w", err)
		}

		p := promclient.NewProvider(settings, jsonData, httpClientProvider, plog)
		pc, err := promclient.NewProviderCache(p)
		if err != nil {
			return nil, err
		}

		mdl := DatasourceInfo{
			ID:           settings.ID,
			URL:          settings.URL,
			TimeInterval: jsonData.TimeInterval,
			getClient:    pc.GetClient,
		}

		return mdl, nil
	}
}

func (s *Service) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	if len(req.Queries) == 0 {
		return &backend.QueryDataResponse{}, fmt.Errorf("query contains no queries")
	}

	q := req.Queries[0]
	dsInfo, err := s.getDSInfo(req.PluginContext)
	if err != nil {
		return nil, err
	}

	var result *backend.QueryDataResponse
	switch q.QueryType {
	case "timeSeriesQuery":
		fallthrough
	default:
		result, err = s.executeTimeSeriesQuery(ctx, req, dsInfo)
	}

	return result, err
}

func (s *Service) getDSInfo(pluginCtx backend.PluginContext) (*DatasourceInfo, error) {
	i, err := s.im.Get(pluginCtx)
	if err != nil {
		return nil, err
	}

	instance := i.(DatasourceInfo)

	return &instance, nil
}

// IsAPIError returns whether err is or wraps a Prometheus error.
func IsAPIError(err error) bool {
	// Check if the right error type is in err's chain.
	var e *apiv1.Error
	return errors.As(err, &e)
}

func ConvertAPIError(err error) error {
	var e *apiv1.Error
	if errors.As(err, &e) {
		return fmt.Errorf("%s: %s", e.Msg, e.Detail)
	}
	return err
}
