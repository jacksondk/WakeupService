package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"

	"gopkg.in/yaml.v3"
	"wakeupservice/wol"
)

type Config struct {
	MACAddress       string `yaml:"mac_address"`
	BroadcastAddress string `yaml:"broadcast_address"`
	WoLPort          int    `yaml:"wol_port"`
	ListenAddress    string `yaml:"listen_address"`
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return &cfg, nil
}

var uiTemplate = template.Must(template.New("ui").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Wake Up</title>
  <style>
    body {
      font-family: sans-serif;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      min-height: 100vh;
      margin: 0;
      background: #1a1a2e;
      color: #eee;
    }
    h1 { margin-bottom: 2rem; font-size: 1.6rem; }
    button {
      padding: 1.2rem 3rem;
      font-size: 1.4rem;
      border: none;
      border-radius: 12px;
      background: #e94560;
      color: white;
      cursor: pointer;
      transition: background 0.2s;
    }
    button:hover { background: #c73652; }
    button:disabled { background: #555; cursor: default; }
    #status { margin-top: 1.5rem; font-size: 1rem; min-height: 1.5em; }
  </style>
</head>
<body>
  <h1>&#x1F4BB; Wake Up PC</h1>
  <button id="btn" onclick="wake()">Wake</button>
  <p id="status"></p>
  <script>
    async function wake() {
      const btn = document.getElementById('btn');
      const status = document.getElementById('status');
      btn.disabled = true;
      status.textContent = 'Sending magic packet\u2026';
      try {
        const resp = await fetch('/wake', { method: 'POST' });
        const data = await resp.json();
        if (resp.ok) {
          status.textContent = '\u2705 Magic packet sent!';
        } else {
          status.textContent = '\u274C Error: ' + (data.error || resp.status);
        }
      } catch (e) {
        status.textContent = '\u274C ' + e.message;
      }
      setTimeout(() => { btn.disabled = false; }, 3000);
    }
  </script>
</body>
</html>
`))

func main() {
	cfgPath := "config.yaml"
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}

	cfg, err := loadConfig(cfgPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	broadcastAddr := fmt.Sprintf("%s:%d", cfg.BroadcastAddress, cfg.WoLPort)

	http.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		uiTemplate.Execute(w, nil)
	})

	http.HandleFunc("POST /wake", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := wol.Send(cfg.MACAddress, broadcastAddr); err != nil {
			log.Printf("Wake-on-LAN error: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		log.Printf("Magic packet sent to %s via %s", cfg.MACAddress, broadcastAddr)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	log.Printf("WakeupService listening on %s (target: %s via %s)", cfg.ListenAddress, cfg.MACAddress, broadcastAddr)
	if err := http.ListenAndServe(cfg.ListenAddress, nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
