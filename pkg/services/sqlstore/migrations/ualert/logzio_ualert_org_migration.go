package ualert

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gofrs/uuid"
	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/infra/metrics"
	"github.com/grafana/grafana/pkg/models"
	ngmodels "github.com/grafana/grafana/pkg/services/ngalert/models"
	"github.com/grafana/grafana/pkg/services/ngalert/notifier/channels"
	"github.com/grafana/grafana/pkg/services/sqlstore/migrator"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/grafana/grafana/pkg/util"
	"github.com/matttproud/golang_protobuf_extensions/pbutil"
	pb "github.com/prometheus/alertmanager/silence/silencepb"
	"github.com/prometheus/common/model"
	"io"
	"strconv"
	"strings"
	"time"
	"xorm.io/xorm"
)

// LOGZ.IO GRAFANA CHANGE :: DEV-30705 - Add endpoint to migrate alerts of organization
const defaultContactPointReceiverName = "Default autogenerated contact point (placeholder)"

const getDashboardAlertsSql = `
SELECT id,
	org_id,
	dashboard_id,
	panel_id,
	org_id,
	name,
	message,
	frequency,
	%s,
	state,
	settings
FROM
	alert
WHERE org_id = ?
`

//MigrateOrgAlerts migrates old (pre Grafana 8 alerts) to new unified alerts with corresponding notification policies. Based on ualert.migration
type MigrateOrgAlerts struct {
	migrator.MigrationBase
	// session and mg are attached for convenience.
	sess *xorm.Session
	mg   *migrator.Migrator

	seenChannelUIDs   map[string]struct{}
	silences          map[int64][]*pb.MeshSilence
	orgId             int64
	channelUidByEmail map[string]string // email address -> Notification channel uuid
}

func NewOrgAlertMigration(orgId int64, channelUidByEmail map[string]string) *MigrateOrgAlerts {
	return &MigrateOrgAlerts{
		seenChannelUIDs:   make(map[string]struct{}),
		silences:          make(map[int64][]*pb.MeshSilence),
		orgId:             orgId,
		channelUidByEmail: channelUidByEmail,
	}
}

func (m *MigrateOrgAlerts) SQL(dialect migrator.Dialect) string {
	return "code migration"
}

