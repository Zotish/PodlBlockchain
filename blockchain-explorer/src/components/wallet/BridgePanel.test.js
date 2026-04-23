import { act } from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import BridgePanel from './BridgePanel';
import { fetchJSON } from '../../utils/api';

jest.mock('../../utils/api', () => ({
  fetchJSON: jest.fn(),
}));

jest.mock('ethers', () => ({
  BrowserProvider: jest.fn(),
  Contract: jest.fn(),
  parseUnits: jest.fn((value) => value),
  formatUnits: jest.fn((value) => String(value)),
}));

beforeEach(() => {
  fetchJSON.mockReset();
  fetchJSON.mockImplementation(async (path) => {
    switch (path) {
      case '/bridge/requests?mode=public':
        return [
          {
            id: 'req-public',
            from: '0xabc1230000000000000000000000000000000000',
            to: '0xdef4560000000000000000000000000000000000',
            amount: '100000000',
            status: 'minted',
            lqd_tx_hash: '0xlqd',
            bsc_tx_hash: '0xbsc',
          },
        ];
      case '/bridge/requests?mode=private':
        return [];
      case '/bridge/tokens':
        return [];
      default:
        throw new Error(`unexpected path ${path}`);
    }
  });
});

test('loads public bridge history and switches to private mode', async () => {
  await act(async () => {
    render(<BridgePanel lqdAddress="0xabc1230000000000000000000000000000000000" lqdPrivateKey="secret" />);
    await Promise.resolve();
  });

  await screen.findByText(/minted/i);
  expect(screen.getByText((_, element) => element?.textContent === 'Total Requests: 1')).toBeInTheDocument();
  expect(screen.getByText(/Current mode:/i)).toHaveTextContent('public');
  expect(screen.getByText(/minted/i)).toBeInTheDocument();

  await userEvent.click(screen.getByRole('button', { name: 'Private' }));

  await waitFor(() => {
    expect(fetchJSON).toHaveBeenCalledWith('/bridge/requests?mode=private');
  });
  expect(screen.getByText(/Current mode:/i)).toHaveTextContent('private');
  expect(await screen.findByText(/No bridge requests for this wallet/i)).toBeInTheDocument();
});
