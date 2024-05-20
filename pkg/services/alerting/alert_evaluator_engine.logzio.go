// LOGZ.IO GRAFANA CHANGE :: DEV-17927 - LOGZ file - custom logzio engine.go
package alerting

import (
	"context"
	"math"
	"time"
	"errors"

	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/registry"
	"github.com/grafana/grafana/pkg/services/rendering"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	tlog "github.com/opentracing/opentracing-go/log"

	"github.com/grafana/grafana/pkg/bus"
	"github.com/grafana/grafana/pkg/models"
)

type AlertEvaluatorEngine struct {
	RenderService rendering.Service `inject:""`
	Bus           bus.Bus           `inject:""`

	evalHandler   evalHandler
	ruleReader    ruleReader
	resultHandler resultHandler

	log log.Logger
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

func init() {
	registry.RegisterService(&AlertEvaluatorEngine{}) // LOGZ.IO GRAFANA CHANGE :: DEV-17927 - Register the new alert check struct
}

func (e *AlertEvaluatorEngine) Init() error {
	e.evalHandler = NewEvalHandler()
	e.ruleReader = newRuleReader()
	e.log = log.New("check-alerting.engine")
	e.resultHandler = newResultHandler(e.RenderService)
	e.Bus.AddHandler(e.HandleEvaluateAlertCommand)     // LOGZ.IO GRAFANA CHANGE :: Add our own alerts handler
	e.Bus.AddHandler(e.HandleEvaluateAlertByIdCommand) // LOGZ.IO GRAFANA CHANGE :: Add our own alerts by id handler

	return nil
}

func (e *AlertEvaluatorEngine) processJob(attemptID int, attemptChan chan int, cancelChan chan context.CancelFunc, job *Job) {
	alertCtx, cancelFn := context.WithTimeout(context.Background(), setting.AlertingEvaluationTimeout)
	cancelChan <- cancelFn
	span := opentracing.StartSpan("alert execution")
	alertCtx = opentracing.ContextWithSpan(alertCtx, span)

	evalContext := NewEvalContext(alertCtx, job.Rule, job.EvalTime) // LOGZ.IO GRAFANA CHANGE :: DEV-17927 - Add eval time
	evalContext.Ctx = alertCtx

	e.evalHandler.Eval(evalContext)
	span.SetTag("alertId", evalContext.Rule.ID)
	span.SetTag("dashboardId", evalContext.Rule.DashboardID)
	span.SetTag("firing", evalContext.Firing)
	span.SetTag("nodatapoints", evalContext.NoDataFound)
	span.SetTag("attemptID", attemptID)

	if evalContext.Error != nil {
		ext.Error.Set(span, true)
		span.LogFields(
			tlog.Error(evalContext.Error),
			tlog.String("message", "alerting execution attempt failed"),
		)
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

	span.Finish()
	e.log.Debug("Job Execution completed", "timeMs", evalContext.GetDurationMs(), "alertId", evalContext.Rule.ID, "name", evalContext.Rule.Name, "firing", evalContext.Firing, "attemptID", attemptID)
	close(attemptChan)
}

func (e *AlertEvaluatorEngine) HandleEvaluateAlertCommand(cmd *EvaluateAlertCommand) error {
	rule, err := NewRuleFromDBAlert(cmd.Alert, true)
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

func (e *AlertEvaluatorEngine) HandleEvaluateAlertByIdCommand(cmd *EvaluateAlertByIdCommand) error {
	query := &models.GetAlertByIdQuery{Id: cmd.AlertId}
	rule := e.ruleReader.fetchOne(query)
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
