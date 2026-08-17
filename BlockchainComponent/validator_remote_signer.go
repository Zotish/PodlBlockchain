package blockchaincomponent

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
)

const remoteSignerMaxBody = 128 << 10

type RemoteValidatorSignerConfig struct {
	URL                   string
	CAFile                string
	ClientCertificateFile string
	ClientKeyFile         string
	ServerName            string
	Timeout               time.Duration
	AllowInsecureLoopback bool
}

type remoteSignerSignRequest struct {
	Domain    string `json:"domain"`
	Message   []byte `json:"message"`
	Slot      string `json:"slot,omitempty"`
	Timestamp int64  `json:"timestamp"`
	Nonce     string `json:"nonce"`
}

type remoteSignerVRFRequest struct {
	Alpha     []byte `json:"alpha"`
	Slot      string `json:"slot"`
	Timestamp int64  `json:"timestamp"`
	Nonce     string `json:"nonce"`
}

type remoteSignerSignResponse struct {
	Address   string `json:"address"`
	Signature string `json:"signature"`
	Error     string `json:"error,omitempty"`
}

type remoteSignerVRFResponse struct {
	Address string             `json:"address"`
	Result  ValidatorVRFResult `json:"result"`
	Error   string             `json:"error,omitempty"`
}

type RemoteValidatorSigner struct {
	baseURL string
	client  *http.Client
	status  ValidatorSignerStatus
}

func loadRemoteSignerTLS(cfg RemoteValidatorSignerConfig) (*tls.Config, error) {
	if cfg.CAFile == "" || cfg.ClientCertificateFile == "" || cfg.ClientKeyFile == "" {
		return nil, fmt.Errorf("remote signer requires CA, client certificate and client key")
	}
	caPEM, err := os.ReadFile(cfg.CAFile)
	if err != nil {
		return nil, err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("remote signer CA file contains no certificates")
	}
	cert, err := tls.LoadX509KeyPair(cfg.ClientCertificateFile, cfg.ClientKeyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: roots, Certificates: []tls.Certificate{cert}, ServerName: cfg.ServerName}, nil
}

func NewRemoteValidatorSigner(ctx context.Context, cfg RemoteValidatorSignerConfig) (*RemoteValidatorSigner, error) {
	parsed, err := url.Parse(strings.TrimSpace(cfg.URL))
	if err != nil || parsed.Host == "" {
		return nil, fmt.Errorf("valid remote signer URL required")
	}
	transport := &http.Transport{DialContext: (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext, ForceAttemptHTTP2: true, MaxIdleConnsPerHost: 4, IdleConnTimeout: 30 * time.Second}
	if parsed.Scheme == "https" {
		transport.TLSClientConfig, err = loadRemoteSignerTLS(cfg)
		if err != nil {
			return nil, err
		}
	} else if !(cfg.AllowInsecureLoopback && parsed.Scheme == "http" && isLoopbackHost(parsed.Hostname())) {
		return nil, fmt.Errorf("remote validator signer requires HTTPS with mTLS")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	client := &http.Client{Transport: transport, Timeout: cfg.Timeout}
	remote := &RemoteValidatorSigner{baseURL: strings.TrimRight(parsed.String(), "/"), client: client}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, remote.baseURL+"/v1/status", nil)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("remote signer status: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remote signer status returned %s", resp.Status)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, remoteSignerMaxBody)).Decode(&remote.status); err != nil || !remote.status.Healthy || !ValidateAddress(remote.status.Address) || remote.status.VRFSuite != ECVRFP256SHA256TAI {
		return nil, fmt.Errorf("remote signer returned an invalid or unhealthy status")
	}
	if remote.status.Backend == "" {
		remote.status.Backend = "remote-mtls"
	} else {
		remote.status.Backend = "remote-mtls/" + remote.status.Backend
	}
	return remote, nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func signerRequestNonce() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(raw)
}

