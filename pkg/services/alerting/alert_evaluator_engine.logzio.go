// LOGZ.IO GRAFANA CHANGE :: DEV-17927 - LOGZ file - custom logzio engine.go
package alerting

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/infra/tracing"
	"github.com/grafana/grafana/pkg/infra/usagestats"
	"github.com/grafana/grafana/pkg/services/encryption"
	"github.com/grafana/grafana/pkg/services/rendering"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/grafana/grafana/pkg/tsdb/legacydata"
	"go.opentelemetry.io/otel/attribute"

	"github.com/grafana/grafana/pkg/bus"
	"github.com/grafana/grafana/pkg/models"
)

type AlertEvaluatorEngine struct {
	RenderService    rendering.Service
	Bus              bus.Bus
	RequestValidator models.PluginRequestValidator
	DataService      legacydata.RequestHandler

	evalHandler   evalHandler
	ruleReader    ruleReader
	resultHandler resultHandler

	log    log.Logger
	tracer tracing.Tracer
}

type EvaluateAlertCommand struct {
	Alert             *models.Alert
	EvalTime          time.Time
	LogzIoHeaders     *models.LogzIoHeaders
	DataSourceUrl     string
	CustomDataSources []*models.DataSource

	Result *EvalContext
}

type EvaluateAlertByIdCommand struct {
	AlertId       int64
	EvalTime      time.Time
	LogzIoHeaders *models.LogzIoHeaders

	Result *EvalContext
}

func ProvideAlertEvaluatorEngine(renderer rendering.Service, bus bus.Bus, requestValidator models.PluginRequestValidator,
	dataService legacydata.RequestHandler, usageStatsService usagestats.Service, encryptionService encryption.Internal,
	cfg *setting.Cfg, tracer tracing.Tracer) *AlertEvaluatorEngine {
	e := &AlertEvaluatorEngine{ // LOGZ.IO GRAFANA CHANGE :: Upgrade to 8.4.0
		tracer:           tracer,           // LOGZ.IO GRAFANA CHANGE :: Upgrade to 8.4.0
		RequestValidator: requestValidator, // LOGZ.IO GRAFANA CHANGE :: Upgrade to 8.4.0
		DataService:      dataService,      // LOGZ.IO GRAFANA CHANGE :: Upgrade to 8.4.0
	} // LOGZ.IO GRAFANA CHANGE :: Upgrade to 8.4.0
	e.evalHandler = NewEvalHandler(e.DataService)
	e.ruleReader = newRuleReader()
	e.log = log.New("check-alerting.engine")
	e.resultHandler = newResultHandler(e.RenderService, encryptionService.GetDecryptedValue)
	e.Bus = bus
	e.Bus.AddHandler(e.HandleEvaluateAlertCommand)     // LOGZ.IO GRAFANA CHANGE :: Add our own alerts handler
	e.Bus.AddHandler(e.HandleEvaluateAlertByIdCommand) // LOGZ.IO GRAFANA CHANGE :: Add our own alerts by id handler

	return e
}

