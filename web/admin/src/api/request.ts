import axios, { type AxiosRequestConfig } from 'axios';
import {
  installResponseInterceptors,
  type RequestErrorOptions,
} from '@/request/responseInterceptors';

const request = <T>(
  options: AxiosRequestConfig & RequestErrorOptions,
): Promise<T> => {
  const service = axios.create({
    baseURL: window.__BASENAME__ || '/',
    timeout: 0,
    withCredentials: true,
    headers: {
      'X-PW-CSRF': '1',
    },
  });
  installResponseInterceptors(service);
  return service.request<unknown, T>(options);
};

export default request;
