package nes

import "log"

func readByte(console *Console, address uint16) byte {
	readController := func(c *Controller) byte {
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
		return readPPURegister(console, 0x2000+address%8)
	case address == 0x4014:
		return readPPURegister(console, address)
	case address == 0x4015:
		// apu read register
		apu := console.APU
		switch address {
		case 0x4015:
			var readStatus byte
			if apu.pulse1.lengthValue > 0 {
				readStatus |= 1
			}
			if apu.pulse2.lengthValue > 0 {
				readStatus |= 2
			}
			if apu.triangle.lengthValue > 0 {
				readStatus |= 4
			}
			if apu.noise.lengthValue > 0 {
				readStatus |= 8
			}
			if apu.dmc.currentLength > 0 {
				readStatus |= 16
			}
			return readStatus
		}
		return 0
	case address == 0x4016:
		return readController(console.Controller1)
	case address == 0x4017:
		return readController(console.Controller2)
	case address < 0x6000:
		// TODO: I/O registers
	case address >= 0x6000:
		return readMapper(console.Mapper, console.Cartridge, address)
	default:
		log.Fatalf("unhandled cpu memory read at address: 0x%04X", address)
	}
	return 0
}

func writeByte(console *Console, address uint16, value byte) {
	writeController := func(c *Controller, value byte) {
		c.strobe = value
		if c.strobe&1 == 1 {
			c.index = 0
		}
	}

	writeRegisterAPU := func(apu *APU, address uint16, value byte) {
		pulseWriteControl := func(p *Pulse, value byte) {
			p.dutyMode = (value >> 6) & 3
			p.lengthEnabled = (value>>5)&1 == 0
			p.envelopeLoop = (value>>5)&1 == 1
			p.envelopeEnabled = (value>>4)&1 == 0
			p.envelopePeriod = value & 15
			p.constantVolume = value & 15
			p.envelopeStart = true
		}
		pulseWriteSweep := func(p *Pulse, value byte) {
			p.sweepEnabled = (value>>7)&1 == 1
			p.sweepPeriod = (value >> 4) & 7
			p.sweepNegate = (value>>3)&1 == 1
			p.sweepShift = value & 7
			p.sweepReload = true
		}
		pulseWriteTimerHigh := func(p *Pulse, value byte) {
			p.lengthValue = lengthTable[value>>3]
			p.timerPeriod = (p.timerPeriod & 0x00FF) | (uint16(value&7) << 8)
			p.envelopeStart = true
			p.dutyValue = 0
		}

		switch address {
		case 0x4000:
			pulseWriteControl(&apu.pulse1, value)
		case 0x4001:
			pulseWriteSweep(&apu.pulse1, value)
		case 0x4002:
			// write timer low
			apu.pulse1.timerPeriod = (apu.pulse1.timerPeriod & 0xFF00) | uint16(value)
		case 0x4003:
			pulseWriteTimerHigh(&apu.pulse1, value)
		case 0x4004:
			pulseWriteControl(&apu.pulse2, value)
		case 0x4005:
			pulseWriteSweep(&apu.pulse2, value)
		case 0x4006:
			// write timer low
			apu.pulse2.timerPeriod = (apu.pulse2.timerPeriod & 0xFF00) | uint16(value)
		case 0x4007:
			pulseWriteTimerHigh(&apu.pulse2, value)
		case 0x4008:
			// write control
			apu.triangle.lengthEnabled = (value>>7)&1 == 0
			apu.triangle.counterPeriod = value & 0x7F
		case 0x4009:
		case 0x4010:
			// write control
			apu.dmc.irq = value&0x80 == 0x80
			apu.dmc.loop = value&0x40 == 0x40
			apu.dmc.tickPeriod = dmcTable[value&0x0F]
		case 0x4011:
			// write value
			apu.dmc.value = value & 0x7F
		case 0x4012:
			// write address
			apu.dmc.sampleAddress = 0xC000 | (uint16(value) << 6)
		case 0x4013:
			// write length
			apu.dmc.sampleLength = (uint16(value) << 4) | 1
		case 0x400A:
			// write timer low
			apu.triangle.timerPeriod = (apu.triangle.timerPeriod & 0xFF00) | uint16(value)
		case 0x400B:
			// write timer high
			apu.triangle.lengthValue = lengthTable[value>>3]
			apu.triangle.timerPeriod = (apu.triangle.timerPeriod & 0x00FF) | (uint16(value&7) << 8)
			apu.triangle.timerValue = apu.triangle.timerPeriod
			apu.triangle.counterReload = true
		case 0x400C:
			// write control
			apu.noise.lengthEnabled = (value>>5)&1 == 0
			apu.noise.envelopeLoop = (value>>5)&1 == 1
			apu.noise.envelopeEnabled = (value>>4)&1 == 0
			apu.noise.envelopePeriod = value & 15
			apu.noise.constantVolume = value & 15
			apu.noise.envelopeStart = true
		case 0x400D:
		case 0x400E:
			// write period
			apu.noise.mode = value&0x80 == 0x80
			apu.noise.timerPeriod = noiseTable[value&0x0F]
		case 0x400F:
			// write length
			apu.noise.lengthValue = lengthTable[value>>3]
			apu.noise.envelopeStart = true
		case 0x4015:
			// apu write control
			apu.pulse1.enabled = value&1 == 1
			apu.pulse2.enabled = value&2 == 2
			apu.triangle.enabled = value&4 == 4
			apu.noise.enabled = value&8 == 8
			apu.dmc.enabled = value&16 == 16
			if !apu.pulse1.enabled {
				apu.pulse1.lengthValue = 0
			}
			if !apu.pulse2.enabled {
				apu.pulse2.lengthValue = 0
			}
			if !apu.triangle.enabled {
				apu.triangle.lengthValue = 0
			}
			if !apu.noise.enabled {
				apu.noise.lengthValue = 0
			}
			if !apu.dmc.enabled {
				apu.dmc.currentLength = 0
			} else {
				if apu.dmc.currentLength == 0 {
					dmcRestart(&apu.dmc)
				}
			}
		case 0x4017:
			// apu write frame counter
			apu.framePeriod = 4 + (value>>7)&1
			apu.frameIRQ = (value>>6)&1 == 0
		}
	}

	writeRegisterPPU := func(console *Console, address uint16, value byte) {
		ppu := console.PPU
		ppu.register = value
		switch address {
		case 0x2000:
			writeControlPPU(ppu, value)
		case 0x2001:
			writeMaskPPU(ppu, value)
		case 0x2003:
			ppu.oamAddress = value
		case 0x2004:
			// write OAM data
			ppu.oamData[ppu.oamAddress] = value
			ppu.oamAddress++
		case 0x2005:
			// write scroll
			if ppu.w == 0 {
				// t: ........ ...HGFED = d: HGFED...
				// x:               CBA = d: .....CBA
				// w:                   = 1
				ppu.t = (ppu.t & 0xFFE0) | (uint16(value) >> 3)
				ppu.x = value & 0x07
				ppu.w = 1
			} else {
				// t: .CBA..HG FED..... = d: HGFEDCBA
				// w:                   = 0
				ppu.t = (ppu.t & 0x8FFF) | ((uint16(value) & 0x07) << 12)
				ppu.t = (ppu.t & 0xFC1F) | ((uint16(value) & 0xF8) << 2)
				ppu.w = 0
			}
		case 0x2006:
			// write address
			if ppu.w == 0 {
				// t: ..FEDCBA ........ = d: ..FEDCBA
				// t: .X...... ........ = 0
				// w:                   = 1
				ppu.t = (ppu.t & 0x80FF) | ((uint16(value) & 0x3F) << 8)
				ppu.w = 1
			} else {
				// t: ........ HGFEDCBA = d: HGFEDCBA
				// v                    = t
				// w:                   = 0
				ppu.t = (ppu.t & 0xFF00) | uint16(value)
				ppu.v = ppu.t
				ppu.w = 0
			}
		case 0x2007:
			// write data
			writePPU(console, ppu.v, value)
			if ppu.flagIncrement == 0 {
				ppu.v += 1
			} else {
				ppu.v += 32
			}
		case 0x4014:
			// write DMA
			cpu := console.CPU
			address := uint16(value) << 8
			for i := 0; i < 256; i++ {
				ppu.oamData[ppu.oamAddress] = readByte(console, address)
				ppu.oamAddress++
				address++
			}
			cpu.stall += 513
			if cpu.Cycles%2 == 1 {
				cpu.stall++
			}
		}
	}

	switch {
	case address < 0x2000:
		console.RAM[address%0x0800] = value
	case address < 0x4000:
		writeRegisterPPU(console, 0x2000+address%8, value)
	case address < 0x4014:
		writeRegisterAPU(console.APU, address, value)
	case address == 0x4014:
		writeRegisterPPU(console, address, value)
	case address == 0x4015:
		writeRegisterAPU(console.APU, address, value)
	case address == 0x4016:
		writeController(console.Controller1, value)
		writeController(console.Controller2, value)
	case address == 0x4017:
		writeRegisterAPU(console.APU, address, value)
	case address < 0x6000:
		// TODO: I/O registers
	case address >= 0x6000:
		writeMapper(console.Mapper, console.Cartridge, address, value)
	default:
		log.Fatalf("unhandled cpu memory write at address: 0x%04X", address)
	}
}

