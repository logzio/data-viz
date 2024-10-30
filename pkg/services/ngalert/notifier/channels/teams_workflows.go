package channels

import (
	"context"
	"encoding/json"
	"github.com/pkg/errors"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"

	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/notifications"
)

// TeamsWorkflowsNotifier is responsible for sending
// alert notifications to Microsoft teams with workflows.
type TeamsWorkflowsNotifier struct {
	*Base
	URL     string
	Message string
	tmpl    *template.Template
	log     log.Logger
	ns      notifications.WebhookSender
}

type TeamsWorkflowsConfig struct {
	*NotificationChannelConfig
	URL     string
	Message string
}

func TeamsWorkflowsFactory(fc FactoryConfig) (NotificationChannel, error) {
	cfg, err := NewTeamsWorkflowsConfig(fc.Config)
	if err != nil {
		return nil, receiverInitError{
			Reason: err.Error(),
			Cfg:    *fc.Config,
		}
	}
	return NewTeamsWorkflowsNotifier(cfg, fc.NotificationService, fc.Template), nil
}

func NewTeamsWorkflowsConfig(config *NotificationChannelConfig) (*TeamsWorkflowsConfig, error) {
	URL := config.Settings.Get("url").MustString()
	if URL == "" {
		return nil, errors.New("could not find url property in settings")
	}
	return &TeamsWorkflowsConfig{
		NotificationChannelConfig: config,
		URL:                       URL,
		Message:                   config.Settings.Get("message").MustString(`{{ template "teams.default.message" .}}`),
	}, nil
}

// NewTeamsWorkflowsNotifier is the constructor for TeamsWorkflows notifier.
func NewTeamsWorkflowsNotifier(config *TeamsWorkflowsConfig, ns notifications.WebhookSender, t *template.Template) *TeamsWorkflowsNotifier {
	return &TeamsWorkflowsNotifier{
		Base: NewBase(&models.AlertNotification{
			Uid:                   config.UID,
			Name:                  config.Name,
			Type:                  config.Type,
			DisableResolveMessage: config.DisableResolveMessage,
			Settings:              config.Settings,
		}),
		URL:     config.URL,
		Message: config.Message,
		log:     log.New("alerting.notifier.teams"),
		ns:      ns,
		tmpl:    t,
	}
}

// Notify send an alert notification to Microsoft Teams.
func (tn *TeamsWorkflowsNotifier) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	var tmplErr error
	tmpl, _ := TmplText(ctx, tn.tmpl, as, tn.log, &tmplErr)

	basePath := ToBasePathWithAccountRedirect(tn.tmpl.ExternalURL, types.Alerts(as...)) //LOGZ.IO GRAFANA CHANGE :: DEV-37746: Add switch to account query param
	ruleURL := ToLogzioAppPath(joinUrlPath(basePath, "/alerting/list", tn.log))         // LOGZ.IO GRAFANA CHANGE :: DEV-31554 - Set APP url to logzio grafana for alert notification URLs

	title := tmpl(DefaultMessageTitleEmbed)

	body := map[string]interface{}{
		"type": "message",
		"attachments": []map[string]interface{}{
			{
				"contentType": "application/vnd.microsoft.card.adaptive",
				"content": map[string]interface{}{
					"$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
					"type":    "AdaptiveCard",
					"version": "1.2",
					"body": []map[string]interface{}{
						{
							"type":   "TextBlock",
							"text":   title,
							"weight": "bolder",
							"size":   "medium",
						},
						{
							"type":    "TextBlock",
							"text":    "Details",
							"weight":  "bolder",
							"spacing": "medium",
						},
						{
							"type": "TextBlock",
							"text": tmpl(tn.Message),
							"wrap": true,
						},
					},
					"actions": []map[string]interface{}{
						{
							"type":  "Action.OpenUrl",
							"title": "View Rule",
							"url":   ruleURL,
						},
					},
					"backgroundImage": map[string]interface{}{
						"color": getAlertStatusColor(types.Alerts(as...).Status()),
					},
				},
			},
		},
	}

	if tmplErr != nil {
		tn.log.Warn("failed to template Teams message", "err", tmplErr.Error())
		tmplErr = nil
	}

	u := tmpl(tn.URL)
	if tmplErr != nil {
		tn.log.Warn("failed to template Teams URL", "err", tmplErr.Error(), "fallback", tn.URL)
		u = tn.URL
	}

	b, err := json.Marshal(&body)
	if err != nil {
		return false, errors.Wrap(err, "marshal json")
	}
	cmd := &models.SendWebhookSync{Url: u, Body: string(b)}

	if err := tn.ns.SendWebhookSync(ctx, cmd); err != nil {
		return false, errors.Wrap(err, "send notification to Teams")
	}

	return true, nil
}

func (tn *TeamsWorkflowsNotifier) SendResolved() bool {
	return !tn.GetDisableResolveMessage()
}
