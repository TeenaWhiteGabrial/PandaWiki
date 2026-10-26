import { message } from '@/components/ui';
import type { AxiosError, AxiosInstance, AxiosRequestConfig } from 'axios';
import { handleAdminAuthError } from './authErrors';

export type RequestErrorOptions = { isAlert?: boolean };

type ErrorBody = {
  success?: boolean;
  code?: number;
  message?: string;
  trace_id?: string;
};

const logTraceID = (traceID?: string) => {
  if (traceID) console.error('API error trace_id:', traceID);
};

const shouldAlert = (config?: AxiosRequestConfig & RequestErrorOptions) =>
  config?.isAlert !== false;

// Shared by both generated clients and the legacy client, so a session probe
// cannot trigger a toast or a login-page reload in just one of the clients.
export const installResponseInterceptors = (instance: AxiosInstance) => {
  instance.interceptors.response.use(
    response => {
      const body = response.data;
      if (response.status === 200 && body?.success) return body.data;

      logTraceID(body?.trace_id);
      if (!handleAdminAuthError(response) && shouldAlert(response.config)) {
        message.error(body?.message || response.statusText || '网络异常');
      }
      return Promise.reject(response.status === 200 ? body : response);
    },
    (error: AxiosError<ErrorBody>) => {
      logTraceID(error.response?.data?.trace_id);
      const handled = handleAdminAuthError(error.response);
      if (
        !handled &&
        error.code !== 'ERR_CANCELED' &&
        shouldAlert(error.config)
      ) {
        message.error(
          error.response?.data?.message ||
            error.response?.statusText ||
            '网络异常',
        );
      }
      return Promise.reject(error.response || error);
    },
  );
};
