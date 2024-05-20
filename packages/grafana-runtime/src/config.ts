import merge from 'lodash/merge';
import { getTheme } from '@grafana/ui';
import {
  BuildInfo,
  DataSourceInstanceSettings,
  FeatureToggles,
  GrafanaConfig,
  GrafanaTheme,
  GrafanaThemeType,
  LicenseInfo,
  PanelPluginMeta,
  logzioConfigs, // LOGZ.IO GRAFANA CHANGE :: DEV-20247 Use logzio provider
  systemDateFormats,
  SystemDateFormatSettings,
} from '@grafana/data';
import { changeDatasourceLogos } from './changeDatasourceLogos.logzio'; // LOGZ.IO GRAFANA CHANGE :: DEV-19985: add datasource logos

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
  theme: GrafanaTheme;
  pluginsToPreload: string[] = [];
  featureToggles: FeatureToggles = {
    live: false,
    expressions: false,
    meta: false,
    ngalert: false,
    traceToLogs: false,
  };
  licenseInfo: LicenseInfo = {} as LicenseInfo;
  rendererAvailable = false;
  http2Enabled = false;
  dateFormats?: SystemDateFormatSettings;
  marketplaceUrl?: string;

  constructor(options: GrafanaBootConfig) {
    this.theme = options.bootData.user.lightTheme ? getTheme(GrafanaThemeType.Light) : getTheme(GrafanaThemeType.Dark);

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
