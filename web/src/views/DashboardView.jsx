import React, { useState } from 'react';
import {
  Power,
  Shield,
  Activity,
  Network,
  Clock,
  HeartPulse,
  CheckCircle2,
  AlertCircle,
  RefreshCw,
} from 'lucide-react';
import { apiClient } from '../api/client';

export default function DashboardView({
  status,
  configs = [],
  onToggleTunnel,
  onSelectTab,
  toggling,
}) {
  const isConnected = status?.state === 'connected';
  const isConnecting = status?.state === 'connecting';
  const activeConfig = (configs || []).find((c) => c.id === status?.activeConfigId);

  const [healthState, setHealthState] = useState({
    checked: false,
    loading: false,
    healthy: null,
    latencyMs: null,
    error: null,
    lastChecked: null,
  });

  const handleCheckHealth = async () => {
    if (healthState.loading) return;
    setHealthState((prev) => ({ ...prev, loading: true, error: null }));
    try {
      const res = await apiClient.checkHealth();
      setHealthState({
        checked: true,
        loading: false,
        healthy: Boolean(res.healthy),
        latencyMs: typeof res.latencyMs === 'number' ? res.latencyMs : null,
        error: res.error || null,
        lastChecked: new Date(),
      });
    } catch (err) {
      setHealthState({
        checked: true,
        loading: false,
        healthy: false,
        latencyMs: null,
        error: err.message || 'Health probe failed',
        lastChecked: new Date(),
      });
    }
  };

  const formatUptime = (sec) => {
    if (!sec) return '0s';
    const hrs = Math.floor(sec / 3600);
    const mins = Math.floor((sec % 3600) / 60);
    const s = sec % 60;
    if (hrs > 0) return `${hrs}h ${mins}m`;
    if (mins > 0) return `${mins}m ${s}s`;
    return `${s}s`;
  };

  return (
    <div className="dashboard-container">
      {/* Master Toggle Banner */}
      <div className="glass-card master-switch-container" style={{ marginBottom: '1.5rem' }}>
        <button
          className={`master-btn ${isConnected ? 'connected' : ''} ${
            isConnecting || toggling ? 'connecting' : ''
          }`}
          onClick={onToggleTunnel}
          disabled={isConnecting || toggling}
          aria-label="Toggle Tunnel Connection"
        >
          <Power size={48} strokeWidth={2.5} />
          <span style={{ fontSize: '0.85rem', fontWeight: 700, letterSpacing: '0.05em' }}>
            {toggling
              ? 'SWITCHING...'
              : isConnected
              ? 'CONNECTED'
              : isConnecting
              ? 'STARTING...'
              : 'DISCONNECTED'}
          </span>
        </button>

        <div style={{ marginTop: '1.5rem', textAlign: 'center' }}>
          <div style={{ fontSize: '1.1rem', fontWeight: 600, color: 'var(--text-primary)' }}>
            {isConnected ? 'Server is Fully Tunneled' : 'System Routing is Direct'}
          </div>
          <div style={{ fontSize: '0.85rem', color: 'var(--text-muted)', marginTop: '0.25rem' }}>
            {isConnected
              ? `All outbound traffic routed via ${status?.tunInterface || 'tun0'}`
              : 'Click above to tunnel all server traffic through active proxy'}
          </div>
        </div>
      </div>

      {/* Responsive Metric Cards Grid */}
      <div
        className="grid-stats"
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))',
          gap: '1rem',
          marginTop: '1.5rem',
        }}
      >
        {/* Health Status & Quick Probe Card */}
        <div
          className="glass-card health-probe-card"
          style={{ display: 'flex', flexDirection: 'column', justifyContent: 'space-between' }}
        >
          <div>
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                color: 'var(--text-muted)',
                fontSize: '0.8rem',
                fontWeight: 600,
                textTransform: 'uppercase',
                marginBottom: '0.5rem',
              }}
            >
              <span>Health Status</span>
              <HeartPulse
                size={16}
                color={
                  healthState.loading
                    ? '#38bdf8'
                    : healthState.checked
                    ? healthState.healthy
                      ? '#34d399'
                      : '#fb7185'
                    : 'var(--text-muted)'
                }
              />
            </div>

            <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', marginTop: '0.25rem' }}>
              {healthState.loading ? (
                <div style={{ fontSize: '1.15rem', fontWeight: 700, color: '#38bdf8' }}>
                  Probing...
                </div>
              ) : !healthState.checked ? (
                <div style={{ fontSize: '1.15rem', fontWeight: 700, color: 'var(--text-secondary)' }}>
                  Not verified yet
                </div>
              ) : healthState.healthy ? (
                <div style={{ display: 'flex', alignItems: 'center', gap: '0.35rem' }}>
                  <CheckCircle2 size={18} color="#34d399" />
                  <span style={{ fontSize: '1.15rem', fontWeight: 700, color: '#34d399' }}>
                    Online ({healthState.latencyMs !== null ? `${healthState.latencyMs}ms` : 'OK'})
                  </span>
                </div>
              ) : (
                <div style={{ display: 'flex', alignItems: 'center', gap: '0.35rem' }}>
                  <AlertCircle size={18} color="#fb7185" />
                  <span style={{ fontSize: '1.15rem', fontWeight: 700, color: '#fb7185' }}>
                    Unhealthy
                  </span>
                </div>
              )}
            </div>

            <div
              style={{
                fontSize: '0.8rem',
                color: healthState.error ? '#fb7185' : 'var(--text-muted)',
                marginTop: '0.35rem',
                lineHeight: 1.4,
                wordBreak: 'break-word',
              }}
            >
              {healthState.loading
                ? 'Testing HTTP 204 connectivity...'
                : healthState.error
                ? healthState.error
                : healthState.checked
                ? `Latency: ${healthState.latencyMs}ms • Endpoint verified`
                : 'Click button below to probe proxy connectivity.'}
            </div>
          </div>

          <button
            className="btn btn-secondary"
            style={{
              width: '100%',
              marginTop: '1rem',
              padding: '0.45rem 0.75rem',
              fontSize: '0.8rem',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              gap: '0.5rem',
            }}
            onClick={handleCheckHealth}
            disabled={healthState.loading}
          >
            {healthState.loading ? (
              <>
                <RefreshCw size={14} className="spin-animate" />
                <span>Checking...</span>
              </>
            ) : (
              <>
                <HeartPulse size={14} />
                <span>Check Health Now</span>
              </>
            )}
          </button>
        </div>

        {/* Active Config Card */}
        <div
          className="glass-card"
          style={{ display: 'flex', flexDirection: 'column', justifyContent: 'space-between' }}
        >
          <div>
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                color: 'var(--text-muted)',
                fontSize: '0.8rem',
                fontWeight: 600,
                textTransform: 'uppercase',
                marginBottom: '0.5rem',
              }}
            >
              <span>Active Config</span>
              <Network size={16} />
            </div>
            <div
              style={{
                fontSize: '1.15rem',
                fontWeight: 700,
                color: 'var(--text-primary)',
                whiteSpace: 'nowrap',
                overflow: 'hidden',
                textOverflow: 'ellipsis',
              }}
            >
              {activeConfig ? activeConfig.name : 'None Selected'}
            </div>
            <div
              style={{
                fontSize: '0.8rem',
                color: 'var(--text-muted)',
                marginTop: '0.25rem',
                fontFamily: 'var(--font-mono)',
              }}
            >
              {activeConfig
                ? `${activeConfig.protocol} • ${activeConfig.server}:${activeConfig.port}`
                : 'Go to Configs tab to select'}
            </div>
          </div>
          {onSelectTab && (
            <button
              className="btn btn-secondary"
              style={{ width: '100%', marginTop: '1rem', padding: '0.45rem 0.75rem', fontSize: '0.8rem' }}
              onClick={() => onSelectTab('configs')}
            >
              Manage Configs
            </button>
          )}
        </div>

        {/* Uptime Card */}
        <div
          className="glass-card"
          style={{ display: 'flex', flexDirection: 'column', justifyContent: 'space-between' }}
        >
          <div>
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                color: 'var(--text-muted)',
                fontSize: '0.8rem',
                fontWeight: 600,
                textTransform: 'uppercase',
                marginBottom: '0.5rem',
              }}
            >
              <span>Tunnel Uptime</span>
              <Clock size={16} />
            </div>
            <div
              style={{
                fontSize: '1.75rem',
                fontWeight: 800,
                color: isConnected ? '#34d399' : 'var(--text-muted)',
              }}
            >
              {formatUptime(status?.uptimeSeconds)}
            </div>
            <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginTop: '0.25rem' }}>
              {isConnected ? `Interface: ${status?.tunInterface || 'tun0'}` : 'Tunnel offline'}
            </div>
          </div>
        </div>

        {/* Security & Safe Mode Card */}
        <div
          className="glass-card"
          style={{ display: 'flex', flexDirection: 'column', justifyContent: 'space-between' }}
        >
          <div>
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                color: 'var(--text-muted)',
                fontSize: '0.8rem',
                fontWeight: 600,
                textTransform: 'uppercase',
                marginBottom: '0.5rem',
              }}
            >
              <span>SSH Protection</span>
              <Shield size={16} color="#34d399" />
            </div>
            <div style={{ fontSize: '1.15rem', fontWeight: 700, color: '#34d399' }}>
              Anti-Lockout Active
            </div>
            <div
              style={{
                fontSize: '0.8rem',
                color: 'var(--text-muted)',
                marginTop: '0.25rem',
                lineHeight: 1.4,
              }}
            >
              Port 22 &amp; Web UI automatically bypass tun0 to ensure server access is never lost.
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
