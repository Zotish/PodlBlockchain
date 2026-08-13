// src/components/wallet/ReceiveSection.jsx
import React, { useState } from 'react';
import { QRCodeSVG } from 'qrcode.react'; // Fixed import

const ReceiveSection = ({ address }) => {
  const [showQR, setShowQR] = useState(true);

  return (
    <div className="receive-section">
      <h3>Receive LQD Coins</h3>
      
      <div className="receive-content">
        <div className="address-display">
          <label>Your Wallet Address:</label>
          <div className="address-box">
            <code>{address}</code>
            <button 
              className="btn-copy"
              onClick={() => navigator.clipboard.writeText(address)}
            >
              Copy Address
            </button>
          </div>
        </div>

        <div className="qr-section">
          <div className="qr-toggle">
            <button 
              className={`tab ${showQR ? 'active' : ''}`}
              onClick={() => setShowQR(true)}
            >
              QR Code
            </button>
            <button 
              className={`tab ${!showQR ? 'active' : ''}`}
              onClick={() => setShowQR(false)}
            >
              Share Options
            </button>
          </div>

          {showQR ? (
            <div className="qr-code-container">
              <QRCodeSVG value={address} size={200} /> {/* Use QRCodeSVG instead of QRCode */}
              <p>Scan this QR code to receive payments</p>
            </div>
          ) : (
            <div className="share-options">
              <h4>Share Your Address</h4>
              <div className="share-buttons">
                <button 
                  className="btn-secondary"
                  onClick={() => {
                    const text = `Send me LQD coins at: ${address}`;
                    navigator.clipboard.writeText(text);
                    alert('Copied to clipboard!');
                  }}
                >
                  Copy Text Message
                </button>
                <button 
                  className="btn-secondary"
                  onClick={() => {
                    if (navigator.share) {
                      navigator.share({
                        title: 'My LiquidityChain Address',
                        text: `Send me LQD coins at: ${address}`,
                        url: window.location.href,
                      });
                    } else {
                      alert('Web Share API not supported in your browser');
                    }
                  }}
                >
                  Share via...
                </button>
              </div>
            </div>
          )}
        </div>

        <div className="receive-instructions">
          <h4>How to Receive Coins</h4>
          <ol>
            <li>Share your wallet address with the sender</li>
            <li>Wait for the transaction to be confirmed (6-12 seconds)</li>
            <li>Check your balance - it will update automatically</li>
            <li>View transaction history for details</li>
          </ol>
        </div>
      </div>
    </div>
  );
};

export default ReceiveSection;