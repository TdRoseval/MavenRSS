package httputil

import (
	"container/list"
	"crypto/tls"
	"log"
	"net/http"
	"sync"
	"time"
)

const maxClientsPerPool = 50

type ClientPool struct {
	mu               sync.RWMutex
	clients          map[string]*pooledClient
	aiClients        map[string]*pooledClient
	userAgentClients map[string]*pooledClient
	defaultConfig    TransportConfig
	aiConfig         TransportConfig
	lastProxyURL     string
	
	clientLRU          *list.List
	aiClientLRU        *list.List
	userAgentClientLRU *list.List
	
	clientKeys          map[string]*list.Element
	aiClientKeys        map[string]*list.Element
	userAgentClientKeys map[string]*list.Element
}

type pooledClient struct {
	client     *http.Client
	transport  *http.Transport
	proxyURL   string
}

type TransportConfig struct {
	MaxIdleConns          int
	MaxIdleConnsPerHost   int
	MaxConnsPerHost       int
	IdleConnTimeout       time.Duration
	ResponseHeaderTimeout time.Duration
	TLSHandshakeTimeout   time.Duration
	ForceAttemptHTTP2     bool
}

var (
	globalPool     *ClientPool
	globalPoolOnce sync.Once
)

func GetClientPool() *ClientPool {
	globalPoolOnce.Do(func() {
		globalPool = &ClientPool{
			clients:          make(map[string]*pooledClient),
			aiClients:        make(map[string]*pooledClient),
			userAgentClients: make(map[string]*pooledClient),
			defaultConfig: TransportConfig{
				MaxIdleConns:          100,
				MaxIdleConnsPerHost:   20,
				MaxConnsPerHost:       50,
				IdleConnTimeout:       90 * time.Second,
				ResponseHeaderTimeout: 30 * time.Second,
				TLSHandshakeTimeout:   15 * time.Second,
				ForceAttemptHTTP2:     true,
			},
			aiConfig: TransportConfig{
				MaxIdleConns:          50,
				MaxIdleConnsPerHost:   20,
				MaxConnsPerHost:       30,
				IdleConnTimeout:       90 * time.Second,
				ResponseHeaderTimeout: 60 * time.Second,
				TLSHandshakeTimeout:   20 * time.Second,
				ForceAttemptHTTP2:     false,
			},
			clientLRU:          list.New(),
			aiClientLRU:        list.New(),
			userAgentClientLRU: list.New(),
			clientKeys:          make(map[string]*list.Element),
			aiClientKeys:        make(map[string]*list.Element),
			userAgentClientKeys: make(map[string]*list.Element),
		}
	})
	return globalPool
}