//nolint: gocyclo
func (m *MigrateOrgAlerts) Exec(sess *xorm.Session, mg *migrator.Migrator) error {
	m.sess = sess
	m.mg = mg

	dashAlerts, err := m.slurpDashAlerts()
	if err != nil {
		return err
	}
	mg.Logger.Info("alerts found to migrate", "alerts", len(dashAlerts))

	// [orgID, dataSourceId] -> UID
	dsIDMap, err := m.slurpDSIDs()
	if err != nil {
		return err
	}

	// [orgID, dashboardId] -> dashUID
	dashIDMap, err := m.slurpDashUIDs()
	if err != nil {
		return err
	}

	// cache for folders created for dashboards that have custom permissions
	folderCache := make(map[string]*dashboard)

	// Store of newly created rules to later create routes
	rulesPerOrg := make(map[int64]map[string]dashAlert)

	for _, da := range dashAlerts {
		newCond, err := transConditions(*da.ParsedSettings, da.OrgId, dsIDMap)
		if err != nil {
			return err
		}

		da.DashboardUID = dashIDMap[[2]int64{da.OrgId, da.DashboardId}]

		// get dashboard
		dash := dashboard{}
		exists, err := m.sess.Where("org_id=? AND uid=?", da.OrgId, da.DashboardUID).Get(&dash)
		if err != nil {
			return MigrationError{
				Err:     fmt.Errorf("failed to get dashboard %s under organisation %d: %w", da.DashboardUID, da.OrgId, err),
				AlertId: da.Id,
			}
		}
		if !exists {
			return MigrationError{
				Err:     fmt.Errorf("dashboard with UID %v under organisation %d not found: %w", da.DashboardUID, da.OrgId, err),
				AlertId: da.Id,
			}
		}

		var folder *dashboard
		switch {
		case dash.HasAcl:
			folderName := getAlertFolderNameFromDashboard(&dash)
			f, ok := folderCache[folderName]
			if !ok {
				mg.Logger.Info("create a new folder for alerts that belongs to dashboard because it has custom permissions", "org", dash.OrgId, "dashboard_uid", dash.Uid, "folder", folderName)
				// create folder and assign the permissions of the dashboard (included default and inherited)
				f, err = m.createFolder(dash.OrgId, folderName)
				if err != nil {
					return MigrationError{
						Err:     fmt.Errorf("failed to create folder: %w", err),
						AlertId: da.Id,
					}
				}
				permissions, err := m.getACL(dash.OrgId, dash.Id)
				if err != nil {
					return MigrationError{
						Err:     fmt.Errorf("failed to get dashboard %d under organisation %d permissions: %w", dash.Id, dash.OrgId, err),
						AlertId: da.Id,
					}
				}
				err = m.setACL(f.OrgId, f.Id, permissions)
				if err != nil {
					return MigrationError{
						Err:     fmt.Errorf("failed to set folder %d under organisation %d permissions: %w", folder.Id, folder.OrgId, err),
						AlertId: da.Id,
					}
				}
				folderCache[folderName] = f
			}
			folder = f
		case dash.FolderId > 0:
			// get folder if exists
			f, err := m.getFolder(dash)
			if err != nil {
				return MigrationError{
					Err:     err,
					AlertId: da.Id,
				}
			}
			folder = &f
		default:
			f, ok := folderCache[GENERAL_FOLDER]
			if !ok {
				// get or create general folder
				f, err = m.getOrCreateGeneralFolder(dash.OrgId)
				if err != nil {
					return MigrationError{
						Err:     fmt.Errorf("failed to get or create general folder under organisation %d: %w", dash.OrgId, err),
						AlertId: da.Id,
					}
				}
				folderCache[GENERAL_FOLDER] = f
			}
			// No need to assign default permissions to general folder
			// because they are included to the query result if it's a folder with no permissions
			// https://github.com/grafana/grafana/blob/076e2ce06a6ecf15804423fcc8dca1b620a321e5/pkg/services/sqlstore/dashboard_acl.go#L109
			folder = f
		}

		if folder.Uid == "" {
			return MigrationError{
				Err:     fmt.Errorf("empty folder identifier"),
				AlertId: da.Id,
			}
		}
		rule, err := m.makeAlertRule(*newCond, da, folder.Uid)
		if err != nil {
			return err
		}

		if _, ok := rulesPerOrg[rule.OrgID]; !ok {
			rulesPerOrg[rule.OrgID] = make(map[string]dashAlert)
		}
		if _, ok := rulesPerOrg[rule.OrgID][rule.UID]; !ok {
			rulesPerOrg[rule.OrgID][rule.UID] = da
		} else {
			return MigrationError{
				Err:     fmt.Errorf("duplicate generated rule UID"),
				AlertId: da.Id,
			}
		}

		if strings.HasPrefix(mg.Dialect.DriverName(), migrator.Postgres) {
			err = mg.InTransaction(func(sess *xorm.Session) error {
				_, err = sess.Insert(rule)
				return err
			})
		} else {
			_, err = m.sess.Insert(rule)
		}
		if err != nil {
			rule.Title += fmt.Sprintf(" %v", rule.UID)
			rule.RuleGroup += fmt.Sprintf(" %v", rule.UID)

			_, err = m.sess.Insert(rule)
			if err != nil {
				return err
			}
		}

		// create entry in alert_rule_version
		_, err = m.sess.Insert(rule.makeVersion())
		if err != nil {
			return err
		}
	}

	for orgID := range rulesPerOrg {
		if err := m.writeSilencesFile(orgID); err != nil {
			m.mg.Logger.Error("alert migration error: failed to write silence file", "err", err)
		}
	}

	amConfigPerOrg, err := m.setupAlertmanagerConfigs(rulesPerOrg)
	if err != nil {
		return err
	}
	for orgID, amConfig := range amConfigPerOrg {
		if err := m.writeAlertmanagerConfig(orgID, amConfig); err != nil {
			return err
		}
	}

	return nil
}

// slurpDashAlerts loads all alerts from the alert database table into the
// the dashAlert type.
// Additionally it unmarshals the json settings for the alert into the
// ParsedSettings property of the dash alert.
func (m *MigrateOrgAlerts) slurpDashAlerts() ([]dashAlert, error) {
	var dashAlerts []dashAlert

	err := m.sess.SQL(fmt.Sprintf(getDashboardAlertsSql, m.mg.Dialect.Quote("for")), m.orgId).Find(&dashAlerts)

	if err != nil {
		return nil, err
	}

	for i := range dashAlerts {
		err = json.Unmarshal(dashAlerts[i].Settings, &dashAlerts[i].ParsedSettings)
		if err != nil {
			return nil, err
		}
	}

	return dashAlerts, nil
}

// slurpDSIDs returns a map of [orgID, dataSourceId] -> UID.
func (m *MigrateOrgAlerts) slurpDSIDs() (dsUIDLookup, error) {
	var dsIDs []struct {
		OrgID int64  `xorm:"org_id"`
		ID    int64  `xorm:"id"`
		UID   string `xorm:"uid"`
	}

	err := m.sess.SQL(`SELECT org_id, id, uid FROM data_source WHERE org_id = ?`, m.orgId).Find(&dsIDs)

	if err != nil {
		return nil, err
	}

	idToUID := make(dsUIDLookup, len(dsIDs))

	for _, ds := range dsIDs {
		idToUID[[2]int64{ds.OrgID, ds.ID}] = ds.UID
	}

	return idToUID, nil
}

// slurpDashUIDs returns a map of [orgID, dashboardId] -> dashUID.
func (m *MigrateOrgAlerts) slurpDashUIDs() (map[[2]int64]string, error) {
	var dashIDs []struct {
		OrgID int64  `xorm:"org_id"`
		ID    int64  `xorm:"id"`
		UID   string `xorm:"uid"`
	}

	err := m.sess.SQL(`SELECT org_id, id, uid FROM dashboard WHERE org_id = ?`, m.orgId).Find(&dashIDs)

	if err != nil {
		return nil, err
	}

	idToUID := make(map[[2]int64]string, len(dashIDs))

	for _, ds := range dashIDs {
		idToUID[[2]int64{ds.OrgID, ds.ID}] = ds.UID
	}

	return idToUID, nil
}

