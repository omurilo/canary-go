package game

// Container holds container-specific metadata.
// It is pointed to by Item when the item is a container, saving memory for non-containers.
type Container struct {
	Contents   []*Item
	MaxSize    uint16
	MaxItems   uint16
	Unlocked   bool
	Pagination bool
	Parent     *Item
	Actor      bool
}

// NewContainer creates a new container data instance.
func NewContainer(maxSize uint16) *Container {
	return &Container{
		MaxSize:    maxSize,
		Contents:   make([]*Item, 0),
		Pagination: true,
	}
}

// HoldingCount returns the total number of items held recursively in this
// container (mirrors C++ Container::getItemHoldingCount). It walks the
// Contents tree depth‑first.
func (c *Container) HoldingCount() int {
	if c == nil {
		return 0
	}
	total := 0
	for _, child := range c.Contents {
		if child == nil {
			continue
		}
		total++
		if child.Container != nil {
			total += child.Container.HoldingCount()
		}
	}
	return total
}
