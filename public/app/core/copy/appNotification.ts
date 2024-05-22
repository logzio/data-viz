import { AppNotification, AppNotificationSeverity, AppNotificationTimeout } from 'app/types';
import { getMessageFromError } from 'app/core/utils/errors';
import { v4 as uuidv4 } from 'uuid';
import { logzioServices } from '@grafana/data';

const defaultSuccessNotification = {
  title: '',
  text: '',
  severity: AppNotificationSeverity.Success,
  icon: 'check',
  timeout: AppNotificationTimeout.Success,
};

const defaultWarningNotification = {
  title: '',
  text: '',
  severity: AppNotificationSeverity.Warning,
  icon: 'exclamation-triangle',
  timeout: AppNotificationTimeout.Warning,
};

const defaultErrorNotification = {
  title: '',
  text: '',
  severity: AppNotificationSeverity.Error,
  icon: 'exclamation-triangle',
  timeout: AppNotificationTimeout.Error,
};

export const createSuccessNotification = (title: string, text = ''): AppNotification => ({
  ...defaultSuccessNotification,
  title: title,
  text: text,
  id: uuidv4(),
});

export const createErrorNotification = (
  title: string,
  text: string | Error = '',
  component?: React.ReactElement
): AppNotification => {
  // LOGZ.IO GRAFANA CHANGE :: DEV-23041 - log to logzio on any error
  const logzLogger = logzioServices.LoggerService;
  logzLogger.logError(logzLogger.Origin.GRAFANA, getMessageFromError(text), null, logzLogger.UXType.TOAST, false, {
    title,
  });

  return {
    ...defaultErrorNotification,
    text: getMessageFromError(text),
    title,
    id: uuidv4(),
    component,
  };
};

export const createWarningNotification = (title: string, text = ''): AppNotification => ({
  ...defaultWarningNotification,
  title: title,
  text: text,
  id: uuidv4(),
});
