import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import BridgePage from './BridgePage';
import { fetchJSON } from '../utils/api';

vi.mock('../utils/api', () => ({
  fetchJSON: vi.fn(),
}));

beforeEach(() => {
  fetchJSON.mockReset();
  fetchJSON.mockImplementation(async (path) => {
    switch (path) {
      case '/bridge/tokens':
        return [{ symbol: 'USDT', lqd_token: '0xlqdtoken' }];
      case '/bridge/requests?mode=public':
        return [];
      case '/bridge/requests?address=0xfeed000000000000000000000000000000000000&mode=private':
        return [{ id: 'req-private', from: '0xfeed', to: '0xbeef', amount: '10', status: 'queued', token: 'LQD' }];
      default:
        throw new Error(`unexpected path ${path}`);
    }
  });
});

test('loads public bridge evidence without accepting signing secrets', async () => {
  render(<BridgePage />);

  expect(await screen.findByText(/Signing disabled/i)).toBeInTheDocument();
  expect(screen.queryByLabelText(/^Private Key$/i)).not.toBeInTheDocument();
  expect(document.querySelector('input[type="password"]')).toBeNull();
  await waitFor(() => expect(fetchJSON).toHaveBeenCalledWith('/bridge/tokens'));
  await waitFor(() => expect(fetchJSON).toHaveBeenCalledWith('/bridge/requests?mode=public'));

  await userEvent.click(screen.getByRole('button', { name: 'Private class' }));

  const fromInput = screen.getByLabelText('Public account address');
  fireEvent.change(fromInput, {
    target: { value: '0xfeed000000000000000000000000000000000000' },
  });

  await waitFor(() => {
    expect(fetchJSON).toHaveBeenCalledWith('/bridge/requests?address=0xfeed000000000000000000000000000000000000&mode=private');
  });

  expect(await screen.findByText(/queued/i)).toBeInTheDocument();
});