// setupAlertmanagerConfigs creates Alertmanager configs with migrated receivers and routes.
func (m *MigrateOrgAlerts) setupAlertmanagerConfigs(rulesPerOrg map[int64]map[string]dashAlert) (amConfigsPerOrg, error) {
	// allChannels: channelUID -> channelConfig
	allChannelsPerOrg, defaultChannelsPerOrg, err := m.getNotificationChannelMap()
	if err != nil {
		return nil, fmt.Errorf("failed to load notification channels: %w", err)
	}

	amConfigPerOrg := make(amConfigsPerOrg, len(allChannelsPerOrg))
	for orgID, orgChannels := range allChannelsPerOrg {
		amConfig := &PostableUserConfig{
			AlertmanagerConfig: PostableApiAlertingConfig{
				Receivers: make([]*PostableApiReceiver, 0),
			},
		}
		amConfigPerOrg[orgID] = amConfig

		// Create all newly migrated receivers from legacy notification channels.
		receiversMap, receivers, err := m.createReceivers(orgChannels)
		if err != nil {
			return nil, fmt.Errorf("failed to create receiver in orgId %d: %w", orgID, err)
		}

		// No need to create an Alertmanager configuration if there are no receivers left that aren't obsolete.
		if len(receivers) == 0 {
			m.mg.Logger.Warn("no available receivers", "orgId", orgID)
			continue
		}

		amConfig.AlertmanagerConfig.Receivers = receivers

		defaultReceivers := make(map[string]struct{})
		defaultChannels, ok := defaultChannelsPerOrg[orgID]
		if ok {
			// If the organization has default channels build a map of default receivers, used to create alert-specific routes later.
			for _, c := range defaultChannels {
				defaultReceivers[c.Name] = struct{}{}
			}
		}
		defaultReceiver, defaultRoute, err := m.createDefaultRouteAndReceiver(defaultChannels)
		if err != nil {
			return nil, fmt.Errorf("failed to create default route & receiver in orgId %d: %w", orgID, err)
		}
		amConfig.AlertmanagerConfig.Route = defaultRoute
		if defaultReceiver != nil {
			amConfig.AlertmanagerConfig.Receivers = append(amConfig.AlertmanagerConfig.Receivers, defaultReceiver)
		}

		// Create routes
		if rules, ok := rulesPerOrg[orgID]; ok {
			for ruleUid, da := range rules {
				route, err := m.createRouteForAlert(ruleUid, da, receiversMap, defaultReceivers)
				if err != nil {
					return nil, fmt.Errorf("failed to create route for alert %s in orgId %d: %w", da.Name, orgID, err)
				}

				if route != nil {
					amConfigPerOrg[da.OrgId].AlertmanagerConfig.Route.Routes = append(amConfigPerOrg[da.OrgId].AlertmanagerConfig.Route.Routes, route)
				}
			}
		}

		// Validate the alertmanager configuration produced, this gives a chance to catch bad configuration at migration time.
		// Validation between legacy and unified alerting can be different (e.g. due to bug fixes) so this would fail the migration in that case.
		if err := m.validateAlertmanagerConfig(orgID, amConfig); err != nil {
			return nil, fmt.Errorf("failed to validate AlertmanagerConfig in orgId %d: %w", orgID, err)
		}
	}

	return amConfigPerOrg, nil
}

func (m *MigrateOrgAlerts) getNotificationChannelMap() (channelsPerOrg, defaultChannelsPerOrg, error) {
	q := `
	SELECT id,
		org_id,
		uid,
		name,
		type,
		disable_resolve_message,
		is_default,
		settings,
		secure_settings
	FROM
		alert_notification
	WHERE
		org_id = ?
	`
	var allChannels []notificationChannel
	err := m.sess.SQL(q, m.orgId).Find(&allChannels)
	if err != nil {
		return nil, nil, err
	}

	if len(allChannels) == 0 {
		return nil, nil, nil
	}

	allChannelsMap := make(channelsPerOrg)
	defaultChannelsMap := make(defaultChannelsPerOrg)
	for i, c := range allChannels {
		if c.Type == "hipchat" || c.Type == "sensu" {
			m.mg.Logger.Error("alert migration error: discontinued notification channel found", "type", c.Type, "name", c.Name, "uid", c.Uid)
			continue
		}

		allChannelsMap[c.OrgID] = append(allChannelsMap[c.OrgID], &allChannels[i])

		if c.IsDefault {
			defaultChannelsMap[c.OrgID] = append(defaultChannelsMap[c.OrgID], &allChannels[i])
		}
	}

	return allChannelsMap, defaultChannelsMap, nil
}

