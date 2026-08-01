package game

const (
	StorageMountsRangeStart = 10002001
	StorageMountsRangeSize  = 10
	StorageCurrentMount     = 10002011
)

func (p *Player) AddMount(mountID uint16) bool {
	if mountID == 0 {
		return false
	}

	if p.Mounts == nil {
		p.Mounts = make(map[uint16]bool)
	}

	p.Mounts[mountID] = true

	tmpMountID := mountID - 1
	storageKey := StorageMountsRangeStart + uint32(tmpMountID/31)

	value := p.GetStorageValue(storageKey)
	if value == -1 {
		value = 0
	}
	value |= (1 << (tmpMountID % 31))
	p.SetStorageValue(storageKey, value)

	return true
}

func (p *Player) RemoveMount(mountID uint16) bool {
	if mountID == 0 {
		return false
	}

	delete(p.Mounts, mountID)

	tmpMountID := mountID - 1
	storageKey := StorageMountsRangeStart + uint32(tmpMountID/31)

	value := p.GetStorageValue(storageKey)
	if value == -1 {
		return true
	}

	value &^= (1 << (tmpMountID % 31))
	p.SetStorageValue(storageKey, value)

	return true
}

func (p *Player) HasMount(mountID uint16) bool {
	if mountID == 0 {
		return false
	}

	if p.Mounts != nil && p.Mounts[mountID] {
		return true
	}

	tmpMountID := mountID - 1
	storageKey := StorageMountsRangeStart + uint32(tmpMountID/31)
	value := p.GetStorageValue(storageKey)
	if value == -1 {
		return false
	}

	return ((1 << (tmpMountID % 31)) & value) != 0
}

func (p *Player) GetCurrentMount() uint16 {
	value := p.GetStorageValue(StorageCurrentMount)
	if value <= 0 {
		return 0
	}
	return uint16(value)
}

func (p *Player) SetCurrentMount(mountID uint16) {
	if mountID == 0 {
		p.SetStorageValue(StorageCurrentMount, -1)
		return
	}
	p.SetStorageValue(StorageCurrentMount, int32(mountID))
}
