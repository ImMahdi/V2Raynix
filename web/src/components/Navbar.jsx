import React from 'react';
import { 
  ShieldCheck, 
  LayoutDashboard, 
  Layers, 
  GitFork, 
  Terminal, 
  Settings, 
  LogOut,
  Zap
} from 'lucide-react';

export default function Navbar({ activeTab, onSelectTab, user, onLogout, hasUpdate, onOpenUpdateModal }) {
  const navItems = [
    { id: 'dashboard', label: 'Dashboard', icon: LayoutDashboard },
    { id: 'configs', label: 'Configs', icon: Layers },
    { id: 'routing', label: 'Routing', icon: GitFork },
    { id: 'logs', label: 'Logs', icon: Terminal },
    { id: 'settings', label: 'Settings', icon: Settings },
  ];

  return (
    <header style={{
      background: 'rgba(15, 23, 42, 0.85)',
      backdropFilter: 'blur(12px)',
      borderBottom: '1px solid var(--border-color)',
      position: 'sticky',
      top: 0,
      zIndex: 50,
    }}>
      <div style={{
        maxWidth: 1200,
        margin: '0 auto',
        padding: '0 1.5rem',
        height: 64,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
      }}>
        {/* Brand */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', cursor: 'pointer' }} onClick={() => onSelectTab('dashboard')}>
          <div style={{
            width: 36,
            height: 36,
            borderRadius: 10,
            background: 'linear-gradient(135deg, #0284c7 0%, #0369a1 100%)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            boxShadow: '0 0 15px rgba(56, 189, 248, 0.4)',
          }}>
            <ShieldCheck size={22} color="#ffffff" />
          </div>
          <div>
            <div style={{ fontWeight: 700, fontSize: '1.1rem', letterSpacing: '-0.02em', display: 'flex', alignItems: 'center', gap: '0.35rem' }}>
              V2Raynix
              <span style={{ fontSize: '0.65rem', background: 'rgba(245, 158, 11, 0.15)', color: '#f59e0b', padding: '1px 6px', borderRadius: 4, fontWeight: 700 }}>BETA</span>
            </div>
          </div>
        </div>

        {/* Nav Tabs */}
        <nav style={{ display: 'flex', alignItems: 'center', gap: '0.25rem' }}>
          {navItems.map((item) => {
            const Icon = item.icon;
            const isActive = activeTab === item.id;
            return (
              <button
                key={item.id}
                onClick={() => onSelectTab(item.id)}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '0.45rem',
                  padding: '0.5rem 0.85rem',
                  borderRadius: 'var(--radius-md)',
                  border: 'none',
                  background: isActive ? 'rgba(56, 189, 248, 0.12)' : 'transparent',
                  color: isActive ? '#38bdf8' : 'var(--text-secondary)',
                  fontWeight: isActive ? 600 : 500,
                  fontSize: '0.875rem',
                  cursor: 'pointer',
                  transition: 'all 0.15s ease',
                }}
              >
                <Icon size={16} />
                <span>{item.label}</span>
              </button>
            );
          })}
        </nav>

        {/* User & Logout */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.85rem' }}>
          {hasUpdate && (
            <button
              onClick={onOpenUpdateModal}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '0.35rem',
                padding: '0.3rem 0.65rem',
                borderRadius: '20px',
                background: 'rgba(56, 189, 248, 0.15)',
                border: '1px solid rgba(56, 189, 248, 0.4)',
                color: '#38bdf8',
                fontSize: '0.75rem',
                fontWeight: 600,
                cursor: 'pointer',
                transition: 'all 0.2s',
              }}
              title="New core versions available"
            >
              <Zap size={13} fill="#38bdf8" />
              <span>Update Available</span>
            </button>
          )}

          <div style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>
            <span style={{ color: 'var(--text-muted)' }}>user:</span> <strong style={{ color: 'var(--text-primary)' }}>{user?.username || 'admin'}</strong>
          </div>
          <button 
            className="btn btn-icon" 
            title="Logout" 
            onClick={onLogout}
            style={{ color: '#fb7185' }}
          >
            <LogOut size={18} />
          </button>
        </div>
      </div>
    </header>
  );
}
