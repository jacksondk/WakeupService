package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
	"wakeupservice/wol"
)

//go:embed favicon.ico
var faviconICO []byte

// ComputerConfig is one entry under the `computers` key in config.yaml.
// broadcast_address / wol_port fall back to the top-level Config values
// when omitted.
type ComputerConfig struct {
	Name             string `yaml:"name"`
	MACAddress       string `yaml:"mac_address"`
	BroadcastAddress string `yaml:"broadcast_address"`
	WoLPort          int    `yaml:"wol_port"`
}

type Config struct {
	ListenAddress    string           `yaml:"listen_address"`
	BroadcastAddress string           `yaml:"broadcast_address"`
	WoLPort          int              `yaml:"wol_port"`
	Computers        []ComputerConfig `yaml:"computers"`
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

// Computer is a fully-resolved wake target: MAC address plus a ready-to-dial
// "host:port" broadcast address, keyed by a stable URL-safe ID derived from
// its name.
type Computer struct {
	ID               string
	Name             string
	MACAddress       string
	BroadcastAddress string
}

// buildComputers resolves each configured computer's broadcast address/port
// (falling back to the config-level defaults) and assigns each a unique
// slug ID used in the /wake/{id} route and UI.
func buildComputers(cfg *Config) ([]Computer, error) {
	if len(cfg.Computers) == 0 {
		return nil, fmt.Errorf("no computers configured (add entries under 'computers' in config.yaml)")
	}

	seenIDs := make(map[string]bool, len(cfg.Computers))
	computers := make([]Computer, 0, len(cfg.Computers))
	for i, c := range cfg.Computers {
		if c.Name == "" {
			return nil, fmt.Errorf("computer %d: name is required", i+1)
		}
		if c.MACAddress == "" {
			return nil, fmt.Errorf("computer %q: mac_address is required", c.Name)
		}

		broadcastHost := c.BroadcastAddress
		if broadcastHost == "" {
			broadcastHost = cfg.BroadcastAddress
		}
		if broadcastHost == "" {
			return nil, fmt.Errorf("computer %q: broadcast_address is required (set it per-computer or as a top-level default)", c.Name)
		}

		port := c.WoLPort
		if port == 0 {
			port = cfg.WoLPort
		}
		if port == 0 {
			port = 9
		}

		id := uniqueSlug(slugify(c.Name), seenIDs)
		seenIDs[id] = true

		computers = append(computers, Computer{
			ID:               id,
			Name:             c.Name,
			MACAddress:       c.MACAddress,
			BroadcastAddress: net.JoinHostPort(broadcastHost, strconv.Itoa(port)),
		})
	}
	return computers, nil
}

// slugify turns a computer name into a lowercase, hyphen-separated,
// URL-safe identifier, e.g. "Desktop PC" -> "desktop-pc".
func slugify(name string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	slug := strings.TrimRight(b.String(), "-")
	if slug == "" {
		slug = "computer"
	}
	return slug
}

// uniqueSlug appends -2, -3, ... to base until it no longer collides with seen.
func uniqueSlug(base string, seen map[string]bool) string {
	id := base
	for n := 2; seen[id]; n++ {
		id = fmt.Sprintf("%s-%d", base, n)
	}
	return id
}

// uiComputer is the subset of Computer data the HTML template renders.
type uiComputer struct {
	ID   string
	Name string
}

type uiData struct {
	Computers []uiComputer
}

var uiTemplate = template.Must(template.New("ui").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <link rel="icon" type="image/x-icon" href="/favicon.ico">
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
    .computers {
      display: flex;
      flex-direction: column;
      gap: 1.5rem;
      align-items: center;
    }
    .computer {
      display: flex;
      flex-direction: column;
      align-items: center;
    }
    button {
      padding: 1.2rem 3rem;
      font-size: 1.4rem;
      border: none;
      border-radius: 12px;
      background: #e94560;
      color: white;
      cursor: pointer;
      transition: background 0.2s;
      min-width: 14rem;
    }
    button:hover { background: #c73652; }
    button:disabled { background: #555; cursor: default; }
    .status { margin-top: 0.75rem; font-size: 1rem; min-height: 1.5em; }
  </style>
</head>
<body>
  <h1>&#x1F4BB; Wake Up</h1>
  <div class="computers">
    {{range .Computers}}
    <div class="computer">
      <button onclick="wake('{{.ID}}', this)">{{.Name}}</button>
      <p class="status" id="status-{{.ID}}"></p>
    </div>
    {{end}}
  </div>
  <script>
    async function wake(id, btn) {
      const status = document.getElementById('status-' + id);
      btn.disabled = true;
      status.textContent = 'Sending magic packet\u2026';
      try {
        const resp = await fetch('/wake/' + encodeURIComponent(id), { method: 'POST' });
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

// defaultConfigPath returns config.yaml next to the executable. Services
// start with System32 as the working directory, so a relative path is unsafe.
func defaultConfigPath() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), "config.yaml")
	}
	return "config.yaml"
}

// run loads the config, serves HTTP until stop is closed (or the server
// fails), then shuts down gracefully.
func run(cfgPath string, stop <-chan struct{}) error {
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	computers, err := buildComputers(cfg)
	if err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	computerByID := make(map[string]Computer, len(computers))
	data := uiData{Computers: make([]uiComputer, 0, len(computers))}
	for _, c := range computers {
		computerByID[c.ID] = c
		data.Computers = append(data.Computers, uiComputer{ID: c.ID, Name: c.Name})
		log.Printf("Configured computer %q (id=%s, mac=%s, broadcast=%s)", c.Name, c.ID, c.MACAddress, c.BroadcastAddress)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		uiTemplate.Execute(w, data)
	})

	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/x-icon")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Write(faviconICO)
	})

	mux.HandleFunc("POST /wake/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		c, ok := computerByID[r.PathValue("id")]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "unknown computer"})
			return
		}
		if err := wol.Send(c.MACAddress, c.BroadcastAddress); err != nil {
			log.Printf("Wake-on-LAN error for %s: %v", c.Name, err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		log.Printf("Magic packet sent to %s (%s) via %s", c.Name, c.MACAddress, c.BroadcastAddress)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	srv := &http.Server{Addr: cfg.ListenAddress, Handler: mux}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	log.Printf("WakeupService listening on %s (%d computer(s) configured)", cfg.ListenAddress, len(computers))
	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case <-stop:
		log.Printf("Shutting down")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
	}
}

func main() {
	cfgPath := defaultConfigPath()
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}

	// When launched by the Windows SCM this runs the service and returns.
	if handled, err := runAsService(cfgPath); handled {
		if err != nil {
			log.Fatalf("Service failed: %v", err)
		}
		return
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(cfgPath, ctx.Done()); err != nil {
		log.Fatalf("%v", err)
	}
}