func (p *ClientPool) GetClient(proxyURL string, timeout time.Duration) *http.Client {
	key := proxyURL

	p.mu.RLock()
	if pc, exists := p.clients[key]; exists {
		p.mu.RUnlock()
		p.mu.Lock()
		if elem, ok := p.clientKeys[key]; ok {
			p.clientLRU.MoveToFront(elem)
		}
		p.mu.Unlock()
		return &http.Client{
			Transport: pc.transport,
			Timeout:   timeout,
		}
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	if pc, exists := p.clients[key]; exists {
		if elem, ok := p.clientKeys[key]; ok {
			p.clientLRU.MoveToFront(elem)
		}
		return &http.Client{
			Transport: pc.transport,
			Timeout:   timeout,
		}
	}

	if len(p.clients) >= maxClientsPerPool {
		p.evictLRUClient()
	}

	transport := p.createTransport(proxyURL, p.defaultConfig)
	pc := &pooledClient{
		transport: transport,
		proxyURL:  proxyURL,
	}
	p.clients[key] = pc
	elem := p.clientLRU.PushFront(key)
	p.clientKeys[key] = elem
	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
}

func (p *ClientPool) GetAIClient(proxyURL string, timeout time.Duration) *http.Client {
	key := proxyURL

	p.mu.RLock()
	if pc, exists := p.aiClients[key]; exists {
		p.mu.RUnlock()
		p.mu.Lock()
		if elem, ok := p.aiClientKeys[key]; ok {
			p.aiClientLRU.MoveToFront(elem)
		}
		p.mu.Unlock()
		return &http.Client{
			Transport: pc.transport,
			Timeout:   timeout,
		}
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	if pc, exists := p.aiClients[key]; exists {
		if elem, ok := p.aiClientKeys[key]; ok {
			p.aiClientLRU.MoveToFront(elem)
		}
		return &http.Client{
			Transport: pc.transport,
			Timeout:   timeout,
		}
	}

	if len(p.aiClients) >= maxClientsPerPool {
		p.evictLRUAIClient()
	}

	transport := p.createTransport(proxyURL, p.aiConfig)
	pc := &pooledClient{
		transport: transport,
		proxyURL:  proxyURL,
	}
	p.aiClients[key] = pc
	elem := p.aiClientLRU.PushFront(key)
	p.aiClientKeys[key] = elem
	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
}

func (p *ClientPool) GetUserAgentClient(proxyURL string, timeout time.Duration, userAgent string) *http.Client {
	key := p.buildUserAgentKey(proxyURL, userAgent)

	p.mu.RLock()
	if pc, exists := p.userAgentClients[key]; exists {
		p.mu.RUnlock()
		p.mu.Lock()
		if elem, ok := p.userAgentClientKeys[key]; ok {
			p.userAgentClientLRU.MoveToFront(elem)
		}
		p.mu.Unlock()
		return &http.Client{
			Transport: &UserAgentTransport{
				Original:  pc.transport,
				userAgent: userAgent,
			},
			Timeout: timeout,
		}
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	if pc, exists := p.userAgentClients[key]; exists {
		if elem, ok := p.userAgentClientKeys[key]; ok {
			p.userAgentClientLRU.MoveToFront(elem)
		}
		return &http.Client{
			Transport: &UserAgentTransport{
				Original:  pc.transport,
				userAgent: userAgent,
			},
			Timeout: timeout,
		}
	}

	if len(p.userAgentClients) >= maxClientsPerPool {
		p.evictLRUUserAgentClient()
	}

	transport := p.createTransport(proxyURL, p.defaultConfig)
	pc := &pooledClient{
		transport: transport,
		proxyURL:  proxyURL,
	}
	client := &http.Client{
		Transport: &UserAgentTransport{
			Original:  transport,
			userAgent: userAgent,
		},
		Timeout: timeout,
	}
	p.userAgentClients[key] = pc
	elem := p.userAgentClientLRU.PushFront(key)
	p.userAgentClientKeys[key] = elem
	return client
}

func (p *ClientPool) evictLRUClient() {
	if p.clientLRU.Len() == 0 {
		return
	}
	oldest := p.clientLRU.Back()
	if oldest == nil {
		return
	}
	key := oldest.Value.(string)
	delete(p.clients, key)
	p.clientLRU.Remove(oldest)
	delete(p.clientKeys, key)
	log.Printf("[ClientPool] Evicted LRU client with key: %s", key)
}

func (p *ClientPool) evictLRUAIClient() {
	if p.aiClientLRU.Len() == 0 {
		return
	}
	oldest := p.aiClientLRU.Back()
	if oldest == nil {
		return
	}
	key := oldest.Value.(string)
	delete(p.aiClients, key)
	p.aiClientLRU.Remove(oldest)
	delete(p.aiClientKeys, key)
	log.Printf("[ClientPool] Evicted LRU AI client with key: %s", key)
}

func (p *ClientPool) evictLRUUserAgentClient() {
	if p.userAgentClientLRU.Len() == 0 {
		return
	}
	oldest := p.userAgentClientLRU.Back()
	if oldest == nil {
		return
	}
	key := oldest.Value.(string)
	delete(p.userAgentClients, key)
	p.userAgentClientLRU.Remove(oldest)
	delete(p.userAgentClientKeys, key)
	log.Printf("[ClientPool] Evicted LRU UserAgent client with key: %s", key)
}

func (p *ClientPool) createTransport(proxyURL string, config TransportConfig) *http.Transport {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: false,
		},
		MaxIdleConns:          config.MaxIdleConns,
		MaxIdleConnsPerHost:   config.MaxIdleConnsPerHost,
		MaxConnsPerHost:       config.MaxConnsPerHost,
		IdleConnTimeout:       config.IdleConnTimeout,
		ResponseHeaderTimeout: config.ResponseHeaderTimeout,
		TLSHandshakeTimeout:   config.TLSHandshakeTimeout,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     config.ForceAttemptHTTP2,
		WriteBufferSize:       64 * 1024,
		ReadBufferSize:        64 * 1024,
	}

	if proxyURL != "" {
		parsedProxy, err := ValidateAndParseProxyURL(proxyURL)
		if err != nil {
			log.Printf("[ClientPool] ERROR: Invalid proxy configuration '%s': %v. Request will proceed WITHOUT proxy. Please check your proxy settings.", proxyURL, err)
		} else if parsedProxy != nil {
			transport.Proxy = http.ProxyURL(parsedProxy)
			log.Printf("[ClientPool] Proxy configured successfully: %s://%s", parsedProxy.Scheme, parsedProxy.Host)
		}
	}

	return transport
}

func (p *ClientPool) buildKey(proxyURL string) string {
	return proxyURL
}

func (p *ClientPool) buildUserAgentKey(proxyURL string, userAgent string) string {
	return proxyURL + "|" + userAgent
}

func (p *ClientPool) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.clients = make(map[string]*pooledClient)
	p.aiClients = make(map[string]*pooledClient)
	p.userAgentClients = make(map[string]*pooledClient)
	p.lastProxyURL = ""
}