// Create one receiver for every unique notification channel.
func (m *MigrateOrgAlerts) createReceivers(allChannels []*notificationChannel) (map[uidOrID]*PostableApiReceiver, []*PostableApiReceiver, error) {
	var receivers []*PostableApiReceiver
	receiversMap := make(map[uidOrID]*PostableApiReceiver)
	for _, c := range allChannels {
		notifier, err := m.createNotifier(c)
		if err != nil {
			return nil, nil, err
		}

		recv := &PostableApiReceiver{
			Name:                    c.Name, // Channel name is unique within an Org.
			GrafanaManagedReceivers: []*PostableGrafanaReceiver{notifier},
		}

		receivers = append(receivers, recv)

		// Store receivers for creating routes from alert rules later.
		if c.Uid != "" {
			receiversMap[c.Uid] = recv
		}
		if c.ID != 0 {
			// In certain circumstances, the alert rule uses ID instead of uid. So, we add this to be able to lookup by ID in case.
			receiversMap[c.ID] = recv
		}
	}

	return receiversMap, receivers, nil
}

// Create a notifier (PostableGrafanaReceiver) from a legacy notification channel
func (m *MigrateOrgAlerts) createNotifier(c *notificationChannel) (*PostableGrafanaReceiver, error) {
	uid, err := m.generateChannelUID()
	if err != nil {
		return nil, err
	}

	settings, secureSettings, err := migrateSettingsToSecureSettings(c.Type, c.Settings, c.SecureSettings)
	if err != nil {
		return nil, err
	}

	return &PostableGrafanaReceiver{
		UID:                   uid,
		Name:                  c.Name,
		Type:                  c.Type,
		DisableResolveMessage: c.DisableResolveMessage,
		Settings:              settings,
		SecureSettings:        secureSettings,
	}, nil
}

// Create the root-level route with the default receiver. If no new receiver is created specifically for the root-level route, the returned receiver will be nil.
func (m *MigrateOrgAlerts) createDefaultRouteAndReceiver(defaultChannels []*notificationChannel) (*PostableApiReceiver, *Route, error) {
	var defaultReceiver *PostableApiReceiver

	defaultReceiverName := defaultContactPointReceiverName
	if len(defaultChannels) != 1 {
		// If there are zero or more than one default channels we create a separate contact group that is used only in the root policy. This is to simplify the migrated notification policy structure.
		// If we ever allow more than one receiver per route this won't be necessary.
		defaultReceiver = &PostableApiReceiver{
			Name:                    defaultReceiverName,
			GrafanaManagedReceivers: []*PostableGrafanaReceiver{},
		}

		for _, c := range defaultChannels {
			// Need to create a new notifier to prevent uid conflict.
			defaultNotifier, err := m.createNotifier(c)
			if err != nil {
				return nil, nil, err
			}

			defaultReceiver.GrafanaManagedReceivers = append(defaultReceiver.GrafanaManagedReceivers, defaultNotifier)
		}
	} else {
		// If there is only a single default channel, we don't need a separate receiver to hold it. We can reuse the existing receiver for that single notifier.
		defaultReceiverName = defaultChannels[0].Name
	}

	defaultRoute := &Route{
		Receiver: defaultReceiverName,
		Routes:   make([]*Route, 0),
	}

	return defaultReceiver, defaultRoute, nil
}

// Wrapper to select receivers for given alert rules based on associated notification channels and then create the migrated route.
func (m *MigrateOrgAlerts) createRouteForAlert(ruleUID string, da dashAlert, receivers map[uidOrID]*PostableApiReceiver, defaultReceivers map[string]struct{}) (*Route, error) {
	// Create route(s) for alert
	filteredReceiverNames := m.filterReceiversForAlert(da, receivers, defaultReceivers)

	if len(filteredReceiverNames) != 0 {
		// Only create a route if there are specific receivers, otherwise it defaults to the root-level route.
		route, err := createRoute(ruleUID, filteredReceiverNames)
		if err != nil {
			return nil, err
		}

		return route, nil
	}

	return nil, nil
}

// Filter receivers to select those that were associated to the given rule as channels.
func (m *MigrateOrgAlerts) filterReceiversForAlert(da dashAlert, receivers map[uidOrID]*PostableApiReceiver, defaultReceivers map[string]struct{}) map[string]interface{} {
	channelIDs := extractChannelIDs(da)
	emailNotificationChannelIDs := extractEmailChannelIDs(da, m.channelUidByEmail)
	channelIDs = append(channelIDs, emailNotificationChannelIDs...)

	if len(channelIDs) == 0 {
		// If there are no channels associated, we use the default route.
		return nil
	}

	// Filter receiver names.
	filteredReceiverNames := make(map[string]interface{})
	for _, uidOrId := range channelIDs {
		recv, ok := receivers[uidOrId]
		if ok {
			filteredReceiverNames[recv.Name] = struct{}{} // Deduplicate on contact point name.
		} else {
			m.mg.Logger.Warn("alert linked to obsolete notification channel, ignoring", "alert", da.Name, "uid", uidOrId)
		}
	}

	coveredByDefault := func(names map[string]interface{}) bool {
		// Check if all receivers are also default ones and if so, just use the default route.
		for n := range names {
			if _, ok := defaultReceivers[n]; !ok {
				return false
			}
		}
		return true
	}

	if len(filteredReceiverNames) == 0 || coveredByDefault(filteredReceiverNames) {
		// Use the default route instead.
		return nil
	}

	// Add default receivers alongside rule-specific ones.
	for n := range defaultReceivers {
		filteredReceiverNames[n] = struct{}{}
	}

	return filteredReceiverNames
}

