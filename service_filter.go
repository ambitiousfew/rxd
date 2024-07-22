package rxd

type FilterMode int

var NoFilter = ServiceFilter{Mode: None, Names: map[string]struct{}{}}

const (
	None FilterMode = iota
	Include
	Exclude
)

type ServiceFilter struct {
	Mode  FilterMode
	Names map[string]struct{}
}

func NewServiceFilter(mode FilterMode, names ...string) ServiceFilter {
	set := make(map[string]struct{}, len(names))
	for _, name := range names {
		set[name] = struct{}{}
	}

	return ServiceFilter{Mode: mode, Names: set}
}
