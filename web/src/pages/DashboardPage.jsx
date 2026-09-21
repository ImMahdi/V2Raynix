import React from 'react';
import { Power, Shield, Activity, Network, Clock, ArrowUpRight, ArrowDownRight } from 'lucide-react';

export default function DashboardPage({ status, configs, onToggleTunnel, onSelectTab, toggling }) {
  const isConnected = status?.state === 'connected';
  const isConnecting = status?.state === 'connecting';
  const activeConfig = configs.find(c => c.id === status?.activeConfigId);

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
    <div>
      {/* Master Toggle Banner */}
      <div className="glass-card master-switch-container" style={{ marginBottom: '2rem' }}>
        <button 
          className={`master-btn ${isConnected ? 'connected' : ''} ${(isConnecting || toggling) ? 'connecting' : ''}`}
          onClick={onToggleTunnel}
          disabled={isConnecting || toggling}
        >
          <Power size={48} strokeWidth={2.5} />
          <span style={{ fontSize: '0.85rem', fontWeight: 700, letterSpacing: '0.05em' }}>
            {toggling ? 'SWITCHING...' : isConnected ? 'CONNECTED' : isConnecting ? 'STARTING...' : 'DISCONNECTED'}
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

      {/* Stats Cards */}
      <div className="grid-stats">
        {/* Active Config Card */}
        <div className="glass-card" style={{ display: 'flex', flexDirection: 'column', justifyContent: 'space-between' }}>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', color: 'var(--text-muted)', fontSize: '0.8rem', fontWeight: 600, textTransform: 'uppercase', marginBottom: '0.5rem' }}>
              <span>Active Config</span>
              <Network size={16} />
            </div>
            <div style={{ fontSize: '1.15rem', fontWeight: 700, color: 'var(--text-primary)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
              {activeConfig ? activeConfig.name : 'None Selected'}
            </div>
            <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginTop: '0.25rem', fontFamily: 'var(--font-mono)' }}>
              {activeConfig ? `${activeConfig.protocol} • ${activeConfig.server}:${activeConfig.port}` : 'Go to Configs tab to select'}
            </div>
          </div>
          <button 
            className="btn btn-secondary" 
            style={{ width: '100%', marginTop: '1rem', padding: '0.4rem 0.75rem', fontSize: '0.8rem' }}
            onClick={() => onSelectTab('configs')}
          >
            Manage Configs
          </button>
        </div>

        {/* Uptime Card */}
        <div className="glass-card">
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', color: 'var(--text-muted)', fontSize: '0.8rem', fontWeight: 600, textTransform: 'uppercase', marginBottom: '0.5rem' }}>
            <span>Tunnel Uptime</span>
            <Clock size={16} />
          </div>
          <div style={{ fontSize: '1.75rem', fontWeight: 800, color: isConnected ? '#34d399' : 'var(--text-muted)' }}>
            {formatUptime(status?.uptimeSeconds)}
          </div>
          <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginTop: '0.25rem' }}>
            {isConnected ? `Interface: ${status?.tunInterface || 'tun0'}` : 'Tunnel offline'}
          </div>
        </div>

        {/* Security & Safe Mode */}
        <div className="glass-card">
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', color: 'var(--text-muted)', fontSize: '0.8rem', fontWeight: 600, textTransform: 'uppercase', marginBottom: '0.5rem' }}>
            <span>SSH Protection</span>
            <Shield size={16} color="#34d399" />
          </div>
          <div style={{ fontSize: '1.15rem', fontWeight: 700, color: '#34d399' }}>
            Anti-Lockout Active
          </div>
          <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginTop: '0.25rem', lineHeight: 1.4 }}>
            Port 22 & Web UI automatically bypass tun0 to ensure server access is never lost.
          </div>
        </div>
      </div>
    </div>
  );
}
