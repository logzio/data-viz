import React, { PureComponent } from 'react';
import { selectors as e2eSelectors } from '@grafana/e2e-selectors';
import { Field, RadioButtonGroup, Switch, ClipboardButton, Icon, Input, FieldSet, Alert } from '@grafana/ui';
// LOGZ.IO GRAFANA CHANGE :: DEV-20247 Use logzio provider
import { SelectableValue, PanelModel, AppEvents, logzioServices, logzioConfigs } from '@grafana/data';
import { DashboardModel } from 'app/features/dashboard/state';
import { buildImageUrl, buildShareUrl } from './utils';
import { appEvents } from 'app/core/core';
import config from 'app/core/config';

const themeOptions: Array<SelectableValue<string>> = [
  { label: 'Current', value: 'current' },
  { label: 'Dark', value: 'dark' },
  { label: 'Light', value: 'light' },
];

export interface Props {
  dashboard: DashboardModel;
  panel?: PanelModel;
}

export interface State {
  useCurrentTimeRange: boolean;
  useShortUrl: boolean;
  selectedTheme: string;
  shareUrl: string;
  imageUrl: string;
}

export class ShareLink extends PureComponent<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = {
      useCurrentTimeRange: true,
      useShortUrl: false,
      selectedTheme: 'current',
      shareUrl: '',
      imageUrl: '',
    };
  }

  componentDidMount() {
    this.buildUrl();
  }

  componentDidUpdate(prevProps: Props, prevState: State) {
    const { useCurrentTimeRange, useShortUrl, selectedTheme } = this.state;
    if (
      prevState.useCurrentTimeRange !== useCurrentTimeRange ||
      prevState.selectedTheme !== selectedTheme ||
      prevState.useShortUrl !== useShortUrl
    ) {
      this.buildUrl();
    }
  }

  buildUrl = async () => {
    const { panel } = this.props;
    const { useCurrentTimeRange, useShortUrl, selectedTheme } = this.state;

    // LOGZ.IO GRAFANA CHANGE :: DEV-19527 Add await to function call
    const grafanaShareUrl = await buildShareUrl(useCurrentTimeRange, selectedTheme, panel, useShortUrl);

    const shareUrl = await logzioServices.shareUrlService.getLogzioGrafanaUrl({
      productUrl: grafanaShareUrl,
      switchToAccountId: logzioConfigs.account.accountId,
    });
    // LOGZ.IO GRAFANA CHANGE :: end

    const imageUrl = buildImageUrl(useCurrentTimeRange, selectedTheme, panel);

    this.setState({ shareUrl, imageUrl });
  };

  onUseCurrentTimeRangeChange = () => {
    this.setState({ useCurrentTimeRange: !this.state.useCurrentTimeRange });
  };

  onUrlShorten = () => {
    this.setState({ useShortUrl: !this.state.useShortUrl });
  };

  onThemeChange = (value: string) => {
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
    const isRelativeTime = this.props.dashboard ? this.props.dashboard.time.to === 'now' : false;
    const { useCurrentTimeRange, selectedTheme, shareUrl, imageUrl } = this.state; // LOGZ.IO GRAFNA CHANGE :: DEV-23431 Remove useShortenUrl
    const selectors = e2eSelectors.pages.SharePanelModal;

    return (
      <>
        <p className="share-modal-info-text">
          Create a direct link to this dashboard or panel, customized with the options below.
        </p>
        <FieldSet>
          <Field
            label="Lock time range"
            description={isRelativeTime ? 'Transforms the current relative time range to an absolute time range' : ''}
          >
            <Switch
              id="share-current-time-range"
              value={useCurrentTimeRange}
              onChange={this.onUseCurrentTimeRangeChange}
            />
          </Field>
          <Field label="Theme">
            <RadioButtonGroup options={themeOptions} value={selectedTheme} onChange={this.onThemeChange} />
          </Field>
          {/* LOGZ.IO GRAFNA CHANGE :: DEV-23431 Remove short url switcher*/}
          {/*<Field label="Shorten URL">*/}
          {/*  <Switch id="share-shorten-url" value={useShortUrl} onChange={this.onUrlShorten} />*/}
          {/*</Field>*/}

          <Field label="Link URL">
            <Input
              value={shareUrl}
              readOnly
              addonAfter={
                <ClipboardButton variant="primary" getText={this.getShareUrl} onClipboardCopy={this.onShareUrlCopy}>
                  <Icon name="copy" /> Copy
                </ClipboardButton>
              }
            />
          </Field>
        </FieldSet>
        {panel && config.rendererAvailable && (
          <div className="gf-form">
            <a href={imageUrl} target="_blank" rel="noreferrer" aria-label={selectors.linkToRenderedImage}>
              <Icon name="camera" /> Direct link rendered image
            </a>
          </div>
        )}
        {panel && !config.rendererAvailable && (
          <Alert severity="info" title="Image renderer plugin not installed" bottomSpacing={0}>
            <>To render a panel image, you must install the </>
            <a
              href="https://grafana.com/grafana/plugins/grafana-image-renderer"
              target="_blank"
              rel="noopener noreferrer"
              className="external-link"
            >
              Grafana image renderer plugin
            </a>
            . Please contact your Grafana administrator to install the plugin.
          </Alert>
        )}
      </>
    );
  }
}