func extractEmailChannelIDs(d dashAlert, channelUidByEmailAddress map[string]string) (channelUids []uidOrID) {
	// Extracting channel UID/ID.
	for _, emailNot := range d.ParsedSettings.EmailNotifications {
		if emailNot.Address != "" {
			if channelUid, found := channelUidByEmailAddress[emailNot.Address]; found {
				channelUids = append(channelUids, channelUid)
			}
		}
	}

	return channelUids
}

func (m *MigrateOrgAlerts) generateChannelUID() (string, error) {
	for i := 0; i < 5; i++ {
		gen := util.GenerateShortUID()
		if _, ok := m.seenChannelUIDs[gen]; !ok {
			m.seenChannelUIDs[gen] = struct{}{}
			return gen, nil
		}
	}

	return "", errors.New("failed to generate UID for notification channel")
}

// returns the folder of the given dashboard (if exists)
func (m *MigrateOrgAlerts) getFolder(dash dashboard) (dashboard, error) {
	// get folder if exists
	folder := dashboard{}
	if dash.FolderId > 0 {
		exists, err := m.sess.Where("id=?", dash.FolderId).Get(&folder)
		if err != nil {
			return folder, fmt.Errorf("failed to get folder %d: %w", dash.FolderId, err)
		}
		if !exists {
			return folder, fmt.Errorf("folder with id %v not found", dash.FolderId)
		}
		if !folder.IsFolder {
			return folder, fmt.Errorf("id %v is a dashboard not a folder", dash.FolderId)
		}
	}
	return folder, nil
}

// based on sqlstore.saveDashboard()
// it should be called from inside a transaction
func (m *MigrateOrgAlerts) createFolder(orgID int64, title string) (*dashboard, error) {
	cmd := saveFolderCommand{
		OrgId:    orgID,
		FolderId: 0,
		IsFolder: true,
		Dashboard: simplejson.NewFromAny(map[string]interface{}{
			"title": title,
		}),
	}
	dash := cmd.getDashboardModel()

	uid, err := m.generateNewDashboardUid(dash.OrgId)
	if err != nil {
		return nil, err
	}
	dash.setUid(uid)

	parentVersion := dash.Version
	dash.setVersion(1)
	dash.Created = time.Now()
	dash.CreatedBy = FOLDER_CREATED_BY
	dash.Updated = time.Now()
	dash.UpdatedBy = FOLDER_CREATED_BY
	metrics.MApiDashboardInsert.Inc()

	if _, err = m.sess.Insert(dash); err != nil {
		return nil, err
	}

	dashVersion := &models.DashboardVersion{
		DashboardId:   dash.Id,
		ParentVersion: parentVersion,
		RestoredFrom:  cmd.RestoredFrom,
		Version:       dash.Version,
		Created:       time.Now(),
		CreatedBy:     dash.UpdatedBy,
		Message:       cmd.Message,
		Data:          dash.Data,
	}

	// insert version entry
	if _, err := m.sess.Insert(dashVersion); err != nil {
		return nil, err
	}
	return dash, nil
}

// based on SQLStore.GetDashboardAclInfoList()
func (m *MigrateOrgAlerts) getACL(orgID, dashboardID int64) ([]*dashboardAcl, error) {
	var err error

	falseStr := m.mg.Dialect.BooleanStr(false)

	result := make([]*dashboardAcl, 0)
	rawSQL := `
			-- get distinct permissions for the dashboard and its parent folder
			SELECT DISTINCT
				da.id,
				da.user_id,
				da.team_id,
				da.permission,
				da.role
			FROM dashboard as d
				LEFT JOIN dashboard folder on folder.id = d.folder_id
				LEFT JOIN dashboard_acl AS da ON
				da.dashboard_id = d.id OR
				da.dashboard_id = d.folder_id  OR
				(
					-- include default permissions --
					da.org_id = -1 AND (
					  (folder.id IS NOT NULL AND folder.has_acl = ` + falseStr + `) OR
					  (folder.id IS NULL AND d.has_acl = ` + falseStr + `)
					)
				)
			WHERE d.org_id = ? AND d.id = ? AND da.id IS NOT NULL
			ORDER BY da.id ASC
			`
	err = m.sess.SQL(rawSQL, orgID, dashboardID).Find(&result)
	return result, err
}

