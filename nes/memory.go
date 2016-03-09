package nes

import "log"

func ReadByte(console *Console, address uint16) byte {
	readController := func (c *Controller) byte {
		value := byte(0)
		if c.index < 8 && c.buttons[c.index] {
			value = 1
		}
		c.index++
		if c.strobe&1 == 1 {
			c.index = 0
		}
		return value
	}
	switch {
	case address < 0x2000:
		return console.RAM[address%0x0800]
	case address < 0x4000:
		return console.PPU.readRegister(0x2000 + address%8)
	case address == 0x4014:
		return console.PPU.readRegister(address)
	case address == 0x4015:
		return console.APU.readRegister(address)
	case address == 0x4016:
		return readController(console.Controller1)
	case address == 0x4017:
		return readController(console.Controller2)
	case address < 0x6000:
		// TODO: I/O registers
	case address >= 0x6000:
		return readMapper(console.Mapper, address)
	default:
		log.Fatalf("unhandled cpu memory read at address: 0x%04X", address)
	}
	return 0
}

func WriteByte(console *Console, address uint16, value byte) {
	writeController := func (c *Controller, value byte) {
		c.strobe = value
		if c.strobe&1 == 1 {
			c.index = 0
		}
	}
	switch {
	case address < 0x2000:
		console.RAM[address%0x0800] = value
	case address < 0x4000:
		console.PPU.writeRegister(0x2000+address%8, value)
	case address < 0x4014:
		console.APU.writeRegister(address, value)
	case address == 0x4014:
		console.PPU.writeRegister(address, value)
	case address == 0x4015:
		console.APU.writeRegister(address, value)
	case address == 0x4016:
		writeController(console.Controller1, value)
		writeController(console.Controller2, value)
	case address == 0x4017:
		console.APU.writeRegister(address, value)
	case address < 0x6000:
		// TODO: I/O registers
	case address >= 0x6000:
		writeMapper(console.Mapper, address, value)
	default:
		log.Fatalf("unhandled cpu memory write at address: 0x%04X", address)
	}
}

func (mem *ppuMemory) Read(address uint16) byte {
	address = address % 0x4000
	switch {
	case address < 0x2000:
		return readMapper(mem.console.Mapper, address)
	case address < 0x3F00:
		mode := mem.console.Cartridge.Mirror
		return mem.console.PPU.nameTableData[MirrorAddress(mode, address)%2048]
	case address < 0x4000:
		return mem.console.PPU.readPalette(address % 32)
	default:
		log.Fatalf("unhandled ppu memory read at address: 0x%04X", address)
	}
	return 0
}

