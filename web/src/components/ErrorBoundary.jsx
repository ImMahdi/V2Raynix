import React from 'react';
import { AlertOctagon, RotateCw } from 'lucide-react';

export class ErrorBoundary extends React.Component {
  constructor(props) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error) {
    return { hasError: true, error };
  }

  componentDidCatch(error, errorInfo) {
    console.error('ErrorBoundary caught unhandled exception:', error, errorInfo);
  }

  handleReset = () => {
    this.setState({ hasError: false, error: null });
    if (this.props.onReset) {
      this.props.onReset();
    } else {
      window.location.reload();
    }
  };

  render() {
    if (this.state.hasError) {
      return (
        <div 
          className="glass-card" 
          style={{ 
            maxWidth: 540, 
            margin: '3rem auto', 
            textAlign: 'center', 
            padding: '2.5rem 2rem',
            border: '1px solid rgba(244, 63, 94, 0.4)',
            background: 'rgba(15, 23, 42, 0.95)',
          }}
        >
          <div style={{
            width: 56,
            height: 56,
            borderRadius: '50%',
            background: 'rgba(244, 63, 94, 0.15)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            margin: '0 auto 1.25rem',
            color: '#fb7185',
          }}>
            <AlertOctagon size={28} />
          </div>

          <h3 style={{ fontSize: '1.25rem', fontWeight: 700, marginBottom: '0.5rem', color: 'var(--text-primary)' }}>
            Component Render Error
          </h3>

          <p style={{ color: 'var(--text-muted)', fontSize: '0.875rem', marginBottom: '1.5rem', lineHeight: 1.6 }}>
            {this.state.error?.message || 'An unexpected rendering error occurred in this view.'}
          </p>

          <button 
            className="btn btn-primary" 
            onClick={this.handleReset}
            style={{ display: 'inline-flex', alignItems: 'center', gap: '0.5rem', margin: '0 auto' }}
          >
            <RotateCw size={16} />
            Reload View
          </button>
        </div>
      );
    }

    return this.props.children;
  }
}

export default ErrorBoundary;
