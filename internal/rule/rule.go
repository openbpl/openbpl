package rule

// Evidence holds all enrichment data collected for a single domain capture.
type Evidence struct {
	Domain     string
	HTML       string
	Screenshot []byte
}

// Label is a verdict produced by a rule.
type Label struct {
	Rule       string
	Name       string
	Confidence float64
	Detail     string
}

// Rule evaluates evidence and returns zero or more labels.
type Rule interface {
	Name() string
	Evaluate(ev Evidence) ([]Label, error)
}
