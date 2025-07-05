import React, { useState, useEffect } from 'react';

// BillingCard component for Orzbob Cloud Dashboard
const BillingCard = ({ apiUrl = '/v1/billing', authToken }) => {
  const [billingData, setBillingData] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    fetchBillingData();
    // Refresh every 60 seconds
    const interval = setInterval(fetchBillingData, 60000);
    return () => clearInterval(interval);
  }, []);

  const fetchBillingData = async () => {
    try {
      setLoading(true);
      const response = await fetch(apiUrl, {
        headers: {
          'Authorization': `Bearer ${authToken}`,
          'Content-Type': 'application/json'
        }
      });

      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }

      const data = await response.json();
      setBillingData(data);
      setError(null);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const formatResetDate = (dateStr) => {
    if (!dateStr) return 'N/A';
    const date = new Date(dateStr);
    const now = new Date();
    const days = Math.ceil((date - now) / (1000 * 60 * 60 * 24));
    return `${days} day${days !== 1 ? 's' : ''}`;
  };

  const getUsageBarClass = (percentUsed) => {
    if (percentUsed > 90) return 'usage-fill danger';
    if (percentUsed > 75) return 'usage-fill warning';
    return 'usage-fill';
  };

  if (loading) {
    return (
      <div className="card billing-card">
        <div className="card-header">
          <h2 className="card-title">Billing & Usage</h2>
          <CalculatorIcon />
        </div>
        <div className="loading">Loading billing information...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="card billing-card">
        <div className="card-header">
          <h2 className="card-title">Billing & Usage</h2>
          <CalculatorIcon />
        </div>
        <div className="error">
          Failed to load billing information: {error}
        </div>
      </div>
    );
  }

  const {
    plan = 'Free Tier',
    hours_used = 0,
    hours_included = 10,
    percent_used = 0,
    in_overage = false,
    reset_date,
    estimated_bill = 0,
    daily_usage = '0h 0m',
    throttle_status = 'OK'
  } = billingData || {};

  return (
    <div className="card billing-card">
      <div className="card-header">
        <h2 className="card-title">Billing & Usage</h2>
        <CalculatorIcon />
      </div>
      
      <div className="billing-content">
        <p className="plan-info">
          <strong>Plan:</strong> {plan}
        </p>
        
        <div className="usage-section">
          <div className="usage-header">
            <span>Usage This Month</span>
            <span>{hours_used.toFixed(1)} / {hours_included} hours</span>
          </div>
          <div className="usage-bar">
            <div 
              className={getUsageBarClass(percent_used)}
              style={{ width: `${Math.min(percent_used, 100)}%` }}
            >
              {percent_used}%
            </div>
          </div>
          {in_overage && (
            <p className="overage-warning">
              ⚠️ You are in overage - additional charges apply
            </p>
          )}
        </div>
        
        <div className="billing-stats">
          <div className="stat">
            <div className="stat-value">${estimated_bill.toFixed(2)}</div>
            <div className="stat-label">Estimated Bill</div>
          </div>
          <div className="stat">
            <div className="stat-value">{daily_usage}</div>
            <div className="stat-label">Today's Usage</div>
          </div>
          <div className="stat">
            <div className="stat-value">{formatResetDate(reset_date)}</div>
            <div className="stat-label">Resets In</div>
          </div>
        </div>

        {throttle_status !== 'OK' && (
          <div className="throttle-warning">
            <strong>Throttle Status:</strong> {throttle_status}
          </div>
        )}
        
        <div className="billing-actions">
          <button className="btn btn-primary" onClick={() => window.location.href = '/billing/details'}>
            View Details
          </button>
          <a href="https://orzbob.cloud/billing" className="btn btn-secondary" target="_blank" rel="noopener noreferrer">
            Manage Plan
          </a>
        </div>
      </div>
    </div>
  );
};

// Calculator Icon Component
const CalculatorIcon = () => (
  <svg className="card-icon" width="24" height="24" fill="none" stroke="currentColor" viewBox="0 0 24 24">
    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 7h6m0 10v-3m-3 3h.01M9 17h.01M9 14h.01M12 14h.01M15 11h.01M12 11h.01M9 11h.01M7 21h10a2 2 0 002-2V5a2 2 0 00-2-2H7a2 2 0 00-2 2v14a2 2 0 002 2z" />
  </svg>
);

export default BillingCard;

// CSS Styles (can be moved to a separate CSS file)
const styles = `
.billing-card {
  grid-column: span 2;
}

.card {
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  padding: 1.5rem;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1rem;
}

.card-title {
  font-size: 1.125rem;
  font-weight: 600;
  color: #374151;
}

.card-icon {
  color: #6366f1;
}

.plan-info {
  margin-bottom: 1rem;
}

.usage-section {
  margin: 1.5rem 0;
}

.usage-header {
  display: flex;
  justify-content: space-between;
  margin-bottom: 0.5rem;
  font-size: 0.875rem;
}

.usage-bar {
  background: #f3f4f6;
  height: 24px;
  border-radius: 12px;
  overflow: hidden;
  position: relative;
}

.usage-fill {
  background: #6366f1;
  height: 100%;
  transition: width 0.3s ease;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  padding-right: 0.5rem;
  color: white;
  font-size: 0.875rem;
  font-weight: 500;
}

.usage-fill.warning {
  background: #f59e0b;
}

.usage-fill.danger {
  background: #ef4444;
}

.overage-warning {
  color: #ef4444;
  margin-top: 0.5rem;
  font-size: 0.875rem;
}

.billing-stats {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 1rem;
  margin-top: 1.5rem;
}

.stat {
  text-align: center;
  padding: 1rem;
  background: #f9fafb;
  border-radius: 8px;
}

.stat-value {
  font-size: 1.5rem;
  font-weight: 700;
  color: #111827;
}

.stat-label {
  font-size: 0.875rem;
  color: #6b7280;
  margin-top: 0.25rem;
}

.throttle-warning {
  background: #fef3c7;
  border: 1px solid #fcd34d;
  color: #92400e;
  padding: 0.75rem;
  border-radius: 6px;
  margin: 1rem 0;
  font-size: 0.875rem;
}

.billing-actions {
  display: flex;
  gap: 1rem;
  margin-top: 1.5rem;
}

.btn {
  padding: 0.5rem 1rem;
  border-radius: 6px;
  font-size: 0.875rem;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
  text-decoration: none;
  display: inline-block;
  border: none;
}

.btn-primary {
  background: #6366f1;
  color: white;
}

.btn-primary:hover {
  background: #4f46e5;
}

.btn-secondary {
  background: white;
  color: #374151;
  border: 1px solid #d1d5db;
}

.btn-secondary:hover {
  background: #f9fafb;
}

.loading {
  text-align: center;
  padding: 2rem;
  color: #6b7280;
}

.error {
  background: #fee;
  border: 1px solid #fcc;
  color: #c00;
  padding: 1rem;
  border-radius: 6px;
}
`;`