// based on SQLStore.UpdateDashboardACL()
// it should be called from inside a transaction
func (m *MigrateOrgAlerts) setACL(orgID int64, dashboardID int64, items []*dashboardAcl) error {
	if dashboardID <= 0 {
		return fmt.Errorf("folder id must be greater than zero for a folder permission")
	}

	// userPermissionsMap is a map keeping the highest permission per user
	// for handling conficting inherited (folder) and non-inherited (dashboard) user permissions
	userPermissionsMap := make(map[int64]*dashboardAcl, len(items))
	// teamPermissionsMap is a map keeping the highest permission per team
	// for handling conficting inherited (folder) and non-inherited (dashboard) team permissions
	teamPermissionsMap := make(map[int64]*dashboardAcl, len(items))
	for _, item := range items {
		if item.UserID != 0 {
			acl, ok := userPermissionsMap[item.UserID]
			if !ok {
				userPermissionsMap[item.UserID] = item
			} else {
				if item.Permission > acl.Permission {
					// the higher permission wins
					userPermissionsMap[item.UserID] = item
				}
			}
		}

		if item.TeamID != 0 {
			acl, ok := teamPermissionsMap[item.TeamID]
			if !ok {
				teamPermissionsMap[item.TeamID] = item
			} else {
				if item.Permission > acl.Permission {
					// the higher permission wins
					teamPermissionsMap[item.TeamID] = item
				}
			}
		}
	}

	type keyType struct {
		UserID     int64 `xorm:"user_id"`
		TeamID     int64 `xorm:"team_id"`
		Role       roleType
		Permission permissionType
	}
	// seen keeps track of inserted perrmissions to avoid duplicates (due to inheritance)
	seen := make(map[keyType]struct{}, len(items))
	for _, item := range items {
		if item.UserID == 0 && item.TeamID == 0 && (item.Role == nil || !item.Role.IsValid()) {
			return models.ErrDashboardAclInfoMissing
		}

		// ignore duplicate user permissions
		if item.UserID != 0 {
			acl, ok := userPermissionsMap[item.UserID]
			if ok {
				if acl.Id != item.Id {
					continue
				}
			}
		}

		// ignore duplicate team permissions
		if item.TeamID != 0 {
			acl, ok := teamPermissionsMap[item.TeamID]
			if ok {
				if acl.Id != item.Id {
					continue
				}
			}
		}

		key := keyType{UserID: item.UserID, TeamID: item.TeamID, Role: "", Permission: item.Permission}
		if item.Role != nil {
			key.Role = *item.Role
		}
		if _, ok := seen[key]; ok {
			continue
		}

		// unset Id so that the new record will get a different one
		item.Id = 0
		item.OrgID = orgID
		item.DashboardID = dashboardID
		item.Created = time.Now()
		item.Updated = time.Now()

		m.sess.Nullable("user_id", "team_id")
		if _, err := m.sess.Insert(item); err != nil {
			return err
		}
		seen[key] = struct{}{}
	}

	// Update dashboard HasAcl flag
	dashboard := models.Dashboard{HasAcl: true}
	_, err := m.sess.Cols("has_acl").Where("id=?", dashboardID).Update(&dashboard)
	return err
}

// getOrCreateGeneralFolder returns the general folder under the specific organisation
// If the general folder does not exist it creates it.
func (m *MigrateOrgAlerts) getOrCreateGeneralFolder(orgID int64) (*dashboard, error) {
	// there is a unique constraint on org_id, folder_id, title
	// there are no nested folders so the parent folder id is always 0
	dashboard := dashboard{OrgId: orgID, FolderId: 0, Title: GENERAL_FOLDER}
	has, err := m.sess.Get(&dashboard)
	if err != nil {
		return nil, err
	} else if !has {
		// create folder
		result, err := m.createFolder(orgID, GENERAL_FOLDER)
		if err != nil {
			return nil, err
		}

		return result, nil
	}
	return &dashboard, nil
}

func (m *MigrateOrgAlerts) makeAlertRule(cond condition, da dashAlert, folderUID string) (*alertRule, error) {
	lbls, annotations := addMigrationInfo(&da)
	lbls["alertname"] = da.Name
	annotations["message"] = da.Message
	var err error

	data, err := migrateAlertRuleQueries(cond.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to migrate alert rule queries: %w", err)
	}

	ar := &alertRule{
		OrgID:           da.OrgId,
		Title:           da.Name,
		UID:             util.GenerateShortUID(),
		Condition:       cond.Condition,
		Data:            data,
		IntervalSeconds: ruleAdjustInterval(da.Frequency),
		Version:         1,
		NamespaceUID:    folderUID, // Folder already created, comes from env var.
		RuleGroup:       da.Name,
		For:             duration(da.For),
		Updated:         time.Now().UTC(),
		Annotations:     annotations,
		Labels:          lbls,
	}

	ar.NoDataState, err = transNoData(da.ParsedSettings.NoDataState)
	if err != nil {
		return nil, err
	}

	ar.ExecErrState, err = transExecErr(da.ParsedSettings.ExecutionErrorState)
	if err != nil {
		return nil, err
	}

	// Label for routing and silences.
	n, v := getLabelForRouteMatching(ar.UID)
	ar.Labels[n] = v

	if err := m.addSilence(da, ar); err != nil {
		m.mg.Logger.Error("alert migration error: failed to create silence", "rule_name", ar.Title, "err", err)
	}

	if err := m.addErrorSilence(da, ar); err != nil {
		m.mg.Logger.Error("alert migration error: failed to create silence for Error", "rule_name", ar.Title, "err", err)
	}

	if err := m.addNoDataSilence(da, ar); err != nil {
		m.mg.Logger.Error("alert migration error: failed to create silence for NoData", "rule_name", ar.Title, "err", err)
	}

	return ar, nil
}

