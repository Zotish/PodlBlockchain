# VPS + GitHub Actions Deploy

This deploy keeps Netlify for the UI and runs the backend services on one VPS:

- Chain node + validator + bridge relayer
- Wallet server
- Aggregator
- Optional Caddy HTTPS proxy

## 1. Prepare The VPS

Use Ubuntu 22.04 or 24.04.

```bash
sudo apt update
sudo apt install -y ca-certificates curl git ufw
sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo tee /etc/apt/keyrings/docker.asc >/dev/null
sudo chmod a+r /etc/apt/keyrings/docker.asc
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | sudo tee /etc/apt/sources.list.d/docker.list >/dev/null
sudo apt update
sudo apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
sudo usermod -aG docker $USER
```

Log out and log in again after adding Docker permission.

Firewall:

```bash
sudo ufw allow OpenSSH
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw allow 6100/tcp
sudo ufw enable
```

## 2. Create Deploy Folder

```bash
sudo mkdir -p /opt/podl
sudo chown -R $USER:$USER /opt/podl
```

## 3. Add GitHub Secrets

GitHub repo:

`Settings -> Secrets and variables -> Actions -> New repository secret`

Add:

- `VPS_HOST`: your server IP
- `VPS_USER`: usually `root` or your sudo user
- `VPS_SSH_KEY`: private SSH key that can access the VPS
- `VPS_DEPLOY_PATH`: `/opt/podl`

## 4. First Deploy

Push to `main` or run:

`Actions -> Deploy Backend To VPS -> Run workflow`

The first run creates `/opt/podl/.env` and stops. Open it on the VPS:

```bash
nano /opt/podl/.env
```

Fill at least:

```env
VALIDATOR_ADDRESS=0xYourValidatorAddressHere
STAKE_AMOUNT=3000000
MIN_STAKE=100000
MINING_ENABLED=true
LQD_API_KEY=replace-with-long-secret
LQD_ALLOWED_ORIGINS=https://your-explorer.netlify.app,https://your-swap.netlify.app,https://your-bridge-admin.netlify.app
```

Run the workflow again.

## 5. Optional Domains + HTTPS

Point DNS A records to the VPS IP:

- `chain.yourdomain.com`
- `wallet.yourdomain.com`
- `agg.yourdomain.com`

Then update `/opt/podl/.env`:

```env
ENABLE_CADDY=true
CHAIN_DOMAIN=chain.yourdomain.com
WALLET_DOMAIN=wallet.yourdomain.com
AGG_DOMAIN=agg.yourdomain.com
```

Run the workflow again. Caddy will request HTTPS automatically.

## 6. Netlify Env Update

Explorer / DEX / admin UI should point to:

```env
REACT_APP_CHAIN_BASE=https://chain.yourdomain.com
REACT_APP_API_BASE=https://agg.yourdomain.com
REACT_APP_WALLET_BASE=https://wallet.yourdomain.com
```

Use your existing Netlify sites for UI.

## 7. Useful VPS Commands

```bash
cd /opt/podl
docker compose ps
docker compose logs -f chain
docker compose logs -f wallet
docker compose logs -f aggregator
docker compose restart chain
```

Backend data stays in:

```bash
/opt/podl/data
```

Back it up before deleting or rebuilding the server.
