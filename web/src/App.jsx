import React, { useState, useEffect } from 'react';
import Navbar from './components/Navbar';
import SafeModeModal from './components/SafeModeModal';
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

  // Poll tunnel status and fetch data when authenticated
  useEffect(() => {
    if (!user) return;

    const refreshData = async () => {
      try {
        const [stat, cfgs, rls] = await Promise.all([
          api.getTunnelStatus(),
          api.getConfigs(),
          api.getRoutingRules(),
        ]);
        setStatus(stat);
        setConfigs(cfgs || []);
        setRoutingRules(rls || []);
      } catch (err) {
        console.error('Error refreshing state:', err);
      }
    };

    refreshData();
    const interval = setInterval(refreshData, 2000);
    return () => clearInterval(interval);
  }, [user]);

  const handleLogout = () => {
    setToken(null);
    setUser(null);
  };

  // Tunnel control
  const handleToggleTunnel = async () => {
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

  const handlePingConfig = async (id) => {
    // Single config ping can trigger all or refresh
    try {
      const res = await api.pingAll();
      setConfigs(prev => prev.map(c => ({
        ...c,
        latencyMs: res[c.id] !== undefined ? res[c.id] : c.latencyMs,
      })));
    } catch (err) {
      console.error(err);
    }
  };

  const handlePingAll = async () => {
    try {
      const res = await api.pingAll();
      setConfigs(prev => prev.map(c => ({
        ...c,
        latencyMs: res[c.id] !== undefined ? res[c.id] : c.latencyMs,
      })));
    } catch (err) {
      alert(err.message || 'Failed to ping configs');
    }
  };

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
        {activeTab === 'dashboard' && (
          <DashboardPage 
            status={status}
            configs={configs}
            onToggleTunnel={handleToggleTunnel}
            onSelectTab={setActiveTab}
          />
        )}

        {activeTab === 'configs' && (
          <ConfigsPage 
            configs={configs}
            activeId={status?.activeConfigId}
            onActivate={handleActivateConfig}
            onDelete={handleDeleteConfig}
            onPing={handlePingConfig}
            onPingAll={handlePingAll}
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
