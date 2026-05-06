package rule

import "log"

// Engine runs a set of rules against evidence and collects labels.
type Engine struct {
	rules []Rule
}

func NewEngine() *Engine {
	return &Engine{}
}

func (e *Engine) Register(r Rule) {
	e.rules = append(e.rules, r)
}

// Run executes all registered rules against the evidence.
// Errors from individual rules are logged but don't stop other rules.
func (e *Engine) Run(ev Evidence) []Label {
	var labels []Label
	for _, r := range e.rules {
		ls, err := r.Evaluate(ev)
		if err != nil {
			log.Printf("rule %s: %v", r.Name(), err)
			continue
		}
		labels = append(labels, ls...)
	}
	return labels
}
