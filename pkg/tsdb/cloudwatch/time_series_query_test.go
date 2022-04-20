package cloudwatch

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatch/cloudwatchiface"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimeSeriesQuery(t *testing.T) {
	executor := newExecutor(nil, newTestConfig(), &fakeSessionCache{})
	now := time.Now()

	origNewCWClient := NewCWClient
	t.Cleanup(func() {
		NewCWClient = origNewCWClient
	})

	var cwClient fakeCWClient

	NewCWClient = func(sess *session.Session) cloudwatchiface.CloudWatchAPI {
		return &cwClient
	}

	t.Run("Custom metrics", func(t *testing.T) {
		cwClient = fakeCWClient{
			CloudWatchAPI: nil,
			GetMetricDataOutput: cloudwatch.GetMetricDataOutput{
				NextToken: nil,
				Messages:  []*cloudwatch.MessageData{},
				MetricDataResults: []*cloudwatch.MetricDataResult{
					{
						StatusCode: aws.String("Complete"), Id: aws.String("a"), Label: aws.String("NetworkOut"), Values: []*float64{aws.Float64(1.0)}, Timestamps: []*time.Time{&now},
					},
					{
						StatusCode: aws.String("Complete"), Id: aws.String("b"), Label: aws.String("NetworkIn"), Values: []*float64{aws.Float64(1.0)}, Timestamps: []*time.Time{&now},
					},
				},
			},
		}

		im := datasource.NewInstanceManager(func(s backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
			return datasourceInfo{}, nil
		})

		executor := newExecutor(im, newTestConfig(), &fakeSessionCache{})
		resp, err := executor.QueryData(context.Background(), &backend.QueryDataRequest{
			PluginContext: backend.PluginContext{
				DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{},
			},
			Queries: []backend.DataQuery{
				{
					RefID: "A",
					TimeRange: backend.TimeRange{
						From: now.Add(time.Hour * -2),
						To:   now.Add(time.Hour * -1),
					},
					JSON: json.RawMessage(`{
						"type":      "timeSeriesQuery",
						"subtype":   "metrics",
						"namespace": "AWS/EC2",
						"metricName": "NetworkOut",
						"expression": "",
						"dimensions": {
						  "InstanceId": "i-00645d91ed77d87ac"
						},
						"region": "us-east-2",
						"id": "a",
						"alias": "NetworkOut",
						"statistics": [
						  "Maximum"
						],
						"period": "300",
						"hide": false,
						"matchExact": true,
						"refId": "A"
					}`),
				},
				{
					RefID: "B",
					TimeRange: backend.TimeRange{
						From: now.Add(time.Hour * -2),
						To:   now.Add(time.Hour * -1),
					},
					JSON: json.RawMessage(`{
						"type":      "timeSeriesQuery",
						"subtype":   "metrics",
						"namespace": "AWS/EC2",
						"metricName": "NetworkIn",
						"expression": "",
						"dimensions": {
						"InstanceId": "i-00645d91ed77d87ac"
						},
						"region": "us-east-2",
						"id": "b",
						"alias": "NetworkIn",
						"statistics": [
						"Maximum"
						],
						"period": "300",
						"matchExact": true,
						"refId": "B"
					}`),
				},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "NetworkOut", resp.Responses["A"].Frames[0].Name)
		assert.Equal(t, "NetworkIn", resp.Responses["B"].Frames[0].Name)
	})

	t.Run("End time before start time should result in error", func(t *testing.T) {
		_, err := executor.executeTimeSeriesQuery(context.Background(), &backend.QueryDataRequest{Queries: []backend.DataQuery{{TimeRange: backend.TimeRange{
			From: now.Add(time.Hour * -1),
			To:   now.Add(time.Hour * -2),
		}}}})
		assert.EqualError(t, err, "invalid time range: start time must be before end time")
	})

	t.Run("End time equals start time should result in error", func(t *testing.T) {
		_, err := executor.executeTimeSeriesQuery(context.Background(), &backend.QueryDataRequest{Queries: []backend.DataQuery{{TimeRange: backend.TimeRange{
			From: now.Add(time.Hour * -1),
			To:   now.Add(time.Hour * -1),
		}}}})
		assert.EqualError(t, err, "invalid time range: start time must be before end time")
	})
}

func Test_QueryData_executeTimeSeriesQuery_no_alias_provided_frame_name_is_queryId_when_query_isMathExpression(t *testing.T) {
	origNewCWClient := NewCWClient
	t.Cleanup(func() {
		NewCWClient = origNewCWClient
	})
	var cwClient fakeCWClient
	NewCWClient = func(sess *session.Session) cloudwatchiface.CloudWatchAPI {
		return &cwClient
	}
	cwClient = fakeCWClient{
		GetMetricDataOutput: cloudwatch.GetMetricDataOutput{
			MetricDataResults: []*cloudwatch.MetricDataResult{
				{StatusCode: aws.String("Complete"), Id: aws.String("query id"), Label: aws.String("NetworkOut"),
					Values: []*float64{aws.Float64(1.0)}, Timestamps: []*time.Time{{}}},
			},
		},
	}
	im := datasource.NewInstanceManager(func(s backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
		return datasourceInfo{}, nil
	})
	executor := newExecutor(im, newTestConfig(), &fakeSessionCache{})

	resp, err := executor.QueryData(context.Background(), &backend.QueryDataRequest{
		PluginContext: backend.PluginContext{DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{}},
		Queries: []backend.DataQuery{
			{
				RefID:     "A",
				TimeRange: backend.TimeRange{From: time.Now().Add(time.Hour * -2), To: time.Now().Add(time.Hour * -1)},
				JSON: json.RawMessage(`{
						"type":      "timeSeriesQuery",
						"metricQueryType": 0,
						"metricEditorMode": 1,
						"namespace": "",
						"metricName": "",
						"region": "us-east-2",
						"id": "query id",
						"statistic": "Maximum",
						"period": "1200",
						"hide": false,
						"matchExact": true,
						"refId": "A"
					}`),
			},
		},
	})

	assert.NoError(t, err)
	assert.Equal(t, "query id", resp.Responses["A"].Frames[0].Name)
}

func Test_QueryData_executeTimeSeriesQuery_no_alias_provided_frame_name_depends_on_dimension_values_and_matchExact(t *testing.T) {
	origNewCWClient := NewCWClient
	t.Cleanup(func() {
		NewCWClient = origNewCWClient
	})
	var cwClient fakeCWClient
	NewCWClient = func(sess *session.Session) cloudwatchiface.CloudWatchAPI {
		return &cwClient
	}
	im := datasource.NewInstanceManager(func(s backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
		return datasourceInfo{}, nil
	})
	executor := newExecutor(im, newTestConfig(), &fakeSessionCache{})

	// "frame name is label when isInferredSearchExpression and not isMultiValuedDimensionExpression"
	testCasesReturningLabel := map[string]struct {
		dimensions string
		matchExact bool
	}{
		"with specific dimension, matchExact false": {dimensions: `"dimensions": {"InstanceId": ["some-instance"]},`, matchExact: false},
		"with wildcard dimension, matchExact false": {dimensions: `"dimensions": {"InstanceId": ["*"]},`, matchExact: false},
		"with wildcard dimension, matchExact true":  {dimensions: `"dimensions": {"InstanceId": ["*"]},`, matchExact: true},
		"without dimension, matchExact false":       {dimensions: "", matchExact: false},
	}
	for name, tc := range testCasesReturningLabel {
		t.Run(name, func(t *testing.T) {
			cwClient = fakeCWClient{
				GetMetricDataOutput: cloudwatch.GetMetricDataOutput{
					MetricDataResults: []*cloudwatch.MetricDataResult{
						{StatusCode: aws.String("Complete"), Id: aws.String("query id"), Label: aws.String("response label"),
							Values: []*float64{aws.Float64(1.0)}, Timestamps: []*time.Time{{}}},
					},
				},
			}

			resp, err := executor.QueryData(context.Background(), &backend.QueryDataRequest{
				PluginContext: backend.PluginContext{DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{}},
				Queries: []backend.DataQuery{
					{
						RefID:     "A",
						TimeRange: backend.TimeRange{From: time.Now().Add(time.Hour * -2), To: time.Now().Add(time.Hour * -1)},
						JSON: json.RawMessage(fmt.Sprintf(`{
						"type":      "timeSeriesQuery",
						"metricQueryType": 0,
						"metricEditorMode": 0,
						"namespace": "",
						"metricName": "",
						%s
						"region": "us-east-2",
						"id": "query id",
						"statistic": "Maximum",
						"period": "1200",
						"hide": false,
						"matchExact": %t,
						"refId": "A"
					}`, tc.dimensions, tc.matchExact)),
					},
				},
			})

			assert.NoError(t, err)
			assert.Equal(t, "response label", resp.Responses["A"].Frames[0].Name)
		})
	}

	// "frame name is metricName_stat when isInferredSearchExpression and not isMultiValuedDimensionExpression"
	testCasesReturningMetricStat := map[string]struct {
		dimensions string
		matchExact bool
	}{
		"with specific dimension, matchExact true": {dimensions: `"dimensions": {"InstanceId": ["some-instance"]},`, matchExact: true},
		"without dimension, matchExact true":       {dimensions: "", matchExact: true},
		"multi dimension, matchExact true":         {dimensions: `"dimensions": {"InstanceId": ["some-instance","another-instance"]},`, matchExact: true},
		"multi dimension, matchExact false":        {dimensions: `"dimensions": {"InstanceId": ["some-instance","another-instance"]},`, matchExact: false},
	}
	for name, tc := range testCasesReturningMetricStat {
		t.Run(name, func(t *testing.T) {
			cwClient = fakeCWClient{
				GetMetricDataOutput: cloudwatch.GetMetricDataOutput{
					MetricDataResults: []*cloudwatch.MetricDataResult{
						{StatusCode: aws.String("Complete"), Id: aws.String("query id"), Label: aws.String(""),
							Values: []*float64{aws.Float64(1.0)}, Timestamps: []*time.Time{{}}},
					},
				},
			}

			resp, err := executor.QueryData(context.Background(), &backend.QueryDataRequest{
				PluginContext: backend.PluginContext{DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{}},
				Queries: []backend.DataQuery{
					{
						RefID:     "A",
						TimeRange: backend.TimeRange{From: time.Now().Add(time.Hour * -2), To: time.Now().Add(time.Hour * -1)},
						JSON: json.RawMessage(fmt.Sprintf(`{
						"type":      "timeSeriesQuery",
						"metricQueryType": 0,
						"metricEditorMode": 0,
						"namespace": "",
						"metricName": "CPUUtilization",
						%s
						"region": "us-east-2",
						"id": "query id",
						"statistic": "Maximum",
						"period": "1200",
						"hide": false,
						"matchExact": %t,
						"refId": "A"
					}`, tc.dimensions, tc.matchExact)),
					},
				},
			})

			assert.NoError(t, err)
			assert.Equal(t, "CPUUtilization_Maximum", resp.Responses["A"].Frames[0].Name)
		})
	}
}

func Test_QueryData_executeTimeSeriesQuery_no_alias_provided_frame_name_is_label_when_query_type_is_MetricQueryTypeQuery(t *testing.T) {
	origNewCWClient := NewCWClient
	t.Cleanup(func() {
		NewCWClient = origNewCWClient
	})
	var cwClient fakeCWClient
	NewCWClient = func(sess *session.Session) cloudwatchiface.CloudWatchAPI {
		return &cwClient
	}

	cwClient = fakeCWClient{
		GetMetricDataOutput: cloudwatch.GetMetricDataOutput{
			MetricDataResults: []*cloudwatch.MetricDataResult{
				{StatusCode: aws.String("Complete"), Id: aws.String("query id"), Label: aws.String("response label"),
					Values: []*float64{aws.Float64(1.0)}, Timestamps: []*time.Time{{}}},
			},
		},
	}
	im := datasource.NewInstanceManager(func(s backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
		return datasourceInfo{}, nil
	})
	executor := newExecutor(im, newTestConfig(), &fakeSessionCache{})

	resp, err := executor.QueryData(context.Background(), &backend.QueryDataRequest{
		PluginContext: backend.PluginContext{DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{}},
		Queries: []backend.DataQuery{
			{
				RefID:     "A",
				TimeRange: backend.TimeRange{From: time.Now().Add(time.Hour * -2), To: time.Now().Add(time.Hour * -1)},
				JSON: json.RawMessage(`{
						"type":      "timeSeriesQuery",
						"metricQueryType": 1,
						"metricEditorMode": 0,
						"namespace": "",
						"metricName": "",
						"dimensions": {"InstanceId": ["some-instance"]},
						"region": "us-east-2",
						"id": "query id",
						"statistic": "Maximum",
						"period": "1200",
						"hide": false,
						"matchExact": false,
						"refId": "A"
					}`),
			},
		},
	})

	assert.NoError(t, err)
	assert.Equal(t, "response label", resp.Responses["A"].Frames[0].Name)
}

func Test_QueryData_timeSeriesQuery_GetMetricDataWithContext_passes_query_alias_as_label(t *testing.T) {
	origNewCWClient := NewCWClient
	t.Cleanup(func() {
		NewCWClient = origNewCWClient
	})
	var cwClient fakeCWClient
	NewCWClient = func(sess *session.Session) cloudwatchiface.CloudWatchAPI {
		return &cwClient
	}

	testCases := map[string]string{
		"not-yet-migrated legacy alias": "{{ period  }} some words {{   InstanceId }}",
		"migrated dynamic labels alias": "${PROP('Period')} some words ${PROP('Dim.InstanceId')}",
	}
	for name, inputAlias := range testCases {
		t.Run(name, func(t *testing.T) {
			cwClient = fakeCWClient{}
			im := datasource.NewInstanceManager(func(s backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
				return datasourceInfo{}, nil
			})
			executor := newExecutor(im, newTestConfig(), &fakeSessionCache{})

			_, err := executor.QueryData(context.Background(), &backend.QueryDataRequest{
				PluginContext: backend.PluginContext{DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{}},
				Queries: []backend.DataQuery{
					{
						RefID: "A",
						TimeRange: backend.TimeRange{
							From: time.Now().Add(time.Hour * -2),
							To:   time.Now().Add(time.Hour * -1),
						},
						JSON: json.RawMessage(fmt.Sprintf(`{
						"type":      "timeSeriesQuery",
						"subtype":   "metrics",
						"namespace": "AWS/EC2",
						"metricName": "NetworkOut",
						"expression": "",
						"dimensions": {
						  "InstanceId": "i-00645d91ed77d87ac"
						},
						"region": "us-east-2",
						"id": "a",
						"alias": "%s",
						"statistics": [
						  "Maximum"
						],
						"period": "300",
						"hide": false,
						"matchExact": true,
						"refId": "A"
					}`, inputAlias)),
					},
				},
			})

			assert.NoError(t, err)
			assert.Len(t, cwClient.calls.getMetricDataWithContext, 1)
			assert.Len(t, cwClient.calls.getMetricDataWithContext[0].MetricDataQueries, 1)
			require.NotNil(t, cwClient.calls.getMetricDataWithContext[0].MetricDataQueries[0].Label)

			assert.Equal(t, "${PROP('Period')} some words ${PROP('Dim.InstanceId')}", *cwClient.calls.getMetricDataWithContext[0].MetricDataQueries[0].Label)
		})
	}
}
