import React, { useState, useEffect } from 'react';
import { AlertTriangle, CheckCircle2, RotateCcw } from 'lucide-react';

export default function SafeModeModal({ remainingSeconds, onConfirm, onRollback }) {
  const [localRemaining, setLocalRemaining] = useState(remainingSeconds || 0);
  const [submitting, setSubmitting] = useState(false);

  // Sync with server state
  useEffect(() => {
    setLocalRemaining(remainingSeconds || 0);
  }, [remainingSeconds]);

  // Smooth 1-second interpolated countdown (WEB-07)
  useEffect(() => {
    if (localRemaining <= 0) return;
    const interval = setInterval(() => {
      setLocalRemaining(prev => Math.max(0, prev - 1));
    }, 1000);
    return () => clearInterval(interval);
  }, [localRemaining > 0]);

  if (localRemaining <= 0) return null;

  const minutes = Math.floor(localRemaining / 60);
  const seconds = localRemaining % 60;
  const timeFormatted = `${minutes}:${seconds < 10 ? '0' : ''}${seconds}`;

  const handleConfirmClick = async () => {
    if (submitting) return;
    setSubmitting(true);
    try {
      await onConfirm();
    } finally {
      setSubmitting(false);
    }
  };

  const handleRollbackClick = async () => {
    if (submitting) return;
    setSubmitting(true);
    try {
      await onRollback();
    } finally {
      setSubmitting(false);
    }
  };

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

        {/* Action Buttons with Submitting Protection (WEB-07) */}
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.75rem' }}>
          <button 
            className="btn btn-danger" 
            onClick={handleRollbackClick}
            disabled={submitting}
            style={{ padding: '0.75rem 1rem' }}
          >
            <RotateCcw size={16} className={submitting ? 'animate-spin' : ''} />
            {submitting ? 'Reverting...' : 'Revert Now'}
          </button>
          <button 
            className="btn btn-success" 
            onClick={handleConfirmClick}
            disabled={submitting}
            style={{ padding: '0.75rem 1rem' }}
          >
            <CheckCircle2 size={16} />
            {submitting ? 'Confirming...' : 'Confirm & Keep'}
          </button>
        </div>
      </div>
    </div>
  );
}
