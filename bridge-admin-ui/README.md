# LQD Bridge Admin UI

This folder contains a standalone admin console for bridge registry management.

What it does:
- Unlocks with a local admin password in the browser.
- Uses `LQD_API_KEY` for backend admin requests.
- Lets the operator add, update, enable, disable, and remove bridge chains.
- Lets the operator add, update, and remove bridge token mappings.

How to use:
1. Open `index.html` in a browser or serve the folder with any static server.
2. Enter a local admin password to unlock the UI.
3. Enter the node URL, usually `http://127.0.0.1:6500`.
4. Enter the backend `LQD_API_KEY`.
5. Use the Chain and Token tabs to manage the bridge registry.

Important:
- The local password protects only the UI.
- The backend still requires the API key for admin actions.
- Normal users should not use this console.

Supported backend routes:
- `GET /bridge/families`
- `GET /bridge/chains`
- `GET /bridge/tokens`
- `POST /bridge/chain`
- `POST /bridge/chain/remove`
- `POST /bridge/token`
- `POST /bridge/token/remove`