func (m *MigrateOrgAlerts) addSilence(da dashAlert, rule *alertRule) error {
	if da.State != "paused" {
		return nil
	}

	uid, err := uuid.NewV4()
	if err != nil {
		return errors.New("failed to create uuid for silence")
	}

	n, v := getLabelForRouteMatching(rule.UID)
	s := &pb.MeshSilence{
		Silence: &pb.Silence{
			Id: uid.String(),
			Matchers: []*pb.Matcher{
				{
					Type:    pb.Matcher_EQUAL,
					Name:    n,
					Pattern: v,
				},
			},
			StartsAt:  time.Now(),
			EndsAt:    time.Now().Add(365 * 20 * time.Hour), // 1 year.
			CreatedBy: "Grafana Migration",
			Comment:   "Created during auto migration to unified alerting",
		},
		ExpiresAt: time.Now().Add(365 * 20 * time.Hour), // 1 year.
	}

	_, ok := m.silences[da.OrgId]
	if !ok {
		m.silences[da.OrgId] = make([]*pb.MeshSilence, 0)
	}
	m.silences[da.OrgId] = append(m.silences[da.OrgId], s)
	return nil
}

func (m *MigrateOrgAlerts) addErrorSilence(da dashAlert, rule *alertRule) error {
	if da.ParsedSettings.ExecutionErrorState != "keep_state" {
		return nil
	}

	uid, err := uuid.NewV4()
	if err != nil {
		return errors.New("failed to create uuid for silence")
	}

	s := &pb.MeshSilence{
		Silence: &pb.Silence{
			Id: uid.String(),
			Matchers: []*pb.Matcher{
				{
					Type:    pb.Matcher_EQUAL,
					Name:    model.AlertNameLabel,
					Pattern: ErrorAlertName,
				},
				{
					Type:    pb.Matcher_EQUAL,
					Name:    "rule_uid",
					Pattern: rule.UID,
				},
			},
			StartsAt:  time.Now(),
			EndsAt:    time.Now().AddDate(1, 0, 0), // 1 year
			CreatedBy: "Grafana Migration",
			Comment:   fmt.Sprintf("Created during migration to unified alerting to silence Error state for alert rule ID '%s' and Title '%s' because the option 'Keep Last State' was selected for Error state", rule.UID, rule.Title),
		},
		ExpiresAt: time.Now().AddDate(1, 0, 0), // 1 year
	}
	if _, ok := m.silences[da.OrgId]; !ok {
		m.silences[da.OrgId] = make([]*pb.MeshSilence, 0)
	}
	m.silences[da.OrgId] = append(m.silences[da.OrgId], s)
	return nil
}

func (m *MigrateOrgAlerts) addNoDataSilence(da dashAlert, rule *alertRule) error {
	if da.ParsedSettings.NoDataState != "keep_state" {
		return nil
	}

	uid, err := uuid.NewV4()
	if err != nil {
		return errors.New("failed to create uuid for silence")
	}

	s := &pb.MeshSilence{
		Silence: &pb.Silence{
			Id: uid.String(),
			Matchers: []*pb.Matcher{
				{
					Type:    pb.Matcher_EQUAL,
					Name:    model.AlertNameLabel,
					Pattern: NoDataAlertName,
				},
				{
					Type:    pb.Matcher_EQUAL,
					Name:    "rule_uid",
					Pattern: rule.UID,
				},
			},
			StartsAt:  time.Now(),
			EndsAt:    time.Now().AddDate(1, 0, 0), // 1 year.
			CreatedBy: "Grafana Migration",
			Comment:   fmt.Sprintf("Created during migration to unified alerting to silence NoData state for alert rule ID '%s' and Title '%s' because the option 'Keep Last State' was selected for NoData state", rule.UID, rule.Title),
		},
		ExpiresAt: time.Now().AddDate(1, 0, 0), // 1 year.
	}
	_, ok := m.silences[da.OrgId]
	if !ok {
		m.silences[da.OrgId] = make([]*pb.MeshSilence, 0)
	}
	m.silences[da.OrgId] = append(m.silences[da.OrgId], s)
	return nil
}

func (m *MigrateOrgAlerts) generateNewDashboardUid(orgId int64) (string, error) {
	for i := 0; i < 3; i++ {
		uid := util.GenerateShortUID()

		exists, err := m.sess.Where("org_id=? AND uid=?", orgId, uid).Get(&models.Dashboard{})
		if err != nil {
			return "", err
		}

		if !exists {
			return uid, nil
		}
	}

	return "", models.ErrDashboardFailedGenerateUniqueUid
}

