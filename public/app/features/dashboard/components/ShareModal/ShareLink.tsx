import React, { PureComponent } from 'react';
import { selectors as e2eSelectors } from '@grafana/e2e-selectors';
import { LegacyForms, ClipboardButton, Icon, InfoBox, Input } from '@grafana/ui';
const { Select, Switch } = LegacyForms;
// LOGZ.IO GRAFANA CHANGE :: DEV-20247 Use logzio provider
import { SelectableValue, PanelModel, AppEvents, logzioServices, logzioConfigs } from '@grafana/data';
import { getBackendSrv } from '@grafana/runtime';
import { DashboardModel } from 'app/features/dashboard/state';
import { buildImageUrl, buildShareUrl, getRelativeURLPath } from './utils';
import { appEvents } from 'app/core/core';
import config from 'app/core/config';

const themeOptions: Array<SelectableValue<string>> = [
  { label: 'current', value: 'current' },
  { label: 'dark', value: 'dark' },
  { label: 'light', value: 'light' },
];

export interface Props {
  dashboard: DashboardModel;
  panel?: PanelModel;
}

export interface State {
  useCurrentTimeRange: boolean;
  includeTemplateVars: boolean;
  useShortUrl: boolean;
  selectedTheme: SelectableValue<string>;
  shareUrl: string;
  imageUrl: string;
}

export class ShareLink extends PureComponent<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = {
      useCurrentTimeRange: true,
      includeTemplateVars: true,
      useShortUrl: false,
      selectedTheme: themeOptions[0],
      shareUrl: '',
      imageUrl: '',
    };
  }

  componentDidMount() {
    this.buildUrl();
  }

  componentDidUpdate(prevProps: Props, prevState: State) {
    const { useCurrentTimeRange, includeTemplateVars, useShortUrl, selectedTheme } = this.state;
    if (
      prevState.useCurrentTimeRange !== useCurrentTimeRange ||
      prevState.includeTemplateVars !== includeTemplateVars ||
      prevState.selectedTheme.value !== selectedTheme.value ||
      prevState.useShortUrl !== useShortUrl
    ) {
      this.buildUrl();
    }
  }

  // LOGZ.IO GRAFANA CHANGE :: DEV-19527 change func to async
  buildUrl = async () => {
    const { panel } = this.props;
    const { useCurrentTimeRange, includeTemplateVars, useShortUrl, selectedTheme } = this.state;

    const grafanaShareUrl = buildShareUrl(useCurrentTimeRange, includeTemplateVars, selectedTheme.value, panel);

    // LOGZ.IO GRAFANA CHANGE :: DEV-19527 Add await to function call
    const shareUrl = await logzioServices.shareUrlService.getLogzioGrafanaUrl({
      productUrl: grafanaShareUrl,
      switchToAccountId: logzioConfigs.account.accountId,
    });
    // LOGZ.IO GRAFANA CHANGE :: end

    const imageUrl = buildImageUrl(useCurrentTimeRange, includeTemplateVars, selectedTheme.value, panel);

    if (useShortUrl) {
      getBackendSrv()
        .post(`/api/short-urls`, {
          path: getRelativeURLPath(shareUrl),
        })
        .then(res => this.setState({ shareUrl: res.url, imageUrl }));
    } else {
      this.setState({ shareUrl, imageUrl });
    }
  };

  onUseCurrentTimeRangeChange = () => {
    this.setState({ useCurrentTimeRange: !this.state.useCurrentTimeRange });
  };

  onIncludeTemplateVarsChange = () => {
    this.setState({ includeTemplateVars: !this.state.includeTemplateVars });
  };

  onUrlShorten = () => {
    this.setState({ useShortUrl: !this.state.useShortUrl });
  };

  onThemeChange = (value: SelectableValue<string>) => {
    this.setState({ selectedTheme: value });
  };

  onShareUrlCopy = () => {
    appEvents.emit(AppEvents.alertSuccess, ['Content copied to clipboard']);
  };

  getShareUrl = () => {
    return this.state.shareUrl;
  };

  render() {
    const { panel } = this.props;
    const { useCurrentTimeRange, includeTemplateVars, selectedTheme, shareUrl, imageUrl } = this.state; // LOGZ.IO GRAFNA CHANGE :: DEV-23431 Remove useShortenUrl
    const selectors = e2eSelectors.pages.SharePanelModal;

    return (
      <div className="share-modal-body">
        <div className="share-modal-header">
          <Icon name="link" className="share-modal-big-icon" size="xxl" />
          <div className="share-modal-content">
            <p className="share-modal-info-text">
              Create a direct link to this dashboard or panel, customized with the options below.
            </p>
            <div className="gf-form-group">
              <Switch
                labelClass="width-12"
                label="Current time range"
                checked={useCurrentTimeRange}
                onChange={this.onUseCurrentTimeRangeChange}
              />
              <Switch
                labelClass="width-12"
                label="Template variables"
                checked={includeTemplateVars}
                onChange={this.onIncludeTemplateVarsChange}
              />
              {/* // LOGZ.IO GRAFNA CHANGE :: DEV-23431 Remove short url switcher
              <Switch labelClass="width-12" label="Shorten URL" checked={useShortUrl} onChange={this.onUrlShorten} /> */}
              <div className="gf-form">
                <label className="gf-form-label width-12">Theme</label>
                <Select width={10} options={themeOptions} value={selectedTheme} onChange={this.onThemeChange} />
              </div>
            </div>
            <div>
              <div className="gf-form-group">
                <div className="gf-form-inline">
                  <div className="gf-form gf-form--grow">
                    <Input
                      value={shareUrl}
                      readOnly
                      addonAfter={
                        <ClipboardButton
                          variant="primary"
                          getText={this.getShareUrl}
                          onClipboardCopy={this.onShareUrlCopy}
                        >
                          <Icon name="copy" /> Copy
                        </ClipboardButton>
                      }
                    />
                  </div>
                </div>
              </div>
            </div>
            {panel && config.rendererAvailable && (
              <div className="gf-form">
                <a href={imageUrl} target="_blank" aria-label={selectors.linkToRenderedImage}>
                  <Icon name="camera" /> Direct link rendered image
                </a>
              </div>
            )}
            {panel && !config.rendererAvailable && (
              <InfoBox>
                <p>
                  <>To render a panel image, you must install the </>
                  <a
                    href="https://grafana.com/grafana/plugins/grafana-image-renderer"
                    target="_blank"
                    rel="noopener"
                    className="external-link"
                  >
                    Grafana Image Renderer plugin
                  </a>
                  . Please contact your Grafana administrator to install the plugin.
                </p>
              </InfoBox>
            )}
          </div>
        </div>
      </div>
    );
  }
}