func (s *RemoteValidatorSigner) post(ctx context.Context, path string, input, output interface{}) error {
	raw, err := json.Marshal(input)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("remote signer returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(io.LimitReader(resp.Body, remoteSignerMaxBody)).Decode(output)
}

func (s *RemoteValidatorSigner) Address() string { return strings.ToLower(s.status.Address) }

func (s *RemoteValidatorSigner) SignMessage(ctx context.Context, domain string, message []byte, slot string) (string, error) {
	if err := validateSignerDomainMessage(domain, message); err != nil {
		return "", err
	}
	request := remoteSignerSignRequest{Domain: domain, Message: message, Slot: slot, Timestamp: time.Now().Unix(), Nonce: signerRequestNonce()}
	response := remoteSignerSignResponse{}
	if err := s.post(ctx, "/v1/sign", request, &response); err != nil {
		return "", err
	}
	if response.Error != "" || !strings.EqualFold(response.Address, s.Address()) {
		return "", fmt.Errorf("remote signer rejected request: %s", response.Error)
	}
	signature, err := hex.DecodeString(strings.TrimPrefix(response.Signature, "0x"))
	if err != nil {
		return "", fmt.Errorf("remote signer returned malformed signature")
	}
	address, ok := recoverSignerAddress(accounts.TextHash(message), signature)
	if !ok || !strings.EqualFold(address, s.Address()) {
		return "", fmt.Errorf("remote signer response failed local signature verification")
	}
	return response.Signature, nil
}

func VerifyValidatorVRFResult(validator string, alpha []byte, result ValidatorVRFResult) bool {
	if !ValidateAddress(validator) || result.Suite != ECVRFP256SHA256TAI {
		return false
	}
	publicKey, err1 := decodeFixedHex(result.PublicKey, 33)
	proof, err2 := decodeFixedHex(result.Proof, ecvrfP256ProofLen)
	wantOutput, err3 := decodeFixedHex(result.Output, 32)
	if err1 != nil || err2 != nil || err3 != nil {
		return false
	}
	output, ok := ECVRFP256Verify(publicKey, alpha, proof)
	if !ok || !strings.EqualFold(hex.EncodeToString(output), hex.EncodeToString(wantOutput)) {
		return false
	}
	bindingRaw, err := hex.DecodeString(strings.TrimPrefix(result.KeyBinding, "0x"))
	if err != nil {
		return false
	}
	message := []byte(vrfBindingMessage(validator, result.Suite, result.PublicKey))
	address, ok := recoverSignerAddress(accounts.TextHash(message), bindingRaw)
	return ok && strings.EqualFold(address, validator)
}

func (s *RemoteValidatorSigner) ProveVRF(ctx context.Context, alpha []byte, slot string) (ValidatorVRFResult, error) {
	request := remoteSignerVRFRequest{Alpha: alpha, Slot: slot, Timestamp: time.Now().Unix(), Nonce: signerRequestNonce()}
	response := remoteSignerVRFResponse{}
	if err := s.post(ctx, "/v1/vrf/prove", request, &response); err != nil {
		return ValidatorVRFResult{}, err
	}
	if response.Error != "" || !strings.EqualFold(response.Address, s.Address()) || !VerifyValidatorVRFResult(s.Address(), alpha, response.Result) {
		return ValidatorVRFResult{}, fmt.Errorf("remote VRF response failed local verification: %s", response.Error)
	}
	return response.Result, nil
}

func (s *RemoteValidatorSigner) Status(_ context.Context) ValidatorSignerStatus { return s.status }
func (s *RemoteValidatorSigner) Close() error {
	if s != nil && s.client != nil {
		if transport, ok := s.client.Transport.(*http.Transport); ok {
			transport.CloseIdleConnections()
		}
	}
	return nil
}

type remoteSignerHandler struct {
	signer       ValidatorSigner
	requireMTLS  bool
	mu           sync.Mutex
	seenRequests map[string]int64
}

func NewValidatorSignerHandler(signer ValidatorSigner, requireMTLS bool) http.Handler {
	return &remoteSignerHandler{signer: signer, requireMTLS: requireMTLS, seenRequests: map[string]int64{}}
}

func (h *remoteSignerHandler) authorize(w http.ResponseWriter, r *http.Request) bool {
	if h.signer == nil {
		http.Error(w, "signer unavailable", http.StatusServiceUnavailable)
		return false
	}
	if h.requireMTLS && (r.TLS == nil || len(r.TLS.VerifiedChains) == 0) {
		http.Error(w, "verified client certificate required", http.StatusUnauthorized)
		return false
	}
	return true
}

func (h *remoteSignerHandler) acceptEnvelope(timestamp int64, nonce string) error {
	if strings.TrimSpace(nonce) == "" || timestamp <= 0 || time.Since(time.Unix(timestamp, 0)) > 60*time.Second || time.Until(time.Unix(timestamp, 0)) > 15*time.Second {
		return fmt.Errorf("stale or incomplete signer request")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now().Unix()
	for key, seen := range h.seenRequests {
		if now-seen > 120 {
			delete(h.seenRequests, key)
		}
	}
	if _, exists := h.seenRequests[nonce]; exists {
		return fmt.Errorf("replayed signer request")
	}
	h.seenRequests[nonce] = now
	return nil
}

func writeSignerJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (h *remoteSignerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/v1/status" {
		writeSignerJSON(w, http.StatusOK, h.signer.Status(r.Context()))
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, remoteSignerMaxBody)
	switch r.URL.Path {
	case "/v1/sign":
		request := remoteSignerSignRequest{}
		if json.NewDecoder(r.Body).Decode(&request) != nil || h.acceptEnvelope(request.Timestamp, request.Nonce) != nil {
			http.Error(w, "invalid signer request", http.StatusBadRequest)
			return
		}
		signature, err := h.signer.SignMessage(r.Context(), request.Domain, request.Message, request.Slot)
		if err != nil {
			writeSignerJSON(w, http.StatusConflict, remoteSignerSignResponse{Address: h.signer.Address(), Error: err.Error()})
			return
		}
		writeSignerJSON(w, http.StatusOK, remoteSignerSignResponse{Address: h.signer.Address(), Signature: signature})
	case "/v1/vrf/prove":
		request := remoteSignerVRFRequest{}
		if json.NewDecoder(r.Body).Decode(&request) != nil || strings.TrimSpace(request.Slot) == "" || h.acceptEnvelope(request.Timestamp, request.Nonce) != nil {
			http.Error(w, "invalid VRF request", http.StatusBadRequest)
			return
		}
		result, err := h.signer.ProveVRF(r.Context(), request.Alpha, request.Slot)
		if err != nil {
			writeSignerJSON(w, http.StatusConflict, remoteSignerVRFResponse{Address: h.signer.Address(), Error: err.Error()})
			return
		}
		writeSignerJSON(w, http.StatusOK, remoteSignerVRFResponse{Address: h.signer.Address(), Result: result})
	default:
		http.NotFound(w, r)
	}
}

type ValidatorSignerTLSFiles struct {
	CertificateFile string
	KeyFile         string
	ClientCAFile    string
}

func ValidatorSignerServerTLSConfig(files ValidatorSignerTLSFiles) (*tls.Config, error) {
	caPEM, err := os.ReadFile(files.ClientCAFile)
	if err != nil {
		return nil, err
	}
	clients := x509.NewCertPool()
	if !clients.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("client CA contains no certificates")
	}
	certificate, err := tls.LoadX509KeyPair(files.CertificateFile, files.KeyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate}, ClientCAs: clients, ClientAuth: tls.RequireAndVerifyClientCert}, nil
}
