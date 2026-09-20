import React, { useState } from 'react';
import { GitFork, Plus, Trash2, Zap, ShieldAlert } from 'lucide-react';

export default function RoutingPage({ rules, onCreateRule, onDeleteRule }) {
  const [target, setTarget] = useState('');
  const [targetType, setTargetType] = useState('domain');
  const [action, setAction] = useState('direct');
  const [adding, setAdding] = useState(false);

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!target.trim()) return;

    setAdding(true);
    try {
      await onCreateRule({
        target: target.trim(),
        targetType,
        action,
      });
      setTarget('');
    } finally {
      setAdding(false);
    }
  };

  const handleAddPreset = async (presetTarget, presetType, presetAction) => {
    await onCreateRule({
      target: presetTarget,
      targetType: presetType,
      action: presetAction,
    });
  };

  return (
    <div>
      {/* Header */}
      <div style={{ marginBottom: '1.75rem' }}>
        <h2 style={{ fontSize: '1.5rem', fontWeight: 700 }}>Custom Routing Rules</h2>
        <p style={{ color: 'var(--text-muted)', fontSize: '0.875rem', marginTop: '0.25rem' }}>
          Define custom policies to bypass, proxy, or block specific domains and IP addresses.
        </p>
      </div>

      {/* Quick Presets */}
      <div className="glass-card" style={{ marginBottom: '1.5rem' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', fontSize: '0.85rem', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '0.75rem' }}>
          <Zap size={16} color="#f59e0b" />
          <span>Quick 1-Click Presets</span>
        </div>
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.5rem' }}>
          <button 
            className="btn btn-secondary"
            style={{ fontSize: '0.8rem', padding: '0.4rem 0.75rem' }}
            onClick={() => handleAddPreset('geosite:ir', 'domain', 'direct')}
          >
            + Bypass Iran Websites (geosite:ir)
          </button>
          <button 
            className="btn btn-secondary"
            style={{ fontSize: '0.8rem', padding: '0.4rem 0.75rem' }}
            onClick={() => handleAddPreset('geoip:ir', 'ip', 'direct')}
          >
            + Bypass Iran IPs (geoip:ir)
          </button>
          <button 
            className="btn btn-secondary"
            style={{ fontSize: '0.8rem', padding: '0.4rem 0.75rem' }}
            onClick={() => handleAddPreset('geosite:category-ads-all', 'domain', 'block')}
          >
            + Block Ad Trackers
          </button>
        </div>
      </div>

      {/* Add Rule Form */}
      <div className="glass-card" style={{ marginBottom: '1.5rem' }}>
        <form onSubmit={handleSubmit} style={{ display: 'flex', flexWrap: 'wrap', gap: '0.75rem', alignItems: 'flex-end' }}>
          <div style={{ flex: '1 1 200px' }}>
            <label style={{ display: 'block', fontSize: '0.75rem', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '0.35rem' }}>
              Target (Domain / IP / CIDR)
            </label>
            <input 
              type="text" 
              className="input-field" 
              placeholder="e.g. google.com, 1.1.1.1, geosite:ir" 
              value={target}
              onChange={(e) => setTarget(e.target.value)}
              required
            />
          </div>

          <div style={{ width: 140 }}>
            <label style={{ display: 'block', fontSize: '0.75rem', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '0.35rem' }}>
              Type
            </label>
            <select 
              className="input-field" 
              value={targetType}
              onChange={(e) => setTargetType(e.target.value)}
            >
              <option value="domain">Domain</option>
              <option value="ip">IP / CIDR</option>
            </select>
          </div>

          <div style={{ width: 140 }}>
            <label style={{ display: 'block', fontSize: '0.75rem', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '0.35rem' }}>
              Action
            </label>
            <select 
              className="input-field" 
              value={action}
              onChange={(e) => setAction(e.target.value)}
            >
              <option value="direct">Direct (Bypass)</option>
              <option value="proxy">Proxy (Tunnel)</option>
              <option value="block">Block (Drop)</option>
            </select>
          </div>

          <button type="submit" className="btn btn-primary" disabled={adding} style={{ height: 38 }}>
            <Plus size={16} /> Add Rule
          </button>
        </form>
      </div>

      {/* Rules Table */}
      <div className="glass-card" style={{ padding: 0, overflow: 'hidden' }}>
        <table style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left', fontSize: '0.875rem' }}>
          <thead>
            <tr style={{ background: 'rgba(255, 255, 255, 0.02)', borderBottom: '1px solid var(--border-color)', color: 'var(--text-muted)', fontSize: '0.75rem', textTransform: 'uppercase' }}>
              <th style={{ padding: '0.85rem 1.25rem' }}>Target</th>
              <th style={{ padding: '0.85rem 1.25rem' }}>Type</th>
              <th style={{ padding: '0.85rem 1.25rem' }}>Behavior</th>
              <th style={{ padding: '0.85rem 1.25rem', textAlign: 'right' }}>Action</th>
            </tr>
          </thead>
          <tbody>
            {rules.length === 0 ? (
              <tr>
                <td colSpan={4} style={{ padding: '2.5rem', textAlign: 'center', color: 'var(--text-muted)' }}>
                  No custom rules defined yet. System defaults to routing un-bypassed traffic through proxy.
                </td>
              </tr>
            ) : (
              rules.map(rule => (
                <tr key={rule.id} style={{ borderBottom: '1px solid rgba(255, 255, 255, 0.04)' }}>
                  <td style={{ padding: '0.85rem 1.25rem', fontWeight: 600, fontFamily: 'var(--font-mono)' }}>
                    {rule.target}
                  </td>
                  <td style={{ padding: '0.85rem 1.25rem', color: 'var(--text-secondary)' }}>
                    {rule.targetType}
                  </td>
                  <td style={{ padding: '0.85rem 1.25rem' }}>
                    <span style={{
                      padding: '0.2rem 0.5rem',
                      borderRadius: 4,
                      fontSize: '0.75rem',
                      fontWeight: 700,
                      textTransform: 'uppercase',
                      background: rule.action === 'direct' ? 'rgba(16, 185, 129, 0.15)' : rule.action === 'proxy' ? 'rgba(56, 189, 248, 0.15)' : 'rgba(244, 63, 94, 0.15)',
                      color: rule.action === 'direct' ? '#34d399' : rule.action === 'proxy' ? '#38bdf8' : '#fb7185',
                    }}>
                      {rule.action}
                    </span>
                  </td>
                  <td style={{ padding: '0.85rem 1.25rem', textAlign: 'right' }}>
                    <button 
                      className="btn btn-icon" 
                      onClick={() => onDeleteRule(rule.id)}
                      style={{ color: '#fb7185' }}
                    >
                      <Trash2 size={16} />
                    </button>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