func readPPU(console *Console, address uint16) byte {
	address = address % 0x4000
	switch {
	case address < 0x2000:
		return readMapper(console.Mapper, console.Cartridge, address)
	case address < 0x3F00:
		mode := console.Cartridge.Mirror
		return console.PPU.nameTableData[mirrorAddress(mode, address)%2048]
	case address < 0x4000:
		return readPalette(console.PPU, address%32)
	default:
		log.Fatalf("unhandled ppu memory read at address: 0x%04X", address)
	}
	return 0
}

func writePPU(console *Console, address uint16, value byte) {
	address = address % 0x4000
	switch {
	case address < 0x2000:
		writeMapper(console.Mapper, console.Cartridge, address, value)
	case address < 0x3F00:
		mode := console.Cartridge.Mirror
		console.PPU.nameTableData[mirrorAddress(mode, address)%2048] = value
	case address < 0x4000:
		// write palette
		address := address % 32
		if address >= 16 && address%4 == 0 {
			address -= 16
		}
		console.PPU.paletteData[address] = value
	default:
		log.Fatalf("unhandled ppu memory write at address: 0x%04X", address)
	}
}

func readMapper(mapper Mapper, cartridge *Cartridge, address uint16) byte {
	switch m := mapper.(type) {
	case *Mapper1:
		switch {
		case address < 0x2000:
			bank := address / 0x1000
			offset := address % 0x1000
			return cartridge.CHR[m.chrOffsets[bank]+int(offset)]
		case address >= 0x8000:
			address = address - 0x8000
			bank := address / 0x4000
			offset := address % 0x4000
			return cartridge.PRG[m.prgOffsets[bank]+int(offset)]
		case address >= 0x6000:
			return cartridge.SRAM[int(address)-0x6000]
		default:
			log.Fatalf("unhandled mapper1 read at address: 0x%04X", address)
		}
	case *Mapper2:
		switch {
		case address < 0x2000:
			return cartridge.CHR[address]
		case address >= 0xC000:
			index := m.prgBank2*0x4000 + int(address-0xC000)
			return cartridge.PRG[index]
		case address >= 0x8000:
			index := m.prgBank1*0x4000 + int(address-0x8000)
			return cartridge.PRG[index]
		case address >= 0x6000:
			index := int(address) - 0x6000
			return cartridge.SRAM[index]
		default:
			log.Fatalf("unhandled mapper2 read at address: 0x%04X", address)
		}
	case *Mapper3:
		switch {
		case address < 0x2000:
			index := m.chrBank*0x2000 + int(address)
			return cartridge.CHR[index]
		case address >= 0xC000:
			index := m.prgBank2*0x4000 + int(address-0xC000)
			return cartridge.PRG[index]
		case address >= 0x8000:
			index := m.prgBank1*0x4000 + int(address-0x8000)
			return cartridge.PRG[index]
		case address >= 0x6000:
			index := int(address) - 0x6000
			return cartridge.SRAM[index]
		default:
			log.Fatalf("unhandled mapper3 read at address: 0x%04X", address)
		}
	case *Mapper4:
		switch {
		case address < 0x2000:
			bank := address / 0x0400
			offset := address % 0x0400
			return cartridge.CHR[m.chrOffsets[bank]+int(offset)]
		case address >= 0x8000:
			address = address - 0x8000
			bank := address / 0x2000
			offset := address % 0x2000
			return cartridge.PRG[m.prgOffsets[bank]+int(offset)]
		case address >= 0x6000:
			return cartridge.SRAM[int(address)-0x6000]
		default:
			log.Fatalf("unhandled mapper4 read at address: 0x%04X", address)
		}
	case *Mapper7:
		switch {
		case address < 0x2000:
			return cartridge.CHR[address]
		case address >= 0x8000:
			index := m.prgBank*0x8000 + int(address-0x8000)
			return cartridge.PRG[index]
		case address >= 0x6000:
			index := int(address) - 0x6000
			return cartridge.SRAM[index]
		default:
			log.Fatalf("unhandled mapper7 read at address: 0x%04X", address)
		}
	}
	return 0 // unreachable
}

