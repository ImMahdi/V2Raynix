import React, { useState, useRef, useEffect } from 'react';
import { Radio, Copy, Trash2, Activity, Check, CheckCircle, Zap } from 'lucide-react';

export default function ConfigCard({ config, isActive, onActivate, onDelete, onPing, onTest }) {
  const [copied, setCopied] = useState(false);
  const [testing, setTesting] = useState(false);
  const copyTimeoutRef = useRef(null);

  useEffect(() => {
    return () => {
      if (copyTimeoutRef.current) {
        clearTimeout(copyTimeoutRef.current);
      }
    };
  }, []);

  const copyToClipboard = (e) => {
    e.stopPropagation();
    navigator.clipboard.writeText(config.rawUrl);
    setCopied(true);
    if (copyTimeoutRef.current) {
      clearTimeout(copyTimeoutRef.current);
    }
    copyTimeoutRef.current = setTimeout(() => setCopied(false), 2000);
  };

  const handleTest = async (e) => {
    e.stopPropagation();
    if (testing) return;
    setTesting(true);
    try {
      if (onTest) {
        await onTest(config.id);
      } else if (onPing) {
        await onPing(config.id);
      }
    } finally {
      setTesting(false);
    }
  };

  const getLatencyClass = (lat) => {
    if (lat === -1 || lat === undefined || lat === null) return 'latency-none';
    if (lat < 150) return 'latency-fast';
    if (lat < 350) return 'latency-medium';
    return 'latency-slow';
  };

  const getLatencyText = (lat) => {
    if (lat === -1 || lat === undefined || lat === null) return 'untested';
    return `${lat} ms`;
  };

  return (
    <div 
      className={`glass-card ${isActive ? 'active-border' : ''}`}
      style={{
        position: 'relative',
        display: 'flex',
        flexDirection: 'column',
        justifyContent: 'space-between',
        cursor: 'pointer',
        borderColor: isActive ? 'var(--accent-emerald)' : 'var(--border-color)',
        background: isActive ? 'rgba(16, 185, 129, 0.05)' : 'var(--bg-card)',
      }}
      onClick={() => onActivate(config.id)}
    >
      {/* Top row: Protocol and Active Status */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '0.75rem' }}>
        <span className={`badge badge-${config.protocol}`}>
          {config.protocol}
        </span>

        {isActive ? (
          <span style={{ display: 'flex', alignItems: 'center', gap: '0.35rem', color: '#34d399', fontSize: '0.75rem', fontWeight: 600 }}>
            <CheckCircle size={14} /> ACTIVE
          </span>
        ) : (
          <button 
            className="btn btn-secondary" 
            style={{ padding: '0.25rem 0.65rem', fontSize: '0.75rem' }}
            onClick={(e) => { e.stopPropagation(); onActivate(config.id); }}
          >
            Activate
          </button>
        )}
      </div>

      {/* Config Name & Server */}
      <div style={{ marginBottom: '1rem' }}>
        <div style={{ fontWeight: 600, fontSize: '1rem', color: 'var(--text-primary)', marginBottom: '0.25rem', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
          {config.name}
        </div>
        <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>
          {config.server}:{config.port}
        </div>
      </div>

      {/* Bottom row: Test delay and Action buttons */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', paddingTop: '0.75rem', borderTop: '1px solid rgba(255, 255, 255, 0.05)' }}>
        <div 
          onClick={handleTest}
          style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', cursor: 'pointer', fontSize: '0.8rem' }}
          title="Click to test real delay"
        >
          <Activity size={14} className={testing ? 'animate-spin' : ''} style={{ color: 'var(--text-muted)' }} />
          <span className={getLatencyClass(config.latencyMs)}>
            {getLatencyText(config.latencyMs)}
          </span>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '0.35rem' }}>
          <button 
            className="btn btn-icon" 
            title="Test Real Proxy Delay" 
            onClick={handleTest}
            disabled={testing}
            style={{ color: '#38bdf8' }}
          >
            <Zap size={16} className={testing ? 'animate-spin' : ''} />
          </button>
          <button 
            className="btn btn-icon" 
            title="Copy Share Link" 
            onClick={copyToClipboard}
          >
            {copied ? <Check size={16} color="#34d399" /> : <Copy size={16} />}
          </button>
          <button 
            className="btn btn-icon" 
            title="Delete Config" 
            onClick={(e) => { e.stopPropagation(); onDelete(config.id); }}
            style={{ color: '#fb7185' }}
          >
            <Trash2 size={16} />
          </button>
        </div>
      </div>
    </div>
  );
}