func readMapper(mapper Mapper, address uint16) byte {
	switch m := mapper.(type) {
	case *Mapper1:
		switch {
		case address < 0x2000:
			bank := address / 0x1000
			offset := address % 0x1000
			return m.CHR[m.chrOffsets[bank]+int(offset)]
		case address >= 0x8000:
			address = address - 0x8000
			bank := address / 0x4000
			offset := address % 0x4000
			return m.PRG[m.prgOffsets[bank]+int(offset)]
		case address >= 0x6000:
			return m.SRAM[int(address)-0x6000]
		default:
			log.Fatalf("unhandled mapper1 read at address: 0x%04X", address)
		}
	case *Mapper2:
		switch {
		case address < 0x2000:
			return m.CHR[address]
		case address >= 0xC000:
			index := m.prgBank2*0x4000 + int(address-0xC000)
			return m.PRG[index]
		case address >= 0x8000:
			index := m.prgBank1*0x4000 + int(address-0x8000)
			return m.PRG[index]
		case address >= 0x6000:
			index := int(address) - 0x6000
			return m.SRAM[index]
		default:
			log.Fatalf("unhandled mapper2 read at address: 0x%04X", address)
		}
	case *Mapper3:
		switch {
		case address < 0x2000:
			index := m.chrBank*0x2000 + int(address)
			return m.CHR[index]
		case address >= 0xC000:
			index := m.prgBank2*0x4000 + int(address-0xC000)
			return m.PRG[index]
		case address >= 0x8000:
			index := m.prgBank1*0x4000 + int(address-0x8000)
			return m.PRG[index]
		case address >= 0x6000:
			index := int(address) - 0x6000
			return m.SRAM[index]
		default:
			log.Fatalf("unhandled mapper3 read at address: 0x%04X", address)
		}
	case *Mapper4:
		switch {
		case address < 0x2000:
			bank := address / 0x0400
			offset := address % 0x0400
			return m.CHR[m.chrOffsets[bank]+int(offset)]
		case address >= 0x8000:
			address = address - 0x8000
			bank := address / 0x2000
			offset := address % 0x2000
			return m.PRG[m.prgOffsets[bank]+int(offset)]
		case address >= 0x6000:
			return m.SRAM[int(address)-0x6000]
		default:
			log.Fatalf("unhandled mapper4 read at address: 0x%04X", address)
		}
	case *Mapper7:
		switch {
		case address < 0x2000:
			return m.CHR[address]
		case address >= 0x8000:
			index := m.prgBank*0x8000 + int(address-0x8000)
			return m.PRG[index]
		case address >= 0x6000:
			index := int(address) - 0x6000
			return m.SRAM[index]
		default:
			log.Fatalf("unhandled mapper7 read at address: 0x%04X", address)
		}
	}
	return 0  // unreachable
}



func writeMapper(mapper Mapper, address uint16, value byte) {
	switch m := mapper.(type) {
	case *Mapper1:
		switch {
		case address < 0x2000:
			bank := address / 0x1000
			offset := address % 0x1000
			m.CHR[m.chrOffsets[bank]+int(offset)] = value
		case address >= 0x8000:
			if value&0x80 == 0x80 {
				m.shiftRegister = 0x10
				writeControl1(m, m.control | 0x0C)
				updateOffsets1(m)
			} else {
				complete := m.shiftRegister&1 == 1
				m.shiftRegister >>= 1
				m.shiftRegister |= (value & 1) << 4
				if complete {
					switch {
					case address <= 0x9FFF:
						writeControl1(m, m.shiftRegister)
					case address <= 0xBFFF:     // CHR bank 0 (internal, $A000-$BFFF)
						m.chrBank0 = m.shiftRegister
					case address <= 0xDFFF:     // CHR bank 1 (internal, $C000-$DFFF)
						m.chrBank1 = m.shiftRegister
					case address <= 0xFFFF:     // PRG bank (internal, $E000-$FFFF)
						m.prgBank = m.shiftRegister & 0x0F
					}
					updateOffsets1(m)
					m.shiftRegister = 0x10
				}
			}
		case address >= 0x6000:
			m.SRAM[int(address)-0x6000] = value
		default:
			log.Fatalf("unhandled mapper1 write at address: 0x%04X", address)
		}
	case *Mapper2:
		switch {
		case address < 0x2000:
			m.CHR[address] = value
		case address >= 0x8000:
			m.prgBank1 = int(value) % m.prgBanks
		case address >= 0x6000:
			index := int(address) - 0x6000
			m.SRAM[index] = value
		default:
			log.Fatalf("unhandled mapper2 write at address: 0x%04X", address)
		}
	case *Mapper3:
		switch {
		case address < 0x2000:
			index := m.chrBank*0x2000 + int(address)
			m.CHR[index] = value
		case address >= 0x8000:
			m.chrBank = int(value & 3)
		case address >= 0x6000:
			index := int(address) - 0x6000
			m.SRAM[index] = value
		default:
			log.Fatalf("unhandled mapper3 write at address: 0x%04X", address)
		}
	case *Mapper4:
		switch {
		case address < 0x2000:
			bank := address / 0x0400
			offset := address % 0x0400
			m.CHR[m.chrOffsets[bank]+int(offset)] = value
		case address >= 0x8000:
			switch {
			case address <= 0x9FFF && address%2 == 0:
				// write bank select
				m.prgMode = (value >> 6) & 1
				m.chrMode = (value >> 7) & 1
				m.register = value & 7
				updateOffsets4(m)
			case address <= 0x9FFF && address%2 == 1:
				// write bank data
				m.registers[m.register] = value
				updateOffsets4(m)  
			case address <= 0xBFFF && address%2 == 0:
				// write mirror
				switch value & 1 {
				case 0:
					m.Cartridge.Mirror = MirrorVertical
				case 1:
					m.Cartridge.Mirror = MirrorHorizontal
				}
			case address <= 0xBFFF && address%2 == 1:
				// btw: think this was stubbed for something never implemented. anything important?
			case address <= 0xDFFF && address%2 == 0:
				// write IRQ latch
				m.reload = value  
			case address <= 0xDFFF && address%2 == 1:
				// write IRQ reload
				m.counter = 0
			case address <= 0xFFFF && address%2 == 0:
				// write IRQ disable
				m.irqEnable = false
			case address <= 0xFFFF && address%2 == 1:
				// write IRQ enable
				m.irqEnable = true
			}
		case address >= 0x6000:
			m.SRAM[int(address)-0x6000] = value
		default:
			log.Fatalf("unhandled mapper4 write at address: 0x%04X", address)
		}
	case *Mapper7:
		switch {
		case address < 0x2000:
			m.CHR[address] = value
		case address >= 0x8000:
			m.prgBank = int(value & 7)
			switch value & 0x10 {
			case 0x00:
				m.Cartridge.Mirror = MirrorSingle0
			case 0x10:
				m.Cartridge.Mirror = MirrorSingle1
			}
		case address >= 0x6000:
			index := int(address) - 0x6000
			m.SRAM[index] = value
		default:
			log.Fatalf("unhandled mapper7 write at address: 0x%04X", address)
		}
	}
}