func writeMapper(mapper Mapper, cartridge *Cartridge, address uint16, value byte) {
	switch m := mapper.(type) {
	case *Mapper1:
		switch {
		case address < 0x2000:
			bank := address / 0x1000
			offset := address % 0x1000
			cartridge.CHR[m.chrOffsets[bank]+int(offset)] = value
		case address >= 0x8000:
			// PRG ROM bank mode (0, 1: switch 32 KB at $8000, ignoring low bit of bank number;
			//                    2: fix first bank at $8000 and switch 16 KB bank at $C000;
			//                    3: fix last bank at $C000 and switch 16 KB bank at $8000)
			// CHR ROM bank mode (0: switch 8 KB at a time; 1: switch two separate 4 KB banks)
			updateOffsets1 := func(m *Mapper1) {
				switch m.prgMode {
				case 0, 1:
					m.prgOffsets[0] = prgBankOffset1(cartridge, int(m.prgBank&0xFE))
					m.prgOffsets[1] = prgBankOffset1(cartridge, int(m.prgBank|0x01))
				case 2:
					m.prgOffsets[0] = 0
					m.prgOffsets[1] = prgBankOffset1(cartridge, int(m.prgBank))
				case 3:
					m.prgOffsets[0] = prgBankOffset1(cartridge, int(m.prgBank))
					m.prgOffsets[1] = prgBankOffset1(cartridge, -1)
				}

				chrBankOffset1 := func(m *Mapper1, index int) int {
					if index >= 0x80 {
						index -= 0x100
					}
					index %= len(cartridge.CHR) / 0x1000
					offset := index * 0x1000
					if offset < 0 {
						offset += len(cartridge.CHR)
					}
					return offset
				}

				switch m.chrMode {
				case 0:
					m.chrOffsets[0] = chrBankOffset1(m, int(m.chrBank0&0xFE))
					m.chrOffsets[1] = chrBankOffset1(m, int(m.chrBank0|0x01))
				case 1:
					m.chrOffsets[0] = chrBankOffset1(m, int(m.chrBank0))
					m.chrOffsets[1] = chrBankOffset1(m, int(m.chrBank1))
				}
			}

			// Control (internal, $8000-$9FFF)
			writeControl1 := func(m *Mapper1, value byte) {
				m.control = value
				m.chrMode = (value >> 4) & 1
				m.prgMode = (value >> 2) & 3
				mirror := value & 3
				switch mirror {
				case 0:
					cartridge.Mirror = MirrorSingle0
				case 1:
					cartridge.Mirror = MirrorSingle1
				case 2:
					cartridge.Mirror = MirrorVertical
				case 3:
					cartridge.Mirror = MirrorHorizontal
				}
			}

			if value&0x80 == 0x80 {
				m.shiftRegister = 0x10
				writeControl1(m, m.control|0x0C)
				updateOffsets1(m)
			} else {
				complete := m.shiftRegister&1 == 1
				m.shiftRegister >>= 1
				m.shiftRegister |= (value & 1) << 4
				if complete {
					switch {
					case address <= 0x9FFF:
						writeControl1(m, m.shiftRegister)
					case address <= 0xBFFF: // CHR bank 0 (internal, $A000-$BFFF)
						m.chrBank0 = m.shiftRegister
					case address <= 0xDFFF: // CHR bank 1 (internal, $C000-$DFFF)
						m.chrBank1 = m.shiftRegister
					case address <= 0xFFFF: // PRG bank (internal, $E000-$FFFF)
						m.prgBank = m.shiftRegister & 0x0F
					}
					updateOffsets1(m)
					m.shiftRegister = 0x10
				}
			}
		case address >= 0x6000:
			cartridge.SRAM[int(address)-0x6000] = value
		default:
			log.Fatalf("unhandled mapper1 write at address: 0x%04X", address)
		}
	case *Mapper2:
		switch {
		case address < 0x2000:
			cartridge.CHR[address] = value
		case address >= 0x8000:
			m.prgBank1 = int(value) % m.prgBanks
		case address >= 0x6000:
			index := int(address) - 0x6000
			cartridge.SRAM[index] = value
		default:
			log.Fatalf("unhandled mapper2 write at address: 0x%04X", address)
		}
	case *Mapper3:
		switch {
		case address < 0x2000:
			index := m.chrBank*0x2000 + int(address)
			cartridge.CHR[index] = value
		case address >= 0x8000:
			m.chrBank = int(value & 3)
		case address >= 0x6000:
			index := int(address) - 0x6000
			cartridge.SRAM[index] = value
		default:
			log.Fatalf("unhandled mapper3 write at address: 0x%04X", address)
		}
	case *Mapper4:
		switch {
		case address < 0x2000:
			bank := address / 0x0400
			offset := address % 0x0400
			cartridge.CHR[m.chrOffsets[bank]+int(offset)] = value
		case address >= 0x8000:
			updateOffsets4 := func(m *Mapper4) {
				switch m.prgMode {
				case 0:
					m.prgOffsets[0] = prgBankOffset4(cartridge, int(m.registers[6]))
					m.prgOffsets[1] = prgBankOffset4(cartridge, int(m.registers[7]))
					m.prgOffsets[2] = prgBankOffset4(cartridge, -2)
					m.prgOffsets[3] = prgBankOffset4(cartridge, -1)
				case 1:
					m.prgOffsets[0] = prgBankOffset4(cartridge, -2)
					m.prgOffsets[1] = prgBankOffset4(cartridge, int(m.registers[7]))
					m.prgOffsets[2] = prgBankOffset4(cartridge, int(m.registers[6]))
					m.prgOffsets[3] = prgBankOffset4(cartridge, -1)
				}

				chrBankOffset4 := func(m *Mapper4, index int) int {
					if index >= 0x80 {
						index -= 0x100
					}
					index %= len(cartridge.CHR) / 0x0400
					offset := index * 0x0400
					if offset < 0 {
						offset += len(cartridge.CHR)
					}
					return offset
				}

				switch m.chrMode {
				case 0:
					m.chrOffsets[0] = chrBankOffset4(m, int(m.registers[0]&0xFE))
					m.chrOffsets[1] = chrBankOffset4(m, int(m.registers[0]|0x01))
					m.chrOffsets[2] = chrBankOffset4(m, int(m.registers[1]&0xFE))
					m.chrOffsets[3] = chrBankOffset4(m, int(m.registers[1]|0x01))
					m.chrOffsets[4] = chrBankOffset4(m, int(m.registers[2]))
					m.chrOffsets[5] = chrBankOffset4(m, int(m.registers[3]))
					m.chrOffsets[6] = chrBankOffset4(m, int(m.registers[4]))
					m.chrOffsets[7] = chrBankOffset4(m, int(m.registers[5]))
				case 1:
					m.chrOffsets[0] = chrBankOffset4(m, int(m.registers[2]))
					m.chrOffsets[1] = chrBankOffset4(m, int(m.registers[3]))
					m.chrOffsets[2] = chrBankOffset4(m, int(m.registers[4]))
					m.chrOffsets[3] = chrBankOffset4(m, int(m.registers[5]))
					m.chrOffsets[4] = chrBankOffset4(m, int(m.registers[0]&0xFE))
					m.chrOffsets[5] = chrBankOffset4(m, int(m.registers[0]|0x01))
					m.chrOffsets[6] = chrBankOffset4(m, int(m.registers[1]&0xFE))
					m.chrOffsets[7] = chrBankOffset4(m, int(m.registers[1]|0x01))
				}
			}

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
					cartridge.Mirror = MirrorVertical
				case 1:
					cartridge.Mirror = MirrorHorizontal
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
			cartridge.SRAM[int(address)-0x6000] = value
		default:
			log.Fatalf("unhandled mapper4 write at address: 0x%04X", address)
		}
	case *Mapper7:
		switch {
		case address < 0x2000:
			cartridge.CHR[address] = value
		case address >= 0x8000:
			m.prgBank = int(value & 7)
			switch value & 0x10 {
			case 0x00:
				cartridge.Mirror = MirrorSingle0
			case 0x10:
				cartridge.Mirror = MirrorSingle1
			}
		case address >= 0x6000:
			index := int(address) - 0x6000
			cartridge.SRAM[index] = value
		default:
			log.Fatalf("unhandled mapper7 write at address: 0x%04X", address)
		}
	}
}

