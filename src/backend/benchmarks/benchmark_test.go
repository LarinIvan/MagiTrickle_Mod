package benchmarks

import (
	"fmt"
	"magitrickle/models"
	"magitrickle/utils/intID"
	"magitrickle/utils/trie"
	"testing"
)

// simulateRuleMatch duplicates the logic from dns.go/group.go
// but works purely on []models.Group to avoid Linux dependencies.
func simulateRuleMatch(groups []*models.Group, domainName string) bool {
	for _, group := range groups {
		if !group.Enable {
			continue
		}
		for _, rule := range group.Rules {
			if !rule.IsEnabled() {
				continue
			}
			if rule.IsMatch(domainName) {
				return true
			}
		}
	}
	return false
}

func generateGroups(groupCount, rulePerGroup int) []*models.Group {
	groups := make([]*models.Group, 0, groupCount)
	for i := 0; i < groupCount; i++ {
		rules := make([]*models.Rule, 0, rulePerGroup)
		for j := 0; j < rulePerGroup; j++ {
			rules = append(rules, &models.Rule{
				ID:     intID.RandomID(),
				Name:   fmt.Sprintf("rule-%d-%d", i, j),
				Type:   "domain",
				Rule:   fmt.Sprintf("domain%d-%d.com", i, j),
				Enable: true,
			})
		}

		g := &models.Group{
			ID:     intID.RandomID(),
			Name:   fmt.Sprintf("group-%d", i),
			Enable: true,
			Rules:  rules,
		}
		groups = append(groups, g)
	}
	return groups
}

func BenchmarkRuleMatching_Linear(b *testing.B) {
	// Scenario: 40 Groups, 300 rules each = 12,000 rules total
	groups := generateGroups(40, 300)

	// Test Cases
	worstCaseDomain := "notfound.com"
	bestCaseDomain := "domain0-0.com"      // First group, first rule
	middleCaseDomain := "domain20-150.com" // Middle
	lastCaseDomain := "domain39-299.com"   // Last group, last rule

	b.Run("WorstCase_NotFound", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			simulateRuleMatch(groups, worstCaseDomain)
		}
	})

	b.Run("BestCase_FirstRule", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			simulateRuleMatch(groups, bestCaseDomain)
		}
	})

	b.Run("MiddleCase", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			simulateRuleMatch(groups, middleCaseDomain)
		}
	})

	b.Run("LastCase_LastRule", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			simulateRuleMatch(groups, lastCaseDomain)
		}
	})
}

func BenchmarkRuleMatching_Trie(b *testing.B) {
	// Scenario: 40 Groups, 300 rules each = 12,000 rules total
	groups := generateGroups(40, 300)

	// Initialize Trie
	domainTrie := trie.New()
	for _, group := range groups {
		if !group.Enable {
			continue
		}
		for _, rule := range group.Rules {
			if !rule.IsEnabled() {
				continue
			}
			// Important: Trie currently only benchmarks "domain" type rules efficiency
			if rule.Type == "domain" {
				domainTrie.Insert(rule.Rule, nil)
			}
		}
	}

	// Test Cases
	worstCaseDomain := "notfound.com"
	bestCaseDomain := "domain0-0.com"      // First group, first rule
	middleCaseDomain := "domain20-150.com" // Middle
	lastCaseDomain := "domain39-299.com"   // Last group, last rule

	b.Run("WorstCase_NotFound", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			domainTrie.Search(worstCaseDomain)
		}
	})

	b.Run("BestCase_FirstRule", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			domainTrie.Search(bestCaseDomain)
		}
	})

	b.Run("MiddleCase", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			domainTrie.Search(middleCaseDomain)
		}
	})

	b.Run("LastCase_LastRule", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			domainTrie.Search(lastCaseDomain)
		}
	})
}
