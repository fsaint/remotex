package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/fsaint/remotex/internal/config"
	"github.com/fsaint/remotex/internal/daemon"
	"github.com/fsaint/remotex/internal/session"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v\nRun 'remotex setup' first.", err)
	}

	sessionsPath := config.Dir() + "/sessions.json"
	mgr := session.NewManager(sessionsPath)
	if err := mgr.Load(); err != nil {
		log.Printf("warn: load sessions: %v", err)
	}
	mgr.PruneDeadPIDs()

	// REMOTEX_BIND_ADDR overrides interface detection (used in tests).
	tailscaleAddr := os.Getenv("REMOTEX_BIND_ADDR")
	if tailscaleAddr == "" {
		var err error
		tailscaleAddr, err = resolveTailscaleAddr()
		if err != nil {
			log.Printf("warn: tailscale interface not found, binding to all interfaces: %v", err)
			tailscaleAddr = "0.0.0.0"
		}
	}

	srv := daemon.NewServer(mgr, cfg.APIKey, tailscaleAddr, cfg.TailscaleHost, cfg.DaemonPort)

	w := session.NewWatchdog(mgr, 30*time.Second)
	go w.Run()
	defer w.Stop()

	log.Printf("remotex-daemon listening on %s:%d", tailscaleAddr, cfg.DaemonPort)
	if err := srv.Start(); err != nil {
		log.Fatalf("server: %v", err)
	}
}

// resolveTailscaleAddr finds the Tailscale IP (100.x.x.x CGNAT range) on a utun interface.
func resolveTailscaleAddr() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if !strings.HasPrefix(iface.Name, "utun") {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip4 := ipnet.IP.To4()
			if ip4 != nil && ip4[0] == 100 {
				return ip4.String(), nil
			}
		}
	}
	return "", fmt.Errorf("no Tailscale interface found (no utun with 100.x.x.x address)")
}
