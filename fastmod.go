package lru

func computemod(divisor uint32) uint64 {
	return ^uint64(0)/uint64(divisor) + 1
}

func fastmod(a uint32, m uint64, d uint32) uint32 {
	m *= uint64(a)
	t := (m>>32)*uint64(d) + ((m&0xFFFFFFFF)*uint64(d))>>32
	return uint32(t >> 32)
}
