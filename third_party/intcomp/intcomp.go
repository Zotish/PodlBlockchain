package intcomp

func CompressUint32(input []uint32, buffer []uint32) []uint32 {
	buffer = buffer[:0]
	return append(buffer, input...)
}

func CompressUint64(input []uint64, buffer []uint64) []uint64 {
	buffer = buffer[:0]
	return append(buffer, input...)
}

func UncompressUint32(input []uint32, buffer []uint32) []uint32 {
	buffer = buffer[:0]
	return append(buffer, input...)
}

func UncompressUint64(input []uint64, buffer []uint64) []uint64 {
	buffer = buffer[:0]
	return append(buffer, input...)
}
