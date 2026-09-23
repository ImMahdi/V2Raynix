import React, { useState, useEffect } from 'react';
import Navbar from './components/Navbar';
import SafeModeModal from './components/SafeModeModal';
import ErrorBoundary from './components/ErrorBoundary';
import DashboardPage from './pages/DashboardPage';
import ConfigsPage from './pages/ConfigsPage';
import RoutingPage from './pages/RoutingPage';
import LogsPage from './pages/LogsPage';
import SettingsPage from './pages/SettingsPage';
import LoginPage from './pages/LoginPage';
import { api, setToken, getToken, onUnauthorized } from './services/api';

export default function App() {
  const [user, setUser] = useState(null);
  const [checkingAuth, setCheckingAuth] = useState(true);
  const [activeTab, setActiveTab] = useState('dashboard');

  const [status, setStatus] = useState(null);
  const [configs, setConfigs] = useState([]);
  const [routingRules, setRoutingRules] = useState([]);
  const [toggling, setToggling] = useState(false);

  // Check initial login session
  useEffect(() => {
    onUnauthorized(() => {
      setUser(null);
    });

    const token = getToken();
    if (!token) {
      setUser(null);
      setCheckingAuth(false);
      return;
    }

    api.getMe()
      .then(res => {
        if (res.authenticated) {
          setUser({ username: res.username });
        } else {
          setUser(null);
        }
      })
      .catch(() => {
        setUser(null);
      })
      .finally(() => {
        setCheckingAuth(false);
      });
  }, []);

  // Poll tunnel status and fetch data with exponential backoff & jitter (WEB-01)
  useEffect(() => {
    if (!user) return;

    let timerId = null;
    let isCancelled = false;
    let failureCount = 0;

    const poll = async () => {
      try {
        const [stat, cfgs, rls] = await Promise.all([
          api.getTunnelStatus(),
          api.getConfigs(),
          api.getRoutingRules(),
        ]);
        if (isCancelled) return;
        setStatus(stat);
        setConfigs(cfgs || []);
        setRoutingRules(rls || []);
        failureCount = 0;
      } catch (err) {
        if (isCancelled) return;
        failureCount++;
      } finally {
        if (!isCancelled) {
          // Standard delay 2000ms; exponential backoff up to 30s + 0-500ms jitter on failure
          const delay = failureCount === 0
            ? 2000
            : Math.min(2000 * Math.pow(1.5, failureCount), 30000) + Math.random() * 500;
          timerId = setTimeout(poll, delay);
        }
      }
    };

    poll();
    return () => {
      isCancelled = true;
      if (timerId) clearTimeout(timerId);
    };
  }, [user]);

  const handleLogout = () => {
    setToken(null);
    setUser(null);
  };

  // Tunnel control with immediate toggling lock (WEB-04)
  const handleToggleTunnel = async () => {
    if (toggling) return;
    setToggling(true);
    try {
      if (status?.state === 'connected') {
        const newStat = await api.disconnectTunnel();
        setStatus(newStat);
      } else {
        const newStat = await api.connectTunnel();
        setStatus(newStat);
      }
    } catch (err) {
      alert(err.message || 'Tunnel operation failed');
    } finally {
      setToggling(false);
    }
  };

  // Safe Mode actions
  const handleConfirmSafeMode = async () => {
    try {
      await api.confirmSafeMode();
      const newStat = await api.getTunnelStatus();
      setStatus(newStat);
    } catch (err) {
      alert(err.message || 'Failed to confirm safe mode');
    }
  };

  const handleRollbackSafeMode = async () => {
    try {
      await api.rollbackSafeMode();
      const newStat = await api.getTunnelStatus();
      setStatus(newStat);
    } catch (err) {
      alert(err.message || 'Failed to rollback safe mode');
    }
  };

  // Config actions
  const handleActivateConfig = async (id) => {
    setStatus(prev => (prev ? { ...prev, activeConfigId: id } : { activeConfigId: id }));
    try {
      await api.activateConfig(id);
      const [newStat, newCfgs] = await Promise.all([
        api.getTunnelStatus(),
        api.getConfigs(),
      ]);
      setStatus(newStat);
      setConfigs(newCfgs);
    } catch (err) {
      alert(err.message || 'Failed to activate config');
    }
  };

  const handleDeleteConfig = async (id) => {
    if (!window.confirm('Are you sure you want to delete this config?')) return;
    try {
      await api.deleteConfig(id);
      setConfigs(configs.filter(c => c.id !== id));
    } catch (err) {
      alert(err.message || 'Failed to delete config');
    }
  };

  const handleTestConfig = async (id) => {
    try {
      const res = await api.testConfig(id);
      setConfigs(prev => prev.map(c => (c.id === id ? { ...c, latencyMs: res.latencyMs } : c)));
    } catch (err) {
      console.error('Failed to test config:', err);
    }
  };

  const handleTestAll = async () => {
    try {
      const res = await api.testAll();
      setConfigs(prev => prev.map(c => ({
        ...c,
        latencyMs: res[c.id] !== undefined ? res[c.id] : c.latencyMs,
      })));
    } catch (err) {
      alert(err.message || 'Failed to test configs');
    }
  };

  const handlePingConfig = handleTestConfig;
  const handlePingAll = handleTestAll;

  const handleImportConfig = async (content, name) => {
    await api.createConfig(content, name);
    const newCfgs = await api.getConfigs();
    setConfigs(newCfgs);
  };

  // Routing actions
  const handleCreateRule = async (rule) => {
    await api.createRoutingRule(rule);
    const newRules = await api.getRoutingRules();
    setRoutingRules(newRules);
  };

  const handleDeleteRule = async (id) => {
    await api.deleteRoutingRule(id);
    setRoutingRules(routingRules.filter(r => r.id !== id));
  };

  if (checkingAuth) {
    return (
      <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--text-muted)' }}>
        Loading V2Raynix...
      </div>
    );
  }

  if (!user) {
    return <LoginPage onLoginSuccess={setUser} />;
  }

  return (
    <div className="app-container">
      <Navbar 
        activeTab={activeTab} 
        onSelectTab={setActiveTab} 
        user={user} 
        onLogout={handleLogout} 
      />

      <main className="main-content">
        <ErrorBoundary>
          {activeTab === 'dashboard' && (
            <DashboardPage 
              status={status}
              configs={configs}
              onToggleTunnel={handleToggleTunnel}
              onSelectTab={setActiveTab}
              toggling={toggling}
            />
          )}

          {activeTab === 'configs' && (
            <ConfigsPage 
              configs={configs}
              activeId={status?.activeConfigId}
              onActivate={handleActivateConfig}
              onDelete={handleDeleteConfig}
              onPing={handleTestConfig}
              onPingAll={handleTestAll}
              onTestConfig={handleTestConfig}
              onTestAll={handleTestAll}
              onImport={handleImportConfig}
            />
          )}

          {activeTab === 'routing' && (
            <RoutingPage 
              rules={routingRules}
              onCreateRule={handleCreateRule}
              onDeleteRule={handleDeleteRule}
            />
          )}

          {activeTab === 'logs' && <LogsPage />}

          {activeTab === 'settings' && <SettingsPage />}
        </ErrorBoundary>
      </main>

      {/* Safe Mode Auto-Rollback Modal */}
      {status?.safeMode?.isActive && (
        <SafeModeModal 
          remainingSeconds={status.safeMode.remainingSeconds}
          onConfirm={handleConfirmSafeMode}
          onRollback={handleRollbackSafeMode}
        />
      )}
    </div>
  );
}
