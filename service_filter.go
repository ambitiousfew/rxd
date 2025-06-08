package rxd

// FilterMode represents the mode of filtering services based on their names.
type FilterMode uint8

// NoFilter is a predefined ServiceFilter that does not filter any services.
var NoFilter = ServiceFilter{Mode: None, Names: map[string]struct{}{}}

const (
	// None represents no filtering mode.
	// In this mode, all services are considered, and no names are filtered out.
	None FilterMode = iota
	// Include represents a filtering mode where only specified service names are included.
	// In this mode, only the services whose names are in the Names set will be included in operations.
	Include
	// Exclude represents a filtering mode where specified service names are excluded.
	// In this mode, all services are considered except those whose names are in the Names set.
	Exclude
)

// ServiceFilter is used to filter services based on their names.
// It can be used to include or exclude services from operations based on their names.
// The Mode field determines whether the names are included or excluded.
// The Names field is a set of service names that are either included or excluded based on the Mode.
// This is useful for operations that need to target specific services or avoid certain services.
type ServiceFilter struct {
	Mode  FilterMode
	Names map[string]struct{}
}

// NewServiceFilter creates a new ServiceFilter with the specified mode and names.
// The names are stored in a set for efficient lookup.
func NewServiceFilter(mode FilterMode, names ...string) ServiceFilter {
	set := make(map[string]struct{}, len(names))
	for _, name := range names {
		set[name] = struct{}{}
	}

	return ServiceFilter{Mode: mode, Names: set}
}
