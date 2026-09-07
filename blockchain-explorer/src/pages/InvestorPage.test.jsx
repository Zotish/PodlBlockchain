import { render, screen, within, waitFor, cleanup, act } from '@testing-library/react';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import InvestorPage from './InvestorPage';
import { fetchChainJSON } from '../utils/api';

vi.mock('../utils/api', () => ({ fetchChainJSON: vi.fn() }));
afterEach(cleanup);
beforeEach(() => { fetchChainJSON.mockReset(); });
const metric = (name) => screen.getByText(name, { selector: '.explorer-metric-strip article > span' }).closest('article');
function responses(status = {}, readiness = {}) {
  fetchChainJSON.mockImplementation(async (path) => path === '/v2/protocol/status' ? status : path === '/readiness/mainnet' ? readiness : {});
}

test('missing data is not displayed as zero revenue or zero mainnet blockers', async () => {
  responses();
  render(<InvestorPage />);
  await waitFor(() => expect(fetchChainJSON).toHaveBeenCalledTimes(3));
  expect(within(metric('Business revenue')).getByText('—')).toBeInTheDocument();
  expect(within(metric('Mainnet blockers')).getByText('—')).toBeInTheDocument();
  expect(screen.getByText(/launch status cannot be assessed/)).toBeInTheDocument();
});

test('legacy revenue containing slashing is never relabelled as business revenue', async () => {
  responses({ investor_metrics: { realized_protocol_revenue: '99900000000' }, economic_history: [{ date: '2026-01-01', revenue: '99900000000' }] });
  render(<InvestorPage />);
  expect(await screen.findByText('separated revenue unavailable')).toBeInTheDocument();
  expect(screen.queryByText(/999/)).not.toBeInTheDocument();
});

test('separated atomic amounts are converted to LQD without including security recovery', async () => {
  responses({ investor_metrics: { realized_business_revenue: '100000000', slashing_counts_as_revenue: false }, economic_history: [{ date: '2026-01-01', revenue: '100000000', security_recovery: '200000000', allocations: { insurance_reserve: '235000000' } }] }, { checks: [{ name: 'Audit', critical: true, ok: false, message: 'Missing audit' }] });
  render(<InvestorPage />);
  await waitFor(() => expect(within(metric('Business revenue')).getByText('1.0 LQD')).toBeInTheDocument());
  expect(within(metric('Mainnet blockers')).getByText('1')).toBeInTheDocument();
  expect(screen.getByText('2.0 LQD')).toBeInTheDocument();
  expect(screen.getByText('2.35 LQD')).toBeInTheDocument();
  expect(screen.getByText('Missing audit')).toBeInTheDocument();
});

test('failed requests show unavailable evidence rather than a successful readiness result', async () => {
  fetchChainJSON.mockImplementation(async () => { throw new Error('Network unavailable'); });
  await act(async () => { render(<InvestorPage />); });
  expect(screen.getByText('Network unavailable')).toBeInTheDocument();
  expect(within(metric('Mainnet blockers')).getByText('—')).toBeInTheDocument();
});