func prgBankOffset1(c *Cartridge, index int) int {
	if index >= 0x80 {
		index -= 0x100
	}
	index %= len(c.PRG) / 0x4000
	offset := index * 0x4000
	if offset < 0 {
		offset += len(c.PRG)
	}
	return offset
}

func prgBankOffset4(c *Cartridge, index int) int {
	if index >= 0x80 {
		index -= 0x100
	}
	index %= len(c.PRG) / 0x2000
	offset := index * 0x2000
	if offset < 0 {
		offset += len(c.PRG)
	}
	return offset
}

func mirrorAddress(mode byte, address uint16) uint16 {
	address = (address - 0x2000) % 0x1000
	table := address / 0x0400
	offset := address % 0x0400
	return 0x2000 + MirrorLookup[mode][table]*0x0400 + offset
}

func readPalette(ppu *PPU, address uint16) byte {
	if address >= 16 && address%4 == 0 {
		address -= 16
	}
	return ppu.paletteData[address]
}

func readPPURegister(console *Console, address uint16) byte {
	ppu := console.PPU
	switch address {
	case 0x2002: // PPUSTATUS
		// read status
		status := ppu.register & 0x1F
		status |= ppu.flagSpriteOverflow << 5
		status |= ppu.flagSpriteZeroHit << 6
		if ppu.nmiOccurred {
			status |= 1 << 7
		}
		ppu.nmiOccurred = false
		nmiChangePPU(ppu)
		ppu.w = 0
		return status
	case 0x2004:
		// OAM = Object Attribute Memory
		return ppu.oamData[ppu.oamAddress]
	case 0x2007:
		// read data
		value := readPPU(console, ppu.v)
		// emulate buffered reads
		if ppu.v%0x4000 < 0x3F00 {
			buffered := ppu.bufferedData
			ppu.bufferedData = value
			value = buffered
		} else {
			ppu.bufferedData = readPPU(console, ppu.v-0x1000)
		}
		// increment address
		if ppu.flagIncrement == 0 {
			ppu.v += 1
		} else {
			ppu.v += 32
		}
		return value
	}
	return 0
}

