package nes



func (m *Mapper4) writeRegister(address uint16, value byte) {
	switch {
	case address <= 0x9FFF && address%2 == 0:
		m.writeBankSelect(value)
	case address <= 0x9FFF && address%2 == 1:
		m.writeBankData(value)
	case address <= 0xBFFF && address%2 == 0:
		m.writeMirror(value)
	case address <= 0xBFFF && address%2 == 1:
		m.writeProtect(value)
	case address <= 0xDFFF && address%2 == 0:
		m.writeIRQLatch(value)
	case address <= 0xDFFF && address%2 == 1:
		m.writeIRQReload(value)
	case address <= 0xFFFF && address%2 == 0:
		m.writeIRQDisable(value)
	case address <= 0xFFFF && address%2 == 1:
		m.writeIRQEnable(value)
	}
}

func (m *Mapper4) writeBankSelect(value byte) {
	m.prgMode = (value >> 6) & 1
	m.chrMode = (value >> 7) & 1
	m.register = value & 7
	m.updateOffsets()
}

func (m *Mapper4) writeBankData(value byte) {
	m.registers[m.register] = value
	m.updateOffsets()
}

func (m *Mapper4) writeMirror(value byte) {
	switch value & 1 {
	case 0:
		m.Cartridge.Mirror = MirrorVertical
	case 1:
		m.Cartridge.Mirror = MirrorHorizontal
	}
}

func (m *Mapper4) writeProtect(value byte) {
}

func (m *Mapper4) writeIRQLatch(value byte) {
	m.reload = value
}

func (m *Mapper4) writeIRQReload(value byte) {
	m.counter = 0
}

func (m *Mapper4) writeIRQDisable(value byte) {
	m.irqEnable = false
}

func (m *Mapper4) writeIRQEnable(value byte) {
	m.irqEnable = true
}

func (m *Mapper4) prgBankOffset(index int) int {
	if index >= 0x80 {
		index -= 0x100
	}
	index %= len(m.PRG) / 0x2000
	offset := index * 0x2000
	if offset < 0 {
		offset += len(m.PRG)
	}
	return offset
}

func (m *Mapper4) chrBankOffset(index int) int {
	if index >= 0x80 {
		index -= 0x100
	}
	index %= len(m.CHR) / 0x0400
	offset := index * 0x0400
	if offset < 0 {
		offset += len(m.CHR)
	}
	return offset
}

func (m *Mapper4) updateOffsets() {
	switch m.prgMode {
	case 0:
		m.prgOffsets[0] = m.prgBankOffset(int(m.registers[6]))
		m.prgOffsets[1] = m.prgBankOffset(int(m.registers[7]))
		m.prgOffsets[2] = m.prgBankOffset(-2)
		m.prgOffsets[3] = m.prgBankOffset(-1)
	case 1:
		m.prgOffsets[0] = m.prgBankOffset(-2)
		m.prgOffsets[1] = m.prgBankOffset(int(m.registers[7]))
		m.prgOffsets[2] = m.prgBankOffset(int(m.registers[6]))
		m.prgOffsets[3] = m.prgBankOffset(-1)
	}
	switch m.chrMode {
	case 0:
		m.chrOffsets[0] = m.chrBankOffset(int(m.registers[0] & 0xFE))
		m.chrOffsets[1] = m.chrBankOffset(int(m.registers[0] | 0x01))
		m.chrOffsets[2] = m.chrBankOffset(int(m.registers[1] & 0xFE))
		m.chrOffsets[3] = m.chrBankOffset(int(m.registers[1] | 0x01))
		m.chrOffsets[4] = m.chrBankOffset(int(m.registers[2]))
		m.chrOffsets[5] = m.chrBankOffset(int(m.registers[3]))
		m.chrOffsets[6] = m.chrBankOffset(int(m.registers[4]))
		m.chrOffsets[7] = m.chrBankOffset(int(m.registers[5]))
	case 1:
		m.chrOffsets[0] = m.chrBankOffset(int(m.registers[2]))
		m.chrOffsets[1] = m.chrBankOffset(int(m.registers[3]))
		m.chrOffsets[2] = m.chrBankOffset(int(m.registers[4]))
		m.chrOffsets[3] = m.chrBankOffset(int(m.registers[5]))
		m.chrOffsets[4] = m.chrBankOffset(int(m.registers[0] & 0xFE))
		m.chrOffsets[5] = m.chrBankOffset(int(m.registers[0] | 0x01))
		m.chrOffsets[6] = m.chrBankOffset(int(m.registers[1] & 0xFE))
		m.chrOffsets[7] = m.chrBankOffset(int(m.registers[1] | 0x01))
	}
}