// validateAlertmanagerConfig validates the alertmanager configuration produced by the migration against the receivers.
func (m *MigrateOrgAlerts) validateAlertmanagerConfig(orgID int64, config *PostableUserConfig) error {
	for _, r := range config.AlertmanagerConfig.Receivers {
		for _, gr := range r.GrafanaManagedReceivers {
			// First, let's decode the secure settings - given they're stored as base64.
			secureSettings := make(map[string][]byte, len(gr.SecureSettings))
			for k, v := range gr.SecureSettings {
				d, err := base64.StdEncoding.DecodeString(v)
				if err != nil {
					return err
				}
				secureSettings[k] = d
			}

			var (
				cfg = &channels.NotificationChannelConfig{
					UID:                   gr.UID,
					OrgID:                 orgID,
					Name:                  gr.Name,
					Type:                  gr.Type,
					DisableResolveMessage: gr.DisableResolveMessage,
					Settings:              gr.Settings,
					SecureSettings:        secureSettings,
				}
				err error
			)

			// decryptFunc represents the legacy way of decrypting data. Before the migration, we don't need any new way,
			// given that the previous alerting will never support it.
			decryptFunc := func(_ context.Context, sjd map[string][]byte, key string, fallback string) string {
				if value, ok := sjd[key]; ok {
					decryptedData, err := util.Decrypt(value, setting.SecretKey)
					if err != nil {
						m.mg.Logger.Warn("unable to decrypt key '%s' for %s receiver with uid %s, returning fallback.", key, gr.Type, gr.UID)
						return fallback
					}
					return string(decryptedData)
				}
				return fallback
			}
			receiverFactory, exists := channels.Factory(gr.Type)
			if !exists {
				return fmt.Errorf("notifier %s is not supported", gr.Type)
			}
			factoryConfig, err := channels.NewFactoryConfig(cfg, nil, decryptFunc, nil)
			if err != nil {
				return err
			}
			_, err = receiverFactory(factoryConfig)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (m *MigrateOrgAlerts) writeAlertmanagerConfig(orgID int64, amConfig *PostableUserConfig) error {
	rawAmConfig, err := json.Marshal(amConfig)
	if err != nil {
		return err
	}

	// We don't need to apply the configuration, given the multi org alertmanager will do an initial sync before the server is ready.
	_, err = m.sess.Insert(AlertConfiguration{
		AlertmanagerConfiguration: string(rawAmConfig),
		// Since we are migration for a snapshot of the code, it is always going to migrate to
		// the v1 config.
		ConfigurationVersion: "v1",
		OrgID:                orgID,
	})
	if err != nil {
		return err
	}

	return nil
}

func (m *MigrateOrgAlerts) writeSilencesFile(orgID int64) error {
	var buf bytes.Buffer
	orgSilences, ok := m.silences[orgID]
	if !ok {
		return nil
	}

	for _, e := range orgSilences {
		if _, err := pbutil.WriteDelimited(&buf, e); err != nil {
			return err
		}
	}

	f, err := openReplace(silencesFileNameForOrg(m.mg, orgID))
	if err != nil {
		return err
	}

	if _, err := io.Copy(f, bytes.NewReader(buf.Bytes())); err != nil {
		return err
	}

	return f.Close()
}

// UpdateOrgDashboardUIDPanelIDMigration sets the dashboard_uid and panel_id columns
// from the __dashboardUid__ and __panelId__ annotations. Based on ualert.updateDashboardUIDPanelIDMigration
type UpdateOrgDashboardUIDPanelIDMigration struct {
	migrator.MigrationBase
	OrgId int64
}

func (m *UpdateOrgDashboardUIDPanelIDMigration) SQL(_ migrator.Dialect) string {
	return "set dashboard_uid and panel_id migration"
}

func (m *UpdateOrgDashboardUIDPanelIDMigration) Exec(sess *xorm.Session, mg *migrator.Migrator) error {
	var results []struct {
		ID          int64             `xorm:"id"`
		Annotations map[string]string `xorm:"annotations"`
	}
	if err := sess.SQL(`SELECT id, annotations FROM alert_rule WHERE org_id = ?`, m.OrgId).Find(&results); err != nil {
		return fmt.Errorf("failed to get annotations for all alert rules: %w", err)
	}
	for _, next := range results {
		var (
			dashboardUID *string
			panelID      *int64
		)
		if s, ok := next.Annotations[ngmodels.DashboardUIDAnnotation]; ok {
			dashboardUID = &s
		}
		if s, ok := next.Annotations[ngmodels.PanelIDAnnotation]; ok {
			i, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				return fmt.Errorf("the %s annotation does not contain a valid Panel ID: %w", ngmodels.PanelIDAnnotation, err)
			}
			panelID = &i
		}
		// We do not want to set panel_id to a non-nil value when dashboard_uid is nil
		// as panel_id is not unique and so cannot be queried without its dashboard_uid.
		// This can happen where users have deleted the dashboard_uid annotation but kept
		// the panel_id annotation.
		if dashboardUID != nil {
			if _, err := sess.Exec(`UPDATE alert_rule SET dashboard_uid = ?, panel_id = ? WHERE id = ?`,
				dashboardUID,
				panelID,
				next.ID); err != nil {
				return fmt.Errorf("failed to set dashboard_uid and panel_id for alert rule: %w", err)
			}
		}
	}
	return nil
}

// LOGZ.IO GRAFANA CHANGE :: end
