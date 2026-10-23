package catwalk

import "sync"

var registry = struct {
	sync.RWMutex
	auth   map[InferenceProvider]Authenticator
	lister map[InferenceProvider]Lister
	byType map[Type]Lister
}{
	auth:   map[InferenceProvider]Authenticator{},
	lister: map[InferenceProvider]Lister{},
	byType: map[Type]Lister{},
}

// RegisterAuth makes a the sign-in of the provider it reports. Implementations
// call it from init, so importing a provider package is what enables it.
func RegisterAuth(a Authenticator) {
	registry.Lock()
	defer registry.Unlock()
	registry.auth[a.Provider()] = a
}

// RegisterLister sets the live model lister of a provider.
func RegisterLister(id InferenceProvider, l Lister) {
	registry.Lock()
	defer registry.Unlock()
	registry.lister[id] = l
}

// RegisterTypeLister sets the lister for providers of an endpoint type that
// have no lister of their own.
func RegisterTypeLister(t Type, l Lister) {
	registry.Lock()
	defer registry.Unlock()
	registry.byType[t] = l
}

// AuthFor returns the sign-in of a provider, or nil when it has none.
func AuthFor(id InferenceProvider) Authenticator {
	registry.RLock()
	defer registry.RUnlock()
	return registry.auth[id]
}

// ListerFor returns the live model lister of a provider, or nil when it has
// none. A provider without its own lister uses the one of its endpoint type.
func ListerFor(id InferenceProvider, t Type) Lister {
	registry.RLock()
	defer registry.RUnlock()
	if l, ok := registry.lister[id]; ok {
		return l
	}
	return registry.byType[t]
}

// Auth returns how to sign in to the provider, or nil when it only takes an
// API key.
func (p Provider) Auth() Authenticator {
	return AuthFor(p.ID)
}

// Lister returns the live model lister of the provider, or nil when it has
// none.
func (p Provider) Lister() Lister {
	return ListerFor(p.ID, p.Type)
}
