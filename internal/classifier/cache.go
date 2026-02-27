package classifier

import "sync"

// Cache armazena classificações para evitar chamadas repetidas à IA.
type Cache struct {
	mu    sync.RWMutex
	items map[string]*Suggestion
}

// NewCache cria um novo cache de classificações.
func NewCache() *Cache {
	return &Cache{
		items: make(map[string]*Suggestion),
	}
}

// Get retorna uma sugestão do cache, ou nil se não existir.
func (c *Cache) Get(key string) *Suggestion {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.items[key]
}

// Set armazena uma sugestão no cache.
func (c *Cache) Set(key string, suggestion *Suggestion) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = suggestion
}

// CacheKey gera uma chave de cache baseada no nome e tipo do arquivo.
func CacheKey(name, mimeType string) string {
	return name + "|" + mimeType
}
