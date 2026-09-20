import React from 'react';
import { AlertTriangle, CheckCircle2, RotateCcw } from 'lucide-react';

export default function SafeModeModal({ remainingSeconds, onConfirm, onRollback }) {
  if (remainingSeconds <= 0) return null;

  const minutes = Math.floor(remainingSeconds / 60);
  const seconds = remainingSeconds % 60;
  const timeFormatted = `${minutes}:${seconds < 10 ? '0' : ''}${seconds}`;

  return (
    <div className="modal-overlay">
      <div className="modal-card" style={{ textAlign: 'center', border: '1px solid rgba(245, 158, 11, 0.4)' }}>
        <div style={{
          width: 56,
          height: 56,
          borderRadius: '50%',
          background: 'rgba(245, 158, 11, 0.15)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          margin: '0 auto 1.25rem',
          color: '#f59e0b',
        }}>
          <AlertTriangle size={28} />
        </div>

        <h3 style={{ fontSize: '1.25rem', fontWeight: 700, marginBottom: '0.5rem' }}>
          Network Safe Mode Active
        </h3>

        <p style={{ color: 'var(--text-secondary)', fontSize: '0.9rem', marginBottom: '1.5rem', lineHeight: 1.5 }}>
          Network routing and tunnel changes have been applied. Please confirm that your connection is stable. 
          If unconfirmed, the system will automatically revert to protect you from being locked out.
        </p>

        {/* Countdown Box */}
        <div style={{
          background: 'rgba(15, 23, 42, 0.9)',
          border: '1px solid rgba(245, 158, 11, 0.25)',
          borderRadius: 'var(--radius-md)',
          padding: '1rem',
          marginBottom: '1.75rem',
        }}>
          <div style={{ fontSize: '0.75rem', textTransform: 'uppercase', color: 'var(--text-muted)', fontWeight: 600, letterSpacing: '0.05em' }}>
            Auto-Reverting In
          </div>
          <div style={{ fontSize: '2.25rem', fontWeight: 800, color: '#f59e0b', fontFamily: 'var(--font-mono)' }}>
            {timeFormatted}
          </div>
        </div>

        {/* Action Buttons */}
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.75rem' }}>
          <button 
            className="btn btn-danger" 
            onClick={onRollback}
            style={{ padding: '0.75rem 1rem' }}
          >
            <RotateCcw size={16} />
            Revert Now
          </button>
          <button 
            className="btn btn-success" 
            onClick={onConfirm}
            style={{ padding: '0.75rem 1rem' }}
          >
            <CheckCircle2 size={16} />
            Confirm & Keep
          </button>
        </div>
      </div>
    </div>
  );
}
