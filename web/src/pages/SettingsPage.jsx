import React, { useState } from 'react';
import { KeyRound, Shield, Check, Info, Cpu, RefreshCw } from 'lucide-react';
import { api } from '../services/api';

export default function SettingsPage({ updateData, onCheckUpdates, onOpenUpdateModal }) {
  const [currPassword, setCurrPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [statusMsg, setStatusMsg] = useState('');
  const [errorMsg, setErrorMsg] = useState('');
  const [updating, setUpdating] = useState(false);

  const handlePasswordChange = async (e) => {
    e.preventDefault();
    if (newPassword !== confirmPassword) {
      setErrorMsg('New passwords do not match');
      return;
    }

    setUpdating(true);
    setErrorMsg('');
    setStatusMsg('');

    try {
      await api.changePassword(currPassword, newPassword);
      setStatusMsg('Password updated successfully!');
      setCurrPassword('');
      setNewPassword('');
      setConfirmPassword('');
    } catch (err) {
      setErrorMsg(err.message || 'Failed to update password');
    } finally {
      setUpdating(false);
    }
  };

  return (
    <div style={{ maxWidth: 640, margin: '0 auto' }}>
      <div style={{ marginBottom: '1.75rem' }}>
        <h2 style={{ fontSize: '1.5rem', fontWeight: 700 }}>Settings</h2>
        <p style={{ color: 'var(--text-muted)', fontSize: '0.875rem', marginTop: '0.25rem' }}>
          Manage administrator security and system preferences.
        </p>
      </div>

      {/* Security Card */}
      <div className="glass-card" style={{ marginBottom: '1.5rem' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.65rem', marginBottom: '1.25rem' }}>
          <KeyRound size={20} color="#38bdf8" />
          <h3 style={{ fontSize: '1.1rem', fontWeight: 600 }}>Change Administrator Password</h3>
        </div>

        <form onSubmit={handlePasswordChange}>
          <div style={{ marginBottom: '1rem' }}>
            <label style={{ display: 'block', fontSize: '0.8rem', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '0.4rem' }}>
              Current Password
            </label>
            <input 
              type="password" 
              className="input-field" 
              value={currPassword}
              onChange={(e) => setCurrPassword(e.target.value)}
              required
            />
          </div>

          <div style={{ marginBottom: '1rem' }}>
            <label style={{ display: 'block', fontSize: '0.8rem', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '0.4rem' }}>
              New Password
            </label>
            <input 
              type="password" 
              className="input-field" 
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              required
            />
          </div>

          <div style={{ marginBottom: '1.5rem' }}>
            <label style={{ display: 'block', fontSize: '0.8rem', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '0.4rem' }}>
              Confirm New Password
            </label>
            <input 
              type="password" 
              className="input-field" 
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              required
            />
          </div>

          {statusMsg && (
            <div style={{ background: 'rgba(16, 185, 129, 0.12)', border: '1px solid rgba(16, 185, 129, 0.3)', color: '#34d399', padding: '0.65rem 0.85rem', borderRadius: 'var(--radius-md)', fontSize: '0.85rem', marginBottom: '1.25rem' }}>
              {statusMsg}
            </div>
          )}

          {errorMsg && (
            <div style={{ background: 'rgba(244, 63, 94, 0.12)', border: '1px solid rgba(244, 63, 94, 0.3)', color: '#fb7185', padding: '0.65rem 0.85rem', borderRadius: 'var(--radius-md)', fontSize: '0.85rem', marginBottom: '1.25rem' }}>
              {errorMsg}
            </div>
          )}

          <button type="submit" className="btn btn-primary" disabled={updating}>
            {updating ? 'Saving...' : 'Update Password'}
          </button>
        </form>
      </div>

      {/* Core Engine Versions */}
      <div className="glass-card" style={{ marginBottom: '1.5rem' }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1rem' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.65rem' }}>
            <Cpu size={20} color="#38bdf8" />
            <h3 style={{ fontSize: '1.1rem', fontWeight: 600 }}>Core Engine Versions</h3>
          </div>
          <button
            type="button"
            onClick={onCheckUpdates}
            className="btn btn-secondary"
            style={{ padding: '0.35rem 0.75rem', fontSize: '0.8rem', display: 'flex', alignItems: 'center', gap: '0.4rem' }}
          >
            <RefreshCw size={13} />
            Check Updates
          </button>
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
          {Object.values(updateData?.cores || {}).map((c) => (
            <div
              key={c.name}
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                padding: '0.75rem 1rem',
                background: 'rgba(255, 255, 255, 0.02)',
                border: '1px solid rgba(255, 255, 255, 0.05)',
                borderRadius: 'var(--radius-sm)',
              }}
            >
              <div>
                <span style={{ fontWeight: 600, textTransform: 'capitalize' }}>{c.name}</span>
                <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginLeft: '0.75rem' }}>
                  {c.current_version ? `v${c.current_version}` : 'Not detected'}
                </span>
              </div>
              {c.update_available ? (
                <button
                  type="button"
                  onClick={onOpenUpdateModal}
                  style={{
                    color: '#38bdf8',
                    fontSize: '0.75rem',
                    fontWeight: 600,
                    background: 'rgba(56, 189, 248, 0.1)',
                    border: '1px solid rgba(56, 189, 248, 0.3)',
                    borderRadius: '12px',
                    padding: '0.2rem 0.6rem',
                    cursor: 'pointer',
                  }}
                >
                  v{c.latest_version} available →
                </button>
              ) : (
                <span style={{ fontSize: '0.75rem', color: '#34d399' }}>✓ Up to date</span>
              )}
            </div>
          ))}
        </div>
      </div>

      {/* System Information */}
      <div className="glass-card">
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.65rem', marginBottom: '1rem' }}>
          <Info size={20} color="#94a3b8" />
          <h3 style={{ fontSize: '1.1rem', fontWeight: 600 }}>System Configuration</h3>
        </div>

        <div style={{ fontSize: '0.875rem', lineHeight: 1.8, color: 'var(--text-secondary)' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', borderBottom: '1px solid rgba(255,255,255,0.05)', paddingBottom: '0.4rem' }}>
            <span>Web UI Port:</span>
            <strong style={{ color: 'var(--text-primary)', fontFamily: 'var(--font-mono)' }}>
              {window?.location?.port || '2080'}
            </strong>
          </div>
          <div style={{ display: 'flex', justifyContent: 'space-between', borderBottom: '1px solid rgba(255,255,255,0.05)', padding: '0.4rem 0' }}>
            <span>Local SOCKS5 Port:</span>
            <strong style={{ color: 'var(--text-primary)', fontFamily: 'var(--font-mono)' }}>10808</strong>
          </div>
          <div style={{ display: 'flex', justifyContent: 'space-between', borderBottom: '1px solid rgba(255,255,255,0.05)', padding: '0.4rem 0' }}>
            <span>Safe Mode Timeout:</span>
            <strong style={{ color: 'var(--text-primary)', fontFamily: 'var(--font-mono)' }}>120 seconds</strong>
          </div>
          <div style={{ display: 'flex', justifyContent: 'space-between', paddingTop: '0.4rem' }}>
            <span>Virtual TUN Interface:</span>
            <strong style={{ color: 'var(--text-primary)', fontFamily: 'var(--font-mono)' }}>tun0</strong>
          </div>
        </div>
      </div>
    </div>
  );
}
