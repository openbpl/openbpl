package detect

import "strings"

// Result describes why a domain matched a keyword.
type Result struct {
	Domain   string
	Keyword  string
	Exact    bool // substring hit
	Distance int  // edit distance (0 when Exact is true)
}

// Matcher holds a set of keywords and checks domains against them.
type Matcher struct {
	keywords []string
	maxDist  int
}

// New creates a Matcher. maxDist is the maximum Levenshtein distance that
// counts as a fuzzy match (e.g. 2).
func New(keywords []string, maxDist int) *Matcher {
	lower := make([]string, len(keywords))
	for i, k := range keywords {
		lower[i] = strings.ToLower(k)
	}
	return &Matcher{keywords: lower, maxDist: maxDist}
}

// Check tests every domain against the keyword list and returns all hits.
func (m *Matcher) Check(domains []string) []Result {
	var results []Result
	for _, domain := range domains {
		domain = strings.ToLower(domain)
		for _, kw := range m.keywords {
			if r, ok := m.check(domain, kw); ok {
				results = append(results, r)
			}
		}
	}
	return results
}

func (m *Matcher) check(domain, keyword string) (Result, bool) {
	// Substring match on the full domain (catches "coinbase-login.phishing.xyz").
	if strings.Contains(domain, keyword) {
		return Result{Domain: domain, Keyword: keyword, Exact: true}, true
	}

	// Scale allowed distance with keyword length to avoid short-word noise.
	// len < 5  → no fuzzy matching (too many false positives)
	// len 5-7  → allow 1 edit
	// len 8+   → allow up to maxDist edits
	allowed := m.maxDist
	if len(keyword) < 5 {
		return Result{}, false
	}
	if len(keyword) <= 7 {
		allowed = min(1, m.maxDist)
	}

	// Fuzzy match: split domain into segments on '.' and '-', compare each
	// segment whose length is close to the keyword length.
	for _, seg := range segments(domain) {
		if len(seg) < 4 || abs(len(seg)-len(keyword)) > allowed {
			continue
		}
		if d := levenshtein(seg, keyword); d > 0 && d <= allowed {
			return Result{Domain: domain, Keyword: keyword, Distance: d}, true
		}
	}
	return Result{}, false
}

// segments splits a domain into pieces on '.' and '-'.
func segments(domain string) []string {
	parts := strings.Split(domain, ".")
	var out []string
	for _, p := range parts {
		out = append(out, strings.Split(p, "-")...)
	}
	return out
}

func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := range prev {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, min(prev[j]+1, prev[j-1]+cost))
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
