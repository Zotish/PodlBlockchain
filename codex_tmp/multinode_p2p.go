package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

type nodeSpec struct {
	name      string
	httpPort  int
	p2pPort   int
	validator string
	remote    string
	dbPath    string
}

type nodeProcess struct {
	spec    nodeSpec
	cmd     *exec.Cmd
	logPath string
}

func main() {
	root, err := os.Getwd()
	if err != nil {
		fatalf("cwd: %v", err)
	}
	tempRoot, err := os.MkdirTemp("", "lqd-multinode-*")
	if err != nil {
		fatalf("temp root: %v", err)
	}
	defer os.RemoveAll(tempRoot)

	nodes := []nodeSpec{
		{name: "node-a", httpPort: 6600, p2pPort: 7600, validator: "0x1111111111111111111111111111111111111111", dbPath: filepath.Join(tempRoot, "node-a", "evodb")},
		{name: "node-b", httpPort: 6601, p2pPort: 7601, validator: "0x2222222222222222222222222222222222222222", remote: "127.0.0.1:7600", dbPath: filepath.Join(tempRoot, "node-b", "evodb")},
		{name: "node-c", httpPort: 6602, p2pPort: 7602, validator: "0x3333333333333333333333333333333333333333", remote: "127.0.0.1:7600", dbPath: filepath.Join(tempRoot, "node-c", "evodb")},
	}

	procs := make([]*nodeProcess, 0, len(nodes))
	defer func() {
		for _, proc := range procs {
			_ = proc.cmd.Process.Kill()
			_, _ = proc.cmd.Process.Wait()
		}
	}()

	for _, spec := range nodes {
		proc, err := startNode(root, tempRoot, spec)
		if err != nil {
			fatalf("start %s: %v", spec.name, err)
		}
		procs = append(procs, proc)
	}

	// Add explicit peer links so all nodes see each other quickly.
	_ = addPeer(nodes[0].httpPort, "127.0.0.1", nodes[1].p2pPort)
	_ = addPeer(nodes[0].httpPort, "127.0.0.1", nodes[2].p2pPort)
	_ = addPeer(nodes[1].httpPort, "127.0.0.1", nodes[2].p2pPort)

	ctx, cancel := context.WithTimeout(context.Background(), 70*time.Second)
	defer cancel()

	if err := waitForCluster(ctx, nodes); err != nil {
		for _, proc := range procs {
			logData, _ := os.ReadFile(proc.logPath)
			fmt.Printf("\n--- %s log (%s) ---\n%s\n", proc.spec.name, proc.logPath, string(logData))
		}
		fatalf("cluster validation failed: %v", err)
	}

	initialHeights, err := captureHeights(nodes)
	if err != nil {
		fatalf("capture heights: %v", err)
	}
	time.Sleep(10 * time.Second)
	finalHeights, err := captureHeights(nodes)
	if err != nil {
		fatalf("capture final heights: %v", err)
	}

	fmt.Println("multi-node validator/P2P summary")
	for _, spec := range nodes {
		peers, _ := getPeers(spec.httpPort)
		validators, _ := getValidators(spec.httpPort)
		fmt.Printf("- %s peers=%d validators=%d height=%d->%d\n", spec.name, len(peers), len(validators), initialHeights[spec.name], finalHeights[spec.name])
	}
	fmt.Println("multinode validator/P2P test: PASS")
}

func startNode(root, tempRoot string, spec nodeSpec) (*nodeProcess, error) {
	if err := os.MkdirAll(filepath.Dir(spec.dbPath), 0o755); err != nil {
		return nil, err
	}
	logPath := filepath.Join(tempRoot, spec.name+".log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, err
	}

	args := []string{
		"run", "main.go", "chain",
		"-port", strconv.Itoa(spec.httpPort),
		"-p2p_port", strconv.Itoa(spec.p2pPort),
		"-db_path", spec.dbPath,
		"-validator", spec.validator,
		"-stake_amount", "3000000",
		"-mining=true",
	}
	if spec.remote != "" {
		args = append(args, "-remote_node", spec.remote)
	}
	cmd := exec.Command("go", args...)
	cmd.Dir = root
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = append(os.Environ(),
		"GOCACHE=/tmp/gocache",
		"GOMODCACHE=/tmp/gomodcache",
		"BSC_TESTNET_RPC=",
		"BSC_TESTNET_PRIVATE_KEY=",
		"BSC_BRIDGE_ADDRESS=",
		"BSC_LOCK_ADDRESS=",
	)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	if err := waitForHTTP(fmt.Sprintf("http://127.0.0.1:%d/getheight", spec.httpPort), 20*time.Second); err != nil {
		_ = cmd.Process.Kill()
		return nil, err
	}
	return &nodeProcess{spec: spec, cmd: cmd, logPath: logPath}, nil
}

func waitForHTTP(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %s", url)
}

func addPeer(httpPort int, address string, port int) error {
	payload, _ := json.Marshal(map[string]any{
		"address": address,
		"port":    port,
	})
	resp, err := http.Post(
		fmt.Sprintf("http://127.0.0.1:%d/peers/add", httpPort),
		"application/json",
		bytes.NewReader(payload),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("peer add status %d", resp.StatusCode)
	}
	return nil
}

func waitForCluster(ctx context.Context, nodes []nodeSpec) error {
	ticker := time.NewTicker(1500 * time.Millisecond)
	defer ticker.Stop()

	for {
		ready := true
		for _, spec := range nodes {
			peers, err := getPeers(spec.httpPort)
			if err != nil {
				ready = false
				break
			}
			validators, err := getValidators(spec.httpPort)
			if err != nil {
				ready = false
				break
			}
			if spec.name == "node-a" && len(peers) < 2 {
				ready = false
			}
			if spec.name != "node-a" && len(peers) < 1 {
				ready = false
			}
			if len(validators) < len(nodes) {
				ready = false
			}
		}
		if ready {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func captureHeights(nodes []nodeSpec) (map[string]int, error) {
	out := make(map[string]int, len(nodes))
	for _, spec := range nodes {
		height, err := getHeight(spec.httpPort)
		if err != nil {
			return nil, err
		}
		out[spec.name] = height
	}
	return out, nil
}

func getHeight(port int) (int, error) {
	var res struct {
		Height int `json:"height"`
	}
	if err := getJSON(fmt.Sprintf("http://127.0.0.1:%d/getheight", port), &res); err != nil {
		return 0, err
	}
	return res.Height, nil
}

func getPeers(port int) ([]map[string]any, error) {
	var peers []map[string]any
	if err := getJSON(fmt.Sprintf("http://127.0.0.1:%d/peers", port), &peers); err != nil {
		return nil, err
	}
	return peers, nil
}

func getValidators(port int) ([]map[string]any, error) {
	var validators []map[string]any
	if err := getJSON(fmt.Sprintf("http://127.0.0.1:%d/validators", port), &validators); err != nil {
		return nil, err
	}
	return validators, nil
}

func getJSON(url string, out any) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
