package main

import (
    "encoding/json"
    "flag"
    "fmt"
    "io"
    "log"
    "net"
    "os"
    "os/signal"
    "path/filepath"
    "runtime"
    "syscall"
    "time"

    "golang.org/x/net/proxy"
)

type Config struct {
    Proxies []ProxyConfig `json:"proxies"`
}

type ProxyConfig struct {
    ListenAddr string `json:"listen_addr"`
    TargetAddr string `json:"target_addr"`
    Protocol   string `json:"protocol"`
    Timeout    int    `json:"timeout"`
    BufferSize int    `json:"buffer_size"`
    SocksProxy string `json:"socks_proxy"`
}

type ProxyServer struct {
    config   ProxyConfig
    listener net.Listener
    stopChan chan struct{}
    dialer   proxy.Dialer
}

func main() {
    listenPort := flag.String("listen", "", "Local port to listen on (e.g., 1199)")
    targetServer := flag.String("target", "", "Target NNTP server (e.g., news.example.org:119)")
    configPath := flag.String("config", "", "Path to config file (default: auto-search)")
    showHelp := flag.Bool("help", false, "Show usage information")
    
    flag.Usage = func() {
        fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
        fmt.Fprintf(os.Stderr, "Options:\n")
        flag.PrintDefaults()
        fmt.Fprintf(os.Stderr, "\nExamples:\n")
        fmt.Fprintf(os.Stderr, "  %s -listen 1199 -target news.example.org:119\n", os.Args[0])
        fmt.Fprintf(os.Stderr, "  %s -config /path/to/proxy.json\n", os.Args[0])
        fmt.Fprintf(os.Stderr, "  %s (uses default config file)\n", os.Args[0])
        fmt.Fprintf(os.Stderr, "\nThe proxy always routes traffic through Nym Mixnet on 127.0.0.1:1080\n")
        fmt.Fprintf(os.Stderr, "Make sure nym-socks5-client is running on port 1080\n")
    }
    
    flag.Parse()
    
    if *showHelp {
        flag.Usage()
        return
    }
    
    var config *Config
    var err error
    
    if *listenPort != "" && *targetServer != "" {
        config = &Config{
            Proxies: []ProxyConfig{
                {
                    ListenAddr: fmt.Sprintf("127.0.0.1:%s", *listenPort),
                    TargetAddr: *targetServer,
                    Protocol:   "tcp",
                    Timeout:    90,
                    BufferSize: 32768,
                    SocksProxy: "127.0.0.1:1080",
                },
            },
        }
    } else if *listenPort != "" || *targetServer != "" {
        log.Fatal("Both -listen and -target must be provided together")
    } else {
        configFile := *configPath
        if configFile == "" {
            configFile = findConfigFile("nym-nntp-proxy.json")
        }
        if configFile == "" {
            log.Fatal("Could not find nym-nntp-proxy.json in any standard location")
        }
        
        config, err = loadConfig(configFile)
        if err != nil {
            log.Fatalf("Error loading configuration: %v", err)
        }
    }
    
    var proxies []*ProxyServer
    for _, proxyConfig := range config.Proxies {
        proxy, err := NewProxyServer(proxyConfig)
        if err != nil {
            log.Printf("Error creating proxy %s -> %s: %v",
                proxyConfig.ListenAddr, proxyConfig.TargetAddr, err)
            continue
        }
        proxies = append(proxies, proxy)
        
        go func(p *ProxyServer) {
            if err := p.Start(); err != nil {
                log.Printf("Error starting proxy %s -> %s: %v",
                    p.config.ListenAddr, p.config.TargetAddr, err)
            }
        }(proxy)
    }
    
    if len(proxies) == 0 {
        log.Fatal("No proxies could be started")
    }
    
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan
    
    for _, proxy := range proxies {
        proxy.Stop()
    }
}

func findConfigFile(filename string) string {
    searchDirs := []string{
        ".",
        getUserConfigDir(),
        getUserHomeDir(),
        getSystemConfigDir(),
        getExecutableDir(),
    }
    
    seen := make(map[string]bool)
    var uniqueDirs []string
    for _, dir := range searchDirs {
        if dir != "" && !seen[dir] {
            seen[dir] = true
            uniqueDirs = append(uniqueDirs, dir)
        }
    }
    
    for _, dir := range uniqueDirs {
        path := filepath.Join(dir, filename)
        if fileExists(path) {
            return path
        }
    }
    
    defaultDir := getUserConfigDir()
    if defaultDir == "" {
        defaultDir = getUserHomeDir()
    }
    if defaultDir == "" {
        defaultDir = "."
    }
    
    defaultPath := filepath.Join(defaultDir, filename)
    
    if _, err := createDefaultConfig(defaultPath); err != nil {
        defaultPath = filename
        createDefaultConfig(defaultPath)
    }
    
    return defaultPath
}

func fileExists(path string) bool {
    info, err := os.Stat(path)
    if os.IsNotExist(err) {
        return false
    }
    return !info.IsDir()
}

func getUserConfigDir() string {
    switch runtime.GOOS {
    case "linux":
        if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
            return filepath.Join(xdgConfig, "nym-nntp-proxy")
        }
        if home := os.Getenv("HOME"); home != "" {
            return filepath.Join(home, ".config", "nym-nntp-proxy")
        }
        return ""
    case "windows":
        if appData := os.Getenv("APPDATA"); appData != "" {
            return filepath.Join(appData, "nym-nntp-proxy")
        }
        return ""
    case "darwin":
        if home := os.Getenv("HOME"); home != "" {
            return filepath.Join(home, "Library", "Application Support", "nym-nntp-proxy")
        }
        return ""
    }
    return ""
}

