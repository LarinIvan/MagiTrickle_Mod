package trie

import (
	"strings"
	"sync"
)

// TrieNode represents a node in the Trie
type TrieNode struct {
	children map[string]*TrieNode
	// data holds the arbitrary data associated with the domain (e.g., pointer to Group or Rule ID)
	data  interface{}
	isEnd bool
}

// Trie is a thread-safe prefix tree for domain matching
type Trie struct {
	root *TrieNode
	mu   sync.RWMutex
}

// New creates a new Trie
func New() *Trie {
	return &Trie{
		root: &TrieNode{
			children: make(map[string]*TrieNode),
		},
	}
}

// Insert adds a domain to the Trie with associated data.
// Domains are stored in reverse part order: "google.com" -> "com" -> "google"
// This allows efficiently matching "*.google.com" (namespace) rules against "mail.google.com".
func (t *Trie) Insert(domain string, data interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()

	parts := strings.Split(domain, ".")
	node := t.root

	// Insert in reverse order
	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]
		if node.children[part] == nil {
			node.children[part] = &TrieNode{
				children: make(map[string]*TrieNode),
			}
		}
		node = node.children[part]
	}
	node.isEnd = true
	node.data = data
}

// Search looks up a domain in the Trie.
// Returns the data associated with the longest matching suffix (or exact match).
// For example, if "google.com" is in Trie:
// - "google.com" -> returns data, true
// - "mail.google.com" -> returns data, true (because it matches the "google.com" suffix/namespace)
// - "yahoo.com" -> nil, false
func (t *Trie) Search(domain string) (interface{}, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	parts := strings.Split(domain, ".")
	node := t.root

	// Track the last valid match found (for longest matching suffix)
	var lastMatchData interface{}
	var found bool

	// Traverse in reverse order
	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]

		// If the current node marks the end of a registered domain,
		// it means we found a match for a shorter suffix.
		// e.g. for "a.b.c.com", if "c.com" is registered, we'll see isEnd=true at "c"
		if node.isEnd {
			lastMatchData = node.data
			found = true
		}

		nextNode, ok := node.children[part]
		if !ok {
			// If we can't go deeper, checking if the current position was already a valid match
			// (meaning we matched a parent domain)
			return lastMatchData, found
		}
		node = nextNode
	}

	// Check the final node (exact match)
	if node.isEnd {
		return node.data, true
	}

	return lastMatchData, found
}

// Clear removes all elements from the Trie
func (t *Trie) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.root = &TrieNode{
		children: make(map[string]*TrieNode),
	}
}