// $2000: PPUCTRL
func writeControlPPU(ppu *PPU, value byte) {
	ppu.flagNameTable = (value >> 0) & 3
	ppu.flagIncrement = (value >> 2) & 1
	ppu.flagSpriteTable = (value >> 3) & 1
	ppu.flagBackgroundTable = (value >> 4) & 1
	ppu.flagSpriteSize = (value >> 5) & 1
	ppu.flagMasterSlave = (value >> 6) & 1
	ppu.nmiOutput = (value>>7)&1 == 1
	nmiChangePPU(ppu)
	// t: ....BA.. ........ = d: ......BA
	ppu.t = (ppu.t & 0xF3FF) | ((uint16(value) & 0x03) << 10)
}

// $2001: PPUMASK
func writeMaskPPU(ppu *PPU, value byte) {
	ppu.flagGrayscale = (value >> 0) & 1
	ppu.flagShowLeftBackground = (value >> 1) & 1
	ppu.flagShowLeftSprites = (value >> 2) & 1
	ppu.flagShowBackground = (value >> 3) & 1
	ppu.flagShowSprites = (value >> 4) & 1
	ppu.flagRedTint = (value >> 5) & 1
	ppu.flagGreenTint = (value >> 6) & 1
	ppu.flagBlueTint = (value >> 7) & 1
}