func getSystemConfigDir() string {
    switch runtime.GOOS {
    case "windows":
        if programData := os.Getenv("PROGRAMDATA"); programData != "" {
            return filepath.Join(programData, "nym-nntp-proxy")
        }
        return "C:\\ProgramData\\nym-nntp-proxy"
    case "darwin":
        return "/Library/Application Support/nym-nntp-proxy"
    default:
        return "/etc/nym-nntp-proxy"
    }
}

func getUserHomeDir() string {
    if home := os.Getenv("HOME"); home != "" {
        return home
    }
    if runtime.GOOS == "windows" {
        if homeDrive := os.Getenv("HOMEDRIVE"); homeDrive != "" {
            if homePath := os.Getenv("HOMEPATH"); homePath != "" {
                return filepath.Join(homeDrive, homePath)
            }
        }
        if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
            return userProfile
        }
    }
    return ""
}

func getExecutableDir() string {
    exe, err := os.Executable()
    if err != nil {
        return ""
    }
    return filepath.Dir(exe)
}

func NewProxyServer(config ProxyConfig) (*ProxyServer, error) {
    if config.Timeout == 0 {
        config.Timeout = 30
    }
    if config.BufferSize == 0 {
        config.BufferSize = 8192
    }
    if config.Protocol == "" {
        config.Protocol = "tcp"
    }
    
    var dialer proxy.Dialer = &net.Dialer{
        Timeout: time.Duration(config.Timeout) * time.Second,
    }
    
    if config.SocksProxy != "" {
        socksDialer, err := proxy.SOCKS5("tcp", config.SocksProxy, nil, proxy.Direct)
        if err != nil {
            return nil, fmt.Errorf("failed to create SOCKS5 dialer: %v", err)
        }
        dialer = socksDialer
    }
    
    return &ProxyServer{
        config:   config,
        stopChan: make(chan struct{}),
        dialer:   dialer,
    }, nil
}

func (p *ProxyServer) Start() error {
    var err error
    p.listener, err = net.Listen(p.config.Protocol, p.config.ListenAddr)
    if err != nil {
        return fmt.Errorf("cannot listen on %s: %v", p.config.ListenAddr, err)
    }
    
    for {
        select {
        case <-p.stopChan:
            return nil
        default:
            p.listener.(*net.TCPListener).SetDeadline(time.Now().Add(time.Second))
            conn, err := p.listener.Accept()
            if err != nil {
                if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
                    continue
                }
                continue
            }
            go p.handleConnection(conn)
        }
    }
}

func (p *ProxyServer) Stop() {
    close(p.stopChan)
    if p.listener != nil {
        p.listener.Close()
    }
}

func (p *ProxyServer) handleConnection(clientConn net.Conn) {
    defer clientConn.Close()
    
    clientConn.SetDeadline(time.Now().Add(time.Duration(p.config.Timeout) * time.Second))
    
    targetConn, err := p.dialer.Dial("tcp", p.config.TargetAddr)
    if err != nil {
        return
    }
    defer targetConn.Close()
    
    targetConn.SetDeadline(time.Now().Add(time.Duration(p.config.Timeout) * time.Second))
    
    errChan := make(chan error, 2)
    
    go p.copyData(targetConn, clientConn, errChan)
    go p.copyData(clientConn, targetConn, errChan)
    
    <-errChan
}

func (p *ProxyServer) copyData(dst io.Writer, src io.Reader, errChan chan error) {
    buffer := make([]byte, p.config.BufferSize)
    
    for {
        nr, err := src.Read(buffer)
        if err != nil {
            errChan <- err
            return
        }
        
        if nr > 0 {
            _, err := dst.Write(buffer[:nr])
            if err != nil {
                errChan <- err
                return
            }
        }
    }
}

func loadConfig(filename string) (*Config, error) {
    file, err := os.Open(filename)
    if err != nil {
        return nil, err
    }
    defer file.Close()
    
    var config Config
    decoder := json.NewDecoder(file)
    if err := decoder.Decode(&config); err != nil {
        return nil, err
    }
    
    return &config, nil
}

func createDefaultConfig(filename string) (*Config, error) {
    defaultConfig := Config{
        Proxies: []ProxyConfig{
            {
                ListenAddr: "127.0.0.1:1199",
                TargetAddr: "news.example.com:119",
                Protocol:   "tcp",
                Timeout:    90,
                BufferSize: 32768,
                SocksProxy: "127.0.0.1:1080",
            },
        },
    }
    
    dir := filepath.Dir(filename)
    if dir != "." && dir != "" {
        if err := os.MkdirAll(dir, 0755); err != nil {
            return nil, fmt.Errorf("cannot create directory %s: %v", dir, err)
        }
    }
    
    file, err := os.Create(filename)
    if err != nil {
        return nil, err
    }
    defer file.Close()
    
    encoder := json.NewEncoder(file)
    encoder.SetIndent("", "    ")
    if err := encoder.Encode(defaultConfig); err != nil {
        return nil, err
    }
    
    return &defaultConfig, nil
}
