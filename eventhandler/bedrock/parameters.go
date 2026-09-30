package bedrock

import "github.com/rambow-cloud/powertools-lambda-go/commons"

// Parameters retains insertion order so returning the received parameters preserves
// the reference JSON body. Each invocation receives an independent instance.
type Parameters struct {
	keys   []string
	values map[string]any
}

func (p *Parameters) Get(name string) any  { return p.values[name] }
func (p *Parameters) Has(name string) bool { _, ok := p.values[name]; return ok }

// Set replaces a value without moving its original key position.
func (p *Parameters) Set(name string, value any) {
	if p.values == nil {
		p.values = map[string]any{}
	}
	if _, ok := p.values[name]; !ok {
		p.keys = append(p.keys, name)
	}
	p.values[name] = value
}

func (p *Parameters) Delete(name string) {
	if !p.Has(name) {
		return
	}
	delete(p.values, name)
	for i, key := range p.keys {
		if key == name {
			p.keys = append(p.keys[:i], p.keys[i+1:]...)
			return
		}
	}
}

// Keys follows Object.keys ordering: numeric indices first, then insertion order.
func (p *Parameters) Keys() []string {
	keys := append([]string{}, p.keys...)
	commons.SortObjectKeys(keys)
	return keys
}

// Values returns a shallow map copy. Go maps do not preserve insertion order.
func (p *Parameters) Values() map[string]any {
	values := make(map[string]any, len(p.values))
	for key, value := range p.values {
		values[key] = value
	}
	return values
}

func (p *Parameters) MarshalJSON() ([]byte, error) { return stringify(p) }
