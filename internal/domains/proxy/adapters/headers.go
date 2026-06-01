package adapters

type HeaderModifier struct {
	set    map[string]string
	remove []string
}

func NewHeaderModifier(set map[string]string, remove []string) *HeaderModifier {
	return &HeaderModifier{
		set:    set,
		remove: remove,
	}
}

func (m *HeaderModifier) Modify(headers map[string][]string) {
	for _, key := range m.remove {
		delete(headers, key)
	}
	for key, value := range m.set {
		headers[key] = []string{value}
	}
}
