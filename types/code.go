package types

type BytecodeInfo struct {
	data []byte
}

func NewBytecodeInfo(data []byte) *BytecodeInfo {
	if len(data) <= 32 {
		panic("Invalid length for BytecodeInfo")
	}
	return &BytecodeInfo{data: data}
}

func (info *BytecodeInfo) CodeHashSlice() []byte {
	return info.data[0:32]
}

func (info *BytecodeInfo) BytecodeSlice() []byte {
	return info.data[32:]
}

func (info *BytecodeInfo) Bytes() []byte {
	if info == nil {
		return nil
	}
	return info.data
}
