import React, { useEffect, useState } from 'react';
import { RefreshCw, Terminal } from 'lucide-react';
import { api } from '../services/api';

export default function LogsPage() {
  const [logs, setLogs] = useState([]);
  const [loading, setLoading] = useState(false);

  const fetchLogs = async () => {
    setLoading(true);
    try {
      const data = await api.getLogs();
      setLogs(data || []);
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchLogs();
    const interval = setInterval(fetchLogs, 3000);
    return () => clearInterval(interval);
  }, []);

  const getLevelColor = (lvl) => {
    switch (lvl?.toLowerCase()) {
      case 'error': return '#fb7185';
      case 'warn': return '#fbbf24';
      default: return '#38bdf8';
    }
  };

  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1.5rem' }}>
        <div>
          <h2 style={{ fontSize: '1.5rem', fontWeight: 700 }}>System & Core Logs</h2>
          <p style={{ color: 'var(--text-muted)', fontSize: '0.875rem', marginTop: '0.25rem' }}>
            Live console output from Xray, tun2socks, and V2Raynix daemon.
          </p>
        </div>

        <button className="btn btn-secondary" onClick={fetchLogs} disabled={loading}>
          <RefreshCw size={16} className={loading ? 'animate-spin' : ''} />
          Refresh
        </button>
      </div>

      <div style={{
        background: '#070b13',
        border: '1px solid var(--border-color)',
        borderRadius: 'var(--radius-lg)',
        padding: '1.25rem',
        fontFamily: 'var(--font-mono)',
        fontSize: '0.825rem',
        minHeight: 400,
        maxHeight: 550,
        overflowY: 'auto',
        lineHeight: 1.6,
      }}>
        {logs.length === 0 ? (
          <div style={{ color: 'var(--text-muted)', textAlign: 'center', padding: '4rem 0' }}>
            No logs captured yet. System running smoothly.
          </div>
        ) : (
          logs.map((log, idx) => (
            <div key={idx} style={{ display: 'flex', gap: '0.75rem', marginBottom: '0.25rem' }}>
              <span style={{ color: 'var(--text-muted)', flexShrink: 0 }}>
                {log.timestamp?.substring(11, 19) || '00:00:00'}
              </span>
              <span style={{ color: getLevelColor(log.level), fontWeight: 600, textTransform: 'uppercase', width: 55, flexShrink: 0 }}>
                [{log.level}]
              </span>
              <span style={{ color: 'var(--text-primary)', wordBreak: 'break-all' }}>
                {log.message}
              </span>
            </div>
          ))
        )}
      </div>
    </div>
  );
}
