package trie

import (
	"strings"
	"sync"
)

// TrieNode represents a node in the Trie
type TrieNode struct {
	children map[string]*TrieNode
	// data holds the arbitrary data associated with the domain (e.g., pointer to Group or Rule ID)
	data    interface{}
	isEnd   bool
	isExact bool // true = Domain, false = Namespace
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
// exact: if true, this rule will NOT match subdomains (strict domain match).
func (t *Trie) Insert(domain string, data interface{}, exact bool) {
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
	node.isExact = exact
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

	// Traverse in reverse order (query: mail.google.com -> com, google, mail)
	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]

		nextNode, ok := node.children[part]
		if !ok {
			// Cannot go deeper. Return whatever we found so far.
			return lastMatchData, found
		}
		node = nextNode

		// If current node is a registered rule end
		if node.isEnd {
			// If strict match is required (Domain type)
			if node.isExact {
				// We must be at the end of the query string (last part processed).
				// Since we loop backwards, "end of query" means index 0 (first part of domain string).
				if i == 0 {
					return node.data, true
				}
				// If i > 0 (e.g. we matched "google.com" but query is "mail.google.com"),
				// we ignore this match because it's NOT exact.
			} else {
				// Namespace type (suffix match) - always valid match
				lastMatchData = node.data
				found = true
			}
		}
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
