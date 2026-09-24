import React, { useState } from 'react';
import { Download, RefreshCw, CheckCircle, AlertTriangle, X } from 'lucide-react';
import { api } from '../services/api';

export default function UpdateCoreModal({ isOpen, onClose, updateData, onRefreshUpdates }) {
  const [updatingCore, setUpdatingCore] = useState(null);
  const [logMessage, setLogMessage] = useState('');
  const [error, setError] = useState(null);

  if (!isOpen || !updateData) return null;

  const cores = Object.values(updateData.cores || {});

  const handleUpdate = async (coreName) => {
    setUpdatingCore(coreName);
    setError(null);
    setLogMessage(`Downloading and verifying ${coreName} (direct / mirror)...`);
    try {
      await api.updateCore(coreName);
      setLogMessage(`${coreName} updated successfully! Refreshing status...`);
      if (onRefreshUpdates) {
        await onRefreshUpdates();
      }
    } catch (err) {
      setError(err.message || 'Update failed');
    } finally {
      setUpdatingCore(null);
    }
  };

  return (
    <div style={{
      position: 'fixed',
      inset: 0,
      zIndex: 100,
      background: 'rgba(0, 0, 0, 0.75)',
      backdropFilter: 'blur(8px)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      padding: '1.5rem',
    }}>
      <div className="glass-card" style={{ maxWidth: 520, width: '100%', padding: '2rem', position: 'relative' }}>
        <button
          onClick={onClose}
          style={{
            position: 'absolute',
            top: 18,
            right: 18,
            background: 'none',
            border: 'none',
            color: 'var(--text-muted)',
            cursor: 'pointer',
          }}
        >
          <X size={20} />
        </button>

        <div style={{ display: 'flex', alignItems: 'center', gap: '0.65rem', marginBottom: '0.5rem' }}>
          <Download size={22} color="#38bdf8" />
          <h3 style={{ fontSize: '1.25rem', fontWeight: 700 }}>Core Engine Updates</h3>
        </div>
        <p style={{ color: 'var(--text-muted)', fontSize: '0.85rem', marginBottom: '1.5rem' }}>
          Official Xray-core and tun2socks engine versions and updates.
        </p>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.85rem' }}>
          {cores.map((c) => (
            <div
              key={c.name}
              style={{
                background: 'rgba(255, 255, 255, 0.03)',
                border: '1px solid rgba(255, 255, 255, 0.07)',
                borderRadius: 'var(--radius-md)',
                padding: '1rem',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
              }}
            >
              <div>
                <div style={{ fontWeight: 600, textTransform: 'capitalize', fontSize: '0.95rem' }}>
                  {c.name}
                </div>
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.25rem' }}>
                  Current: <strong style={{ color: 'var(--text-secondary)' }}>{c.current_version || 'Not detected'}</strong>
                  {' → '}
                  Latest: <strong style={{ color: '#38bdf8' }}>{c.latest_version || 'Checking...'}</strong>
                </div>
              </div>

              {c.update_available ? (
                <button
                  disabled={updatingCore !== null}
                  onClick={() => handleUpdate(c.name)}
                  className="btn btn-primary"
                  style={{
                    padding: '0.4rem 0.85rem',
                    fontSize: '0.8rem',
                    display: 'flex',
                    alignItems: 'center',
                    gap: '0.35rem',
                  }}
                >
                  {updatingCore === c.name ? (
                    <>
                      <RefreshCw size={14} className="animate-spin" />
                      Updating...
                    </>
                  ) : (
                    <>
                      <Download size={14} />
                      Update
                    </>
                  )}
                </button>
              ) : (
                <span style={{ fontSize: '0.75rem', color: '#34d399', display: 'flex', alignItems: 'center', gap: '0.3rem' }}>
                  <CheckCircle size={14} /> Up to date
                </span>
              )}
            </div>
          ))}
        </div>

        {logMessage && (
          <div style={{
            marginTop: '1.25rem',
            padding: '0.75rem',
            background: 'rgba(0, 0, 0, 0.4)',
            border: '1px solid rgba(255, 255, 255, 0.1)',
            borderRadius: 'var(--radius-sm)',
            fontSize: '0.75rem',
            fontFamily: 'var(--font-mono)',
            color: 'var(--text-secondary)',
          }}>
            {logMessage}
          </div>
        )}

        {error && (
          <div style={{
            marginTop: '1.25rem',
            padding: '0.75rem',
            background: 'rgba(244, 63, 94, 0.12)',
            border: '1px solid rgba(244, 63, 94, 0.3)',
            borderRadius: 'var(--radius-sm)',
            fontSize: '0.8rem',
            color: '#fb7185',
            display: 'flex',
            alignItems: 'center',
            gap: '0.5rem',
          }}>
            <AlertTriangle size={16} style={{ flexShrink: 0 }} />
            <span>{error}</span>
          </div>
        )}
      </div>
    </div>
  );
}