func (p *ClientPool) UpdateConfig(defaultConfig, aiConfig TransportConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.defaultConfig = defaultConfig
	p.aiConfig = aiConfig
	p.clients = make(map[string]*pooledClient)
	p.aiClients = make(map[string]*pooledClient)
}

func (p *ClientPool) ClearProxyClients(oldProxyURL string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	newClients := make(map[string]*pooledClient)
	for key, client := range p.clients {
		if !p.keyUsesProxy(key, oldProxyURL) {
			newClients[key] = client
		}
	}
	p.clients = newClients

	newAIClients := make(map[string]*pooledClient)
	for key, client := range p.aiClients {
		if !p.keyUsesProxy(key, oldProxyURL) {
			newAIClients[key] = client
		}
	}
	p.aiClients = newAIClients

	newUserAgentClients := make(map[string]*pooledClient)
	for key, client := range p.userAgentClients {
		if !p.keyUsesProxy(key, oldProxyURL) {
			newUserAgentClients[key] = client
		}
	}
	p.userAgentClients = newUserAgentClients
}

func (p *ClientPool) keyUsesProxy(key, proxyURL string) bool {
	if proxyURL == "" {
		return key != ""
	}
	return key == proxyURL || contains(key, proxyURL+"|")
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func (p *ClientPool) OnProxyChanged(newProxyURL string) {
	p.mu.Lock()
	oldProxyURL := p.lastProxyURL
	p.lastProxyURL = newProxyURL
	p.mu.Unlock()

	if oldProxyURL != newProxyURL {
		p.ClearAllClients()
	}
}

// ClearAllClients clears all clients in the pool regardless of proxy configuration
// This ensures that when proxy settings change, all connections are re-established
func (p *ClientPool) ClearAllClients() {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	oldClientCount := len(p.clients)
	oldAIClientCount := len(p.aiClients)
	oldUserAgentClientCount := len(p.userAgentClients)
	
	p.clients = make(map[string]*pooledClient)
	p.aiClients = make(map[string]*pooledClient)
	p.userAgentClients = make(map[string]*pooledClient)
	
	p.clientLRU.Init()
	p.aiClientLRU.Init()
	p.userAgentClientLRU.Init()
	
	p.clientKeys = make(map[string]*list.Element)
	p.aiClientKeys = make(map[string]*list.Element)
	p.userAgentClientKeys = make(map[string]*list.Element)
	
	log.Printf("[ClientPool] Cleared all clients (regular: %d, AI: %d, UserAgent: %d) due to proxy configuration change", 
		oldClientCount, oldAIClientCount, oldUserAgentClientCount)
}

func GetPooledHTTPClient(proxyURL string, timeout time.Duration) *http.Client {
	return GetClientPool().GetClient(proxyURL, timeout)
}

func GetPooledAIHTTPClient(proxyURL string, timeout time.Duration) *http.Client {
	return GetClientPool().GetAIClient(proxyURL, timeout)
}

func GetPooledUserAgentClient(proxyURL string, timeout time.Duration, userAgent string) *http.Client {
	return GetClientPool().GetUserAgentClient(proxyURL, timeout, userAgent)
}

func RefreshProxyClients(newProxyURL string) {
	GetClientPool().OnProxyChanged(newProxyURL)
}
