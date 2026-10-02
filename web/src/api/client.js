import { request, api, getToken, setToken, onUnauthorized } from '../services/api';

export { request, getToken, setToken, onUnauthorized };

export const checkHealth = (targetUrl) =>
  request('/health/check', {
    method: 'POST',
    body: targetUrl ? JSON.stringify({ targetUrl }) : undefined,
  });

export const getSettings = () => request('/settings');

export const saveSettings = (settings) =>
  request('/settings', {
    method: 'POST',
    body: JSON.stringify(settings),
  });

export const updateSettings = saveSettings;

export const apiClient = {
  ...api,
  checkHealth,
  getSettings,
  saveSettings,
  updateSettings,
};

export default apiClient;
