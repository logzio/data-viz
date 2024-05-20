import React, { PureComponent } from 'react';
import appEvents from '../../app_events';
import TopSection from './TopSection';
import BottomSection from './BottomSection';
// import config from 'app/core/config'; // LOGZ.IO GRAFANA CHANGE :: comment out to prevent ts errors
import { CoreEvents } from 'app/types';
// import { Branding } from 'app/core/components/Branding/Branding'; // LOGZ.IO GRAFANA CHANGE :: comment out to prevent ts errors
import { Icon } from '@grafana/ui';

// const homeUrl = config.appSubUrl || '/'; // LOGZ.IO GRAFANA CHANGE :: comment out to prevent ts errors

export class SideMenu extends PureComponent {
  toggleSideMenuSmallBreakpoint = () => {
    appEvents.emit(CoreEvents.toggleSidemenuMobile);
  };

  render() {
    return [
      // LOGZ.IO GRAFANA CHANGE :: remove grafana sidemenu logo
      // <a href={homeUrl} className="sidemenu__logo" key="logo">
      //   <Branding.MenuLogo />
      // </a>,
      // LOGZ.IO GRAFANA CHANGE :: END
      <div className="sidemenu__logo_small_breakpoint" onClick={this.toggleSideMenuSmallBreakpoint} key="hamburger">
        <Icon name="bars" size="xl" />
        <span className="sidemenu__close">
          <Icon name="times" />
          &nbsp;Close
        </span>
      </div>,
      <TopSection key="topsection" />,
      <BottomSection key="bottomsection" />,
    ];
  }
}
