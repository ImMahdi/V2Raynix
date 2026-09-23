import React, { useState } from 'react';
import { Plus, Activity, Search, X } from 'lucide-react';
import ConfigCard from '../components/ConfigCard';

export default function ConfigsPage({ configs, activeId, onActivate, onDelete, onPing, onPingAll, onImport, onTestConfig, onTestAll }) {
  const [searchTerm, setSearchTerm] = useState('');
  const [sortBy, setSortBy] = useState('date');
  const [showImportModal, setShowImportModal] = useState(false);
  const [importContent, setImportContent] = useState('');
  const [importName, setImportName] = useState('');
  const [importing, setImporting] = useState(false);
  const [importError, setImportError] = useState('');

  const filteredConfigs = (configs || []).filter(c => 
    (c?.name || '').toLowerCase().includes(searchTerm.toLowerCase()) ||
    (c?.server || '').toLowerCase().includes(searchTerm.toLowerCase()) ||
    (c?.protocol || '').toLowerCase().includes(searchTerm.toLowerCase())
  );

  const sortedConfigs = [...filteredConfigs].sort((a, b) => {
    if (sortBy === 'ping') {
      const pA = (a.latencyMs > 0) ? a.latencyMs : 99999;
      const pB = (b.latencyMs > 0) ? b.latencyMs : 99999;
      if (pA !== pB) return pA - pB;
      return (b.createdAt || '').localeCompare(a.createdAt || '') || (a.id || '').localeCompare(b.id || '');
    }
    if (sortBy === 'name') {
      const comp = (a.name || '').localeCompare(b.name || '');
      if (comp !== 0) return comp;
      return (a.id || '').localeCompare(b.id || '');
    }
    // Default 'date': newest first
    const comp = (b.createdAt || '').localeCompare(a.createdAt || '');
    if (comp !== 0) return comp;
    return (a.id || '').localeCompare(b.id || '');
  });

  const handleImportSubmit = async (e) => {
    e.preventDefault();
    if (!importContent.trim()) return;

    setImporting(true);
    setImportError('');
    try {
      await onImport(importContent, importName);
      setImportContent('');
      setImportName('');
      setShowImportModal(false);
    } catch (err) {
      setImportError(err.message || 'Failed to import configurations');
    } finally {
      setImporting(false);
    }
  };

  return (
    <div>
      {/* Header bar */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: '1rem', marginBottom: '1.5rem' }}>
        <div>
          <h2 style={{ fontSize: '1.5rem', fontWeight: 700 }}>Configurations</h2>
          <p style={{ color: 'var(--text-muted)', fontSize: '0.875rem', marginTop: '0.25rem' }}>
            Store multiple proxy servers and switch with one click.
          </p>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
          <button 
            className="btn btn-secondary"
            onClick={onTestAll || onPingAll}
          >
            <Activity size={16} />
            Test All
          </button>
          <button 
            className="btn btn-primary"
            onClick={() => setShowImportModal(true)}
          >
            <Plus size={16} />
            Import Configs
          </button>
        </div>
      </div>

      {/* Search and Sort Toolbar */}
      <div style={{ display: 'flex', gap: '0.75rem', marginBottom: '1.5rem', flexWrap: 'wrap' }}>
        <div style={{ position: 'relative', flex: 1, minWidth: 260 }}>
          <Search size={18} style={{ position: 'absolute', left: 12, top: 12, color: 'var(--text-muted)' }} />
          <input 
            type="text" 
            placeholder="Search by name, server IP, or protocol..." 
            className="input-field" 
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            style={{ paddingLeft: '2.5rem' }}
          />
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
          <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)', whiteSpace: 'nowrap' }}>Sort by:</span>
          <select 
            className="input-field" 
            value={sortBy} 
            onChange={(e) => setSortBy(e.target.value)}
            style={{ width: 'auto', minWidth: 140, height: 42 }}
          >
            <option value="date">Date Added</option>
            <option value="ping">Lowest Latency</option>
            <option value="name">Name (A-Z)</option>
          </select>
        </div>
      </div>

      {/* Configs Grid */}
      {sortedConfigs.length === 0 ? (
        <div className="glass-card" style={{ textAlign: 'center', padding: '3.5rem 1.5rem' }}>
          <div style={{ color: 'var(--text-muted)', fontSize: '1rem', marginBottom: '1rem' }}>
            {searchTerm ? 'No configurations match your search.' : 'No configurations added yet.'}
          </div>
          <button className="btn btn-primary" onClick={() => setShowImportModal(true)}>
            <Plus size={16} /> Add Your First Config
          </button>
        </div>
      ) : (
        <div className="grid-configs">
          {sortedConfigs.map(cfg => (
            <ConfigCard 
              key={cfg.id}
              config={cfg}
              isActive={cfg.id === activeId}
              onActivate={onActivate}
              onDelete={onDelete}
              onPing={onPing}
              onTest={onTestConfig}
            />
          ))}
        </div>
      )}

      {/* Import Modal */}
      {showImportModal && (
        <div className="modal-overlay">
          <div className="modal-card">
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1.25rem' }}>
              <h3 style={{ fontSize: '1.2rem', fontWeight: 700 }}>Import Configurations</h3>
              <button className="btn btn-icon" onClick={() => setShowImportModal(false)}>
                <X size={20} />
              </button>
            </div>

            <form onSubmit={handleImportSubmit}>
              <div style={{ marginBottom: '1rem' }}>
                <label style={{ display: 'block', fontSize: '0.8rem', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '0.5rem' }}>
                  Paste Share Links or Raw Xray JSON
                </label>
                <textarea
                  className="input-field"
                  placeholder="vless://...&#10;vmess://...&#10;trojan://...&#10;ss://...&#10;or raw { ... } JSON"
                  value={importContent}
                  onChange={(e) => setImportContent(e.target.value)}
                  required
                />
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.35rem' }}>
                  Supports multiple links pasted on separate lines.
                </div>
              </div>

              <div style={{ marginBottom: '1.5rem' }}>
                <label style={{ display: 'block', fontSize: '0.8rem', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '0.5rem' }}>
                  Custom Name (Optional)
                </label>
                <input
                  type="text"
                  className="input-field"
                  placeholder="e.g. My Fast Germany Server"
                  value={importName}
                  onChange={(e) => setImportName(e.target.value)}
                />
              </div>

              {importError && (
                <div style={{ background: 'rgba(244, 63, 94, 0.1)', border: '1px solid rgba(244, 63, 94, 0.3)', color: '#fb7185', padding: '0.65rem 0.85rem', borderRadius: 'var(--radius-md)', fontSize: '0.85rem', marginBottom: '1.25rem' }}>
                  {importError}
                </div>
              )}

              <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.75rem' }}>
                <button type="button" className="btn btn-secondary" onClick={() => setShowImportModal(false)}>
                  Cancel
                </button>
                <button type="submit" className="btn btn-primary" disabled={importing}>
                  {importing ? 'Importing...' : 'Save Configs'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
