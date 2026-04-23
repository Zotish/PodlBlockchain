import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import BridgePage from './BridgePage';
import { fetchJSON } from '../utils/api';

jest.mock('../utils/api', () => ({
  API_BASE: 'http://127.0.0.1:9000',
  fetchJSON: jest.fn(),
}));

beforeEach(() => {
  fetchJSON.mockReset();
  fetchJSON.mockImplementation(async (path) => {
    switch (path) {
      case '/bridge/tokens':
        return [{ symbol: 'USDT', lqd_token: '0xlqdtoken' }];
      case '/bridge/requests?address=0xfeed000000000000000000000000000000000000&mode=private':
        return [{ id: 'req-private', from: '0xfeed', to: '0xbeef', amount: '10', status: 'queued', token: 'LQD' }];
      default:
        throw new Error(`unexpected path ${path}`);
    }
  });
});

test('loads token mappings and refreshes private-mode requests', async () => {
  render(<BridgePage />);

  expect(await screen.findByText(/Current mode: public/i)).toBeInTheDocument();
  expect(fetchJSON).toHaveBeenCalledWith('/bridge/tokens');

  await userEvent.click(screen.getByRole('button', { name: 'Private' }));
  expect(screen.getByText(/Current mode: private/i)).toBeInTheDocument();

  const fromLabel = screen.getByText('From (LQD address)');
  const fromInput = fromLabel.parentElement.querySelector('input');
  fireEvent.change(fromInput, {
    target: { value: '0xfeed000000000000000000000000000000000000' },
  });

  await waitFor(() => {
    expect(fetchJSON).toHaveBeenCalledWith('/bridge/requests?address=0xfeed000000000000000000000000000000000000&mode=private');
  });

  expect(await screen.findByText(/queued/i)).toBeInTheDocument();
});
