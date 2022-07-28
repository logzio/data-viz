import { merge } from 'lodash';
import {
  BuildInfo,
  createTheme,
  DataSourceInstanceSettings,
  FeatureToggles,
  logzioConfigs, // LOGZ.IO GRAFANA CHANGE :: DEV-20247 Use logzio provider
  GrafanaConfig,
  GrafanaTheme,
  GrafanaTheme2,
  LicenseInfo,
  PanelPluginMeta,
  systemDateFormats,
  SystemDateFormatSettings,
} from '@grafana/data';

import { changeDatasourceLogos } from './changeDatasourceLogos.logzio'; // LOGZ.IO GRAFANA CHANGE :: DEV-19985: add datasource logos

export interface AzureSettings {
  cloud?: string;
  managedIdentityEnabled: boolean;
}

export class GrafanaBootConfig implements GrafanaConfig {
  datasources: { [str: string]: DataSourceInstanceSettings } = {};
  panels: { [key: string]: PanelPluginMeta } = {};
  minRefreshInterval = '';
  appUrl = '';
  appSubUrl = '';
  windowTitlePrefix = '';
  buildInfo: BuildInfo = {} as BuildInfo;
  newPanelTitle = '';
  bootData: any;
  externalUserMngLinkUrl = '';
  externalUserMngLinkName = '';
  externalUserMngInfo = '';
  allowOrgCreate = false;
  disableLoginForm = false;
  defaultDatasource = '';
  alertingEnabled = false;
  alertingErrorOrTimeout = '';
  alertingNoDataOrNullValues = '';
  alertingMinInterval = 1;
  authProxyEnabled = false;
  exploreEnabled = false;
  ldapEnabled = false;
  sigV4AuthEnabled = false;
  samlEnabled = false;
  autoAssignOrg = true;
  verifyEmailEnabled = false;
  oauth: any;
  disableUserSignUp = false;
  loginHint: any;
  passwordHint: any;
  loginError: any;
  navTree: any;
  viewersCanEdit = false;
  editorsCanAdmin = false;
  disableSanitizeHtml = false;
  liveEnabled = true;
  theme: GrafanaTheme;
  theme2: GrafanaTheme2;
  pluginsToPreload: string[] = [];
  featureToggles: FeatureToggles = {
    meta: false,
    ngalert: false,
    reportVariables: false,
    accesscontrol: false,
    trimDefaults: false,
  };
  licenseInfo: LicenseInfo = {} as LicenseInfo;
  rendererAvailable = false;
  rendererVersion = '';
  http2Enabled = false;
  dateFormats?: SystemDateFormatSettings;
  sentry = {
    enabled: false,
    dsn: '',
    customEndpoint: '',
    sampleRate: 1,
  };
  pluginCatalogURL = 'https://grafana.com/grafana/plugins/';
  pluginAdminEnabled = false;
  pluginAdminExternalManageEnabled = false;
  expressionsEnabled = false;
  customTheme?: any;
  awsAllowedAuthProviders: string[] = [];
  awsAssumeRoleEnabled = false;
  azure: AzureSettings = {
    managedIdentityEnabled: false,
  };
  caching = {
    enabled: false,
  };

  constructor(options: GrafanaBootConfig) {
    const mode = options.bootData.user.lightTheme ? 'light' : 'dark';
    this.theme2 = createTheme({ colors: { mode } });
    this.theme = this.theme2.v1;

    const defaults = {
      datasources: {},
      windowTitlePrefix: 'Grafana - ',
      panels: {},
      newPanelTitle: 'Panel Title',
      playlist_timespan: '1m',
      unsaved_changes_warning: true,
      appUrl: '',
      appSubUrl: '',
      buildInfo: {
        version: 'v1.0',
        commit: '1',
        env: 'production',
        isEnterprise: false,
      },
      viewersCanEdit: false,
      editorsCanAdmin: false,
      disableSanitizeHtml: false,
    };

    // LOGZ.IO GRAFANA CHANGE :: DEV-19985: add datasource logos
    changeDatasourceLogos(options.datasources);

    // LOGZ.IO GRAFANA CHANGE :: Add logzio presets to grafana config
    if (Object.keys(logzioConfigs).length === 0) {
      console.error('Error loading logzioConfigs');
    }
    merge(this, defaults, options, logzioConfigs);
    // LOGZ.IO GRAFANA CHANGE :: end

    if (this.dateFormats) {
      systemDateFormats.update(this.dateFormats);
    }
  }
}

const bootData = (window as any).grafanaBootData || {
  settings: {},
  user: {},
  navTree: [],
};

// LOGZ.IO GRAFANA CHANGE :: DEV-26843: add datasource logos
const isPanelEnabled = (window as any).logzio?.configs?.featureFlags?.grafanaFlowchartingPanel;

const panels = bootData?.settings?.panels;

if (panels && !isPanelEnabled) {
  const filteredPanels = Object.fromEntries(
    Object.entries(panels).filter(([key]) => key !== 'agenty-flowcharting-panel')
  );

  bootData.settings.panels = filteredPanels;
}
// LOGZ.IO GRAFANA CHANGE :: end

const options = bootData.settings;
options.bootData = bootData;

/**
 * Use this to access the {@link GrafanaBootConfig} for the current running Grafana instance.
 *
 * @public
 */
export const config = new GrafanaBootConfig(options);
