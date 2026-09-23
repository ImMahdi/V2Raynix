const API_BASE = '/api';

export function getToken() {
  try {
    return sessionStorage.getItem('v2raynix_token') || localStorage.getItem('v2raynix_token');
  } catch (e) {
    return null;
  }
}

let onUnauthorizedCallback = null;

export function onUnauthorized(callback) {
  onUnauthorizedCallback = callback;
  return () => {
    if (onUnauthorizedCallback === callback) {
      onUnauthorizedCallback = null;
    }
  };
}

export function setToken(token) {
  try {
    if (token) {
      sessionStorage.setItem('v2raynix_token', token);
      localStorage.setItem('v2raynix_token', token);
    } else {
      sessionStorage.removeItem('v2raynix_token');
      localStorage.removeItem('v2raynix_token');
    }
  } catch (e) {
    console.warn('Storage operation failed:', e);
  }
}

async function request(endpoint, options = {}) {
  const url = `${API_BASE}${endpoint}`;
  const token = getToken();

  const headers = {
    'Content-Type': 'application/json',
    ...(options.headers || {}),
  };

  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const response = await fetch(url, {
    ...options,
    headers,
  });

  if (response.status === 401 && endpoint !== '/auth/login') {
    setToken(null);
    if (onUnauthorizedCallback) {
      onUnauthorizedCallback();
    }
    throw new Error('Session expired, please log in again.');
  }

  const data = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(data.error || `HTTP ${response.status}: Request failed`);
  }

  return data;
}

export const api = {
  // Auth
  login: (username, password) => request('/auth/login', { method: 'POST', body: JSON.stringify({ username, password }) }),
  getMe: () => request('/auth/me'),
  changePassword: (currentPassword, newPassword) => request('/auth/password', { method: 'POST', body: JSON.stringify({ currentPassword, newPassword }) }),

  // Configs
  getConfigs: () => request('/configs'),
  createConfig: (content, name) => request('/configs', { method: 'POST', body: JSON.stringify({ content, name }) }),
  deleteConfig: (id) => request(`/configs/${id}`, { method: 'DELETE' }),
  activateConfig: (id) => request(`/configs/${id}/activate`, { method: 'POST' }),
  pingAll: () => request('/configs/ping-all', { method: 'POST' }),
  testConfig: (id) => request(`/configs/${id}/test`, { method: 'POST' }),
  testAll: () => request('/configs/test-all', { method: 'POST' }),

  // Tunnel & Safe Mode
  getTunnelStatus: () => request('/tunnel/status'),
  connectTunnel: (configId) => request('/tunnel/connect', { method: 'POST', body: JSON.stringify({ configId }) }),
  disconnectTunnel: () => request('/tunnel/disconnect', { method: 'POST' }),
  confirmSafeMode: () => request('/tunnel/safe-mode/confirm', { method: 'POST' }),
  rollbackSafeMode: () => request('/tunnel/safe-mode/rollback', { method: 'POST' }),

  // Routing Rules
  getRoutingRules: () => request('/routing/rules'),
  createRoutingRule: (rule) => request('/routing/rules', { method: 'POST', body: JSON.stringify(rule) }),
  deleteRoutingRule: (id) => request(`/routing/rules/${id}`, { method: 'DELETE' }),

  // Logs
  getLogs: (options = {}) => request('/system/logs', options),
};

