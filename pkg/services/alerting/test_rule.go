package alerting

import (
	"context"
	"fmt"
	"time" // LOGZ.IO GRAFANA CHANGE :: DEV-17927 - import time

	"github.com/grafana/grafana/pkg/bus"
	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/models"
)

// AlertTestCommand initiates an test evaluation
// of an alert rule.
type AlertTestCommand struct {
	Dashboard     *simplejson.Json
	PanelID       int64
	OrgID         int64
	User          *models.SignedInUser
	LogzIoHeaders *models.LogzIoHeaders // LOGZ.IO GRAFANA CHANGE :: DEV-17927 - add LogzIoHeaders

	Result *EvalContext
}

func init() {
	bus.AddHandler("alerting", handleAlertTestCommand)
}

func handleAlertTestCommand(cmd *AlertTestCommand) error {
	dash := models.NewDashboardFromJson(cmd.Dashboard)

	extractor := NewDashAlertExtractor(dash, cmd.OrgID, cmd.User)
	alerts, err := extractor.GetAlerts()
	if err != nil {
		return err
	}

	for _, alert := range alerts {
		if alert.PanelId == cmd.PanelID {
			rule, err := NewRuleFromDBAlert(alert, true)
			if err != nil {
				return err
			}

			rule.LogzIoHeaders = cmd.LogzIoHeaders // LOGZ.IO GRAFANA CHANGE :: DEV-17927 - add LogzIoHeaders
			cmd.Result = testAlertRule(rule)
			return nil
		}
	}

	return fmt.Errorf("Could not find alert with panel id %d", cmd.PanelID)
}

func testAlertRule(rule *Rule) *EvalContext {
	handler := NewEvalHandler()

	context := NewEvalContext(context.Background(), rule, time.Now()) // LOGZ.IO GRAFANA CHANGE :: DEV-17927 - Add time.now()
	context.IsTestRun = true
	context.IsDebug = true

	handler.Eval(context)
	context.Rule.State = context.GetNewState()

	return context
}
