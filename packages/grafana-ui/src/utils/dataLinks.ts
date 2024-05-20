import { ContextMenuItem } from '../components/ContextMenu/ContextMenu';
import { LinkModel, locationUtil } from '@grafana/data'; // LOGZ.IO GRAFANA CHANGE :: DEV-23541 Use the url without the grafana-app part
/**
 * Delays creating links until we need to open the ContextMenu
 */
export const linkModelToContextMenuItems: (links: () => LinkModel[]) => ContextMenuItem[] = links => {
  return links().map(link => {
    return {
      label: link.title,
      // TODO: rename to href
      url: locationUtil.stripBaseFromUrl(link.href), // LOGZ.IO GRAFANA CHANGE :: DEV-23541 Use the url without the grafana-app part
      target: link.target,
      icon: `${link.target === '_self' ? 'link' : 'external-link-alt'}`,
      onClick: link.onClick,
    };
  });
};
