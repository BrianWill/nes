package nes

import "log"

func (mem *cpuMemory) Read(address uint16) byte {
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
		return mem.console.RAM[address%0x0800]
	case address < 0x4000:
		return mem.console.PPU.readRegister(0x2000 + address%8)
	case address == 0x4014:
		return mem.console.PPU.readRegister(address)
	case address == 0x4015:
		return mem.console.APU.readRegister(address)
	case address == 0x4016:
		return readController(mem.console.Controller1)
	case address == 0x4017:
		return readController(mem.console.Controller2)
	case address < 0x6000:
		// TODO: I/O registers
	case address >= 0x6000:
		return readMapper(mem.console.Mapper, address)
	default:
		log.Fatalf("unhandled cpu memory read at address: 0x%04X", address)
	}
	return 0
}

func (mem *cpuMemory) Write(address uint16, value byte) {
	writeController := func (c *Controller, value byte) {
		c.strobe = value
		if c.strobe&1 == 1 {
			c.index = 0
		}
	}
	switch {
	case address < 0x2000:
		mem.console.RAM[address%0x0800] = value
	case address < 0x4000:
		mem.console.PPU.writeRegister(0x2000+address%8, value)
	case address < 0x4014:
		mem.console.APU.writeRegister(address, value)
	case address == 0x4014:
		mem.console.PPU.writeRegister(address, value)
	case address == 0x4015:
		mem.console.APU.writeRegister(address, value)
	case address == 0x4016:
		writeController(mem.console.Controller1, value)
		writeController(mem.console.Controller2, value)
	case address == 0x4017:
		mem.console.APU.writeRegister(address, value)
	case address < 0x6000:
		// TODO: I/O registers
	case address >= 0x6000:
		writeMapper(mem.console.Mapper, value)
	default:
		log.Fatalf("unhandled cpu memory write at address: 0x%04X", address)
	}
}

// PPU Memory Map

type ppuMemory struct {
	console *Console
}

func NewPPUMemory(console *Console) Memory {
	return &ppuMemory{console}
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
			m.loadRegister(address, value)
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
			m.writeRegister(address, value)
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
	return 0  // unreachable
}




func (mem *ppuMemory) Write(address uint16, value byte) {
	address = address % 0x4000
	switch {
	case address < 0x2000:
		writeMapper(mem.console.Mapper, value)
	case address < 0x3F00:
		mode := mem.console.Cartridge.Mirror
		mem.console.PPU.nameTableData[MirrorAddress(mode, address)%2048] = value
	case address < 0x4000:
		mem.console.PPU.writePalette(address%32, value)
	default:
		log.Fatalf("unhandled ppu memory write at address: 0x%04X", address)
	}
}

// Mirroring Modes

const (
	MirrorHorizontal = 0
	MirrorVertical   = 1
	MirrorSingle0    = 2
	MirrorSingle1    = 3
	MirrorFour       = 4
)

var MirrorLookup = [...][4]uint16{
	{0, 0, 1, 1},
	{0, 1, 0, 1},
	{0, 0, 0, 0},
	{1, 1, 1, 1},
	{0, 1, 2, 3},
}

func MirrorAddress(mode byte, address uint16) uint16 {
	address = (address - 0x2000) % 0x1000
	table := address / 0x0400
	offset := address % 0x0400
	return 0x2000 + MirrorLookup[mode][table]*0x0400 + offset
}