func (e *AlertEvaluatorEngine) processJob(attemptID int, attemptChan chan int, cancelChan chan context.CancelFunc, job *Job) {
	alertCtx, cancelFn := context.WithTimeout(context.Background(), setting.AlertingEvaluationTimeout)
	cancelChan <- cancelFn
	alertCtx, span := e.tracer.Start(alertCtx, "alert execution")

	evalContext := NewEvalContext(alertCtx, job.Rule, job.EvalTime, e.RequestValidator) // LOGZ.IO GRAFANA CHANGE :: DEV-17927 - Add eval time
	evalContext.Ctx = alertCtx

	e.evalHandler.Eval(evalContext)
	span.SetAttributes("alertId", evalContext.Rule.ID, attribute.Key("alertId").Int64(evalContext.Rule.ID))
	span.SetAttributes("dashboardId", evalContext.Rule.DashboardID, attribute.Key("dashboardId").Int64(evalContext.Rule.DashboardID))
	span.SetAttributes("firing", evalContext.Firing, attribute.Key("firing").Bool(evalContext.Firing))
	span.SetAttributes("nodatapoints", evalContext.NoDataFound, attribute.Key("nodatapoints").Bool(evalContext.NoDataFound))
	span.SetAttributes("attemptID", attemptID, attribute.Key("attemptID").Int(attemptID))

	if evalContext.Error != nil {
		span.RecordError(evalContext.Error)
		span.AddEvents(
			[]string{"error", "message"},
			[]tracing.EventValue{
				{Str: fmt.Sprintf("%v", evalContext.Error)},
				{Str: "alerting execution attempt failed"},
			})
		// LOGZ.IO GRAFANA CHANGE :: DEV-17927 - remove retries
	}

	// create new context with timeout for notifications
	resultHandleCtx, resultHandleCancelFn := context.WithTimeout(context.Background(), setting.AlertingNotificationTimeout)
	cancelChan <- resultHandleCancelFn

	// override the context used for evaluation with a new context for notifications.
	// This makes it possible for notifiers to execute when datasources
	// dont respond within the timeout limit. We should rewrite this so notifications
	// dont reuse the evalContext and get its own context.
	evalContext.Ctx = resultHandleCtx
	evalContext.Rule.State = evalContext.GetNewState()
	if err := e.resultHandler.handle(evalContext); err != nil {
		if errors.Is(err, context.Canceled) {
			e.log.Debug("Result handler returned context.Canceled")
		} else if errors.Is(err, context.DeadlineExceeded) {
			e.log.Debug("Result handler returned context.DeadlineExceeded")
		} else {
			e.log.Error("Failed to handle result", "err", err)
		}
	}

	job.Result = evalContext // LOGZ.IO GRAFANA CHANGE :: DEV-17927 - Set the job result

	span.End()
	e.log.Debug("Job Execution completed", "timeMs", evalContext.GetDurationMs(), "alertId", evalContext.Rule.ID, "name", evalContext.Rule.Name, "firing", evalContext.Firing, "attemptID", attemptID)
	close(attemptChan)
}

func (e *AlertEvaluatorEngine) HandleEvaluateAlertCommand(ctx context.Context, cmd *EvaluateAlertCommand) error {
	rule, err := NewRuleFromDBAlert(ctx, cmd.Alert, true)
	if err != nil {
		e.log.Error("Could not build alert model for rule", "ruleId", cmd.Alert.Id, "error", err)
		return nil
	}
	rule.LogzIoHeaders = cmd.LogzIoHeaders
	rule.DataSourceUrl = cmd.DataSourceUrl         // LOGZ.IO GRAFANA CHANGE :: DEV-19069 - add DataSourceUrl
	rule.CustomDataSources = cmd.CustomDataSources // LOGZ.IO GRAFANA CHANGE :: DEV-21780 - support prometheus alerts

	result := e.handleRule(rule, cmd.EvalTime)
	cmd.Result = result

	return nil
}

func (e *AlertEvaluatorEngine) HandleEvaluateAlertByIdCommand(ctx context.Context, cmd *EvaluateAlertByIdCommand) error {
	query := &models.GetAlertByIdQuery{Id: cmd.AlertId}
	rule := e.ruleReader.fetchOne(ctx, query)
	if rule == nil {
		e.log.Error("Could not find alert rule", "ruleId", cmd.AlertId)
		return nil
	}

	rule.LogzIoHeaders = cmd.LogzIoHeaders
	result := e.handleRule(rule, cmd.EvalTime)
	cmd.Result = result

	return nil
}

func (e *AlertEvaluatorEngine) handleRule(rule *Rule, evalTime time.Time) *EvalContext {
	var job *Job
	job = &Job{}
	job.SetRunning(false)

	job.Rule = rule
	job.EvalTime = evalTime

	offset := (rule.Frequency * 1000)
	job.Offset = int64(math.Floor(float64(offset) / 1000))
	if job.Offset == 0 { //zero offset causes division with 0 panics.
		job.Offset = 1
	}

	cancelChan := make(chan context.CancelFunc, setting.AlertingMaxAttempts*2)
	attemptChan := make(chan int, 1)
	attemptChan <- 1

	e.processJob(1, attemptChan, cancelChan, job)

	return job.Result
}