// Control (internal, $8000-$9FFF)
func writeControl1(m *Mapper1, value byte) {
	m.control = value
	m.chrMode = (value >> 4) & 1
	m.prgMode = (value >> 2) & 3
	mirror := value & 3
	switch mirror {
	case 0:
		m.Cartridge.Mirror = MirrorSingle0
	case 1:
		m.Cartridge.Mirror = MirrorSingle1
	case 2:
		m.Cartridge.Mirror = MirrorVertical
	case 3:
		m.Cartridge.Mirror = MirrorHorizontal
	}
}


func prgBankOffset1(m *Mapper1, index int) int {
	if index >= 0x80 {
		index -= 0x100
	}
	index %= len(m.PRG) / 0x4000
	offset := index * 0x4000
	if offset < 0 {
		offset += len(m.PRG)
	}
	return offset
}

func chrBankOffset1(m *Mapper1, index int) int {
	if index >= 0x80 {
		index -= 0x100
	}
	index %= len(m.CHR) / 0x1000
	offset := index * 0x1000
	if offset < 0 {
		offset += len(m.CHR)
	}
	return offset
}

// PRG ROM bank mode (0, 1: switch 32 KB at $8000, ignoring low bit of bank number;
//                    2: fix first bank at $8000 and switch 16 KB bank at $C000;
//                    3: fix last bank at $C000 and switch 16 KB bank at $8000)
// CHR ROM bank mode (0: switch 8 KB at a time; 1: switch two separate 4 KB banks)
func updateOffsets1(m *Mapper1) {
	switch m.prgMode {
	case 0, 1:
		m.prgOffsets[0] = prgBankOffset1(m, int(m.prgBank & 0xFE))
		m.prgOffsets[1] = prgBankOffset1(m, int(m.prgBank | 0x01))
	case 2:
		m.prgOffsets[0] = 0
		m.prgOffsets[1] = prgBankOffset1(m, int(m.prgBank))
	case 3:
		m.prgOffsets[0] = prgBankOffset1(m, int(m.prgBank))
		m.prgOffsets[1] = prgBankOffset1(m, -1)
	}
	switch m.chrMode {
	case 0:
		m.chrOffsets[0] = chrBankOffset1(m, int(m.chrBank0 & 0xFE))
		m.chrOffsets[1] = chrBankOffset1(m, int(m.chrBank0 | 0x01))
	case 1:
		m.chrOffsets[0] = chrBankOffset1(m, int(m.chrBank0))
		m.chrOffsets[1] = chrBankOffset1(m, int(m.chrBank1))
	}
}


func prgBankOffset4(m *Mapper4, index int) int {
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

func chrBankOffset4(m *Mapper4, index int) int {
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

func updateOffsets4(m *Mapper4) {
	switch m.prgMode {
	case 0:
		m.prgOffsets[0] = prgBankOffset4(m, int(m.registers[6]))
		m.prgOffsets[1] = prgBankOffset4(m, int(m.registers[7]))
		m.prgOffsets[2] = prgBankOffset4(m, -2)
		m.prgOffsets[3] = prgBankOffset4(m, -1)
	case 1:
		m.prgOffsets[0] = prgBankOffset4(m, -2)
		m.prgOffsets[1] = prgBankOffset4(m, int(m.registers[7]))
		m.prgOffsets[2] = prgBankOffset4(m, int(m.registers[6]))
		m.prgOffsets[3] = prgBankOffset4(m, -1)
	}
	switch m.chrMode {
	case 0:
		m.chrOffsets[0] = chrBankOffset4(m, int(m.registers[0] & 0xFE))
		m.chrOffsets[1] = chrBankOffset4(m, int(m.registers[0] | 0x01))
		m.chrOffsets[2] = chrBankOffset4(m, int(m.registers[1] & 0xFE))
		m.chrOffsets[3] = chrBankOffset4(m, int(m.registers[1] | 0x01))
		m.chrOffsets[4] = chrBankOffset4(m, int(m.registers[2]))
		m.chrOffsets[5] = chrBankOffset4(m, int(m.registers[3]))
		m.chrOffsets[6] = chrBankOffset4(m, int(m.registers[4]))
		m.chrOffsets[7] = chrBankOffset4(m, int(m.registers[5]))
	case 1:
		m.chrOffsets[0] = chrBankOffset4(m, int(m.registers[2]))
		m.chrOffsets[1] = chrBankOffset4(m, int(m.registers[3]))
		m.chrOffsets[2] = chrBankOffset4(m, int(m.registers[4]))
		m.chrOffsets[3] = chrBankOffset4(m, int(m.registers[5]))
		m.chrOffsets[4] = chrBankOffset4(m, int(m.registers[0] & 0xFE))
		m.chrOffsets[5] = chrBankOffset4(m, int(m.registers[0] | 0x01))
		m.chrOffsets[6] = chrBankOffset4(m, int(m.registers[1] & 0xFE))
		m.chrOffsets[7] = chrBankOffset4(m, int(m.registers[1] | 0x01))
	}
}

func (mem *ppuMemory) Write(address uint16, value byte) {
	address = address % 0x4000
	switch {
	case address < 0x2000:
		writeMapper(mem.console.Mapper, address, value)
	case address < 0x3F00:
		mode := mem.console.Cartridge.Mirror
		mem.console.PPU.nameTableData[MirrorAddress(mode, address)%2048] = value
	case address < 0x4000:
		mem.console.PPU.writePalette(address%32, value)
	default:
		log.Fatalf("unhandled ppu memory write at address: 0x%04X", address)
	}
}

func MirrorAddress(mode byte, address uint16) uint16 {
	address = (address - 0x2000) % 0x1000
	table := address / 0x0400
	offset := address % 0x0400
	return 0x2000 + MirrorLookup[mode][table]*0x0400 + offset
}
