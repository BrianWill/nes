package nes

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
)

func NewConsole(path string) (*Console, error) {
	// read an iNES file (.nes) and returns a Cartridge on success.
	// http://wiki.nesdev.com/w/index.php/INES
	// http://nesdev.com/NESDoc.pdf (page 28)
	cartridge, err := (func() (*Cartridge, error) {
		// open file
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer file.Close()

		// read file header
		header := iNESFileHeader{}
		if err := binary.Read(file, binary.LittleEndian, &header); err != nil {
			return nil, err
		}

		// verify header magic number
		if header.Magic != iNESFileMagic {
			return nil, errors.New("invalid .nes file")
		}

		// mapper type
		mapper1 := header.Control1 >> 4
		mapper2 := header.Control2 >> 4
		mapper := mapper1 | mapper2<<4

		// mirroring type
		mirror1 := header.Control1 & 1
		mirror2 := (header.Control1 >> 3) & 1
		mirror := mirror1 | mirror2<<1

		// battery-backed RAM
		battery := (header.Control1 >> 1) & 1

		// read trainer if present (unused)
		if header.Control1&4 == 4 {
			trainer := make([]byte, 512)
			if _, err := io.ReadFull(file, trainer); err != nil {
				return nil, err
			}
		}

		// read prg-rom bank(s)
		prg := make([]byte, int(header.NumPRG)*16384)
		if _, err := io.ReadFull(file, prg); err != nil {
			return nil, err
		}

		// read chr-rom bank(s)
		chr := make([]byte, int(header.NumCHR)*8192)
		if _, err := io.ReadFull(file, chr); err != nil {
			return nil, err
		}

		// provide chr-rom/ram if not in file
		if header.NumCHR == 0 {
			chr = make([]byte, 8192)
		}

		// success
		return &Cartridge{prg, chr, make([]byte, 0x2000), mapper, mirror, battery}, nil
	})()
	if err != nil {
		return nil, err
	}

	ram := make([]byte, 2048)
	controller1 := &Controller{}
	controller2 := &Controller{}
	console := Console{nil, nil, nil, cartridge, controller1, controller2, nil, ram}

	// btw: why does the console need a cartridge if the mapper also has the same cartridge?
	switch cartridge.Mapper {
	case 0:
		prgBanks := len(cartridge.PRG) / 0x4000
		console.Mapper = &Mapper2{prgBanks, 0, prgBanks - 1}
	case 1:
		m := Mapper1{shiftRegister: 0x10}
		m.prgOffsets[1] = prgBankOffset1(cartridge, -1)
		console.Mapper = &m
	case 2:
		prgBanks := len(cartridge.PRG) / 0x4000
		console.Mapper = &Mapper2{prgBanks, 0, prgBanks - 1}
	case 3:
		prgBanks := len(cartridge.PRG) / 0x4000
		console.Mapper = &Mapper3{0, 0, prgBanks - 1}
	case 4:
		m := Mapper4{}
		m.prgOffsets[0] = prgBankOffset4(cartridge, 0)
		m.prgOffsets[1] = prgBankOffset4(cartridge, 1)
		m.prgOffsets[2] = prgBankOffset4(cartridge, -2)
		m.prgOffsets[3] = prgBankOffset4(cartridge, -1)
		console.Mapper = &m
	case 7:
		console.Mapper = &Mapper7{0}
	default:
		return nil, fmt.Errorf("unsupported mapper: %d", cartridge.Mapper)
	}

	cpu := CPU{}
	console.CPU = &cpu
	Reset(&console)

	apu := APU{}
	apu.noise.shiftRegister = 1
	apu.pulse1.channel = 1
	apu.pulse2.channel = 2
	console.APU = &apu

	ppu := PPU{
		front:    image.NewRGBA(image.Rect(0, 0, 256, 240)),
		back:     image.NewRGBA(image.Rect(0, 0, 256, 240)),
		Cycle:    340,
		ScanLine: 250,
		Frame:    0,
	}
	writeControlPPU(&ppu, 0)
	writeMaskPPU(&ppu, 0)
	ppu.oamAddress = 0
	console.PPU = &ppu

	return &console, nil
}

func StepSeconds(console *Console, seconds float64) {
	// causes an IRQ interrupt to occur on the next cycle
	triggerIRQ := func(cpu *CPU) {
		if cpu.I == 0 {
			cpu.interrupt = interruptIRQ
		}
	}

	// executes a single PPU cycle
	stepPPU := func(ppu *PPU) {
		// update Cycle, ScanLine and Frame counters
		if ppu.nmiDelay > 0 {
			ppu.nmiDelay--
			if ppu.nmiDelay == 0 && ppu.nmiOutput && ppu.nmiOccurred {
				console.CPU.interrupt = interruptNMI // non-maskable interrupt on next cycle
			}
		}
		if (ppu.flagShowBackground != 0 || ppu.flagShowSprites != 0) &&
			ppu.f == 1 && ppu.ScanLine == 261 && ppu.Cycle == 339 {
			ppu.Cycle = 0
			ppu.ScanLine = 0
			ppu.Frame++
			ppu.f ^= 1
		} else {
			ppu.Cycle++
			if ppu.Cycle > 340 {
				ppu.Cycle = 0
				ppu.ScanLine++
				if ppu.ScanLine > 261 {
					ppu.ScanLine = 0
					ppu.Frame++
					ppu.f ^= 1
				}
			}
		}

		renderingEnabled := ppu.flagShowBackground != 0 || ppu.flagShowSprites != 0
		preLine := ppu.ScanLine == 261
		visibleLine := ppu.ScanLine < 240
		renderLine := preLine || visibleLine
		preFetchCycle := ppu.Cycle >= 321 && ppu.Cycle <= 336
		visibleCycle := ppu.Cycle >= 1 && ppu.Cycle <= 256
		fetchCycle := preFetchCycle || visibleCycle

		// background logic
		if renderingEnabled {
			if visibleLine && visibleCycle {
				// render pixel
				x := ppu.Cycle - 1
				y := ppu.ScanLine

				// background pixel
				background := byte(0)
				if ppu.flagShowBackground != 0 {
					data := uint32(ppu.tileData>>32) >> ((7 - ppu.x) * 4)
					background = byte(data & 0x0F)
				}

				spritePixel := func() (byte, byte) {
					if ppu.flagShowSprites == 0 {
						return 0, 0
					}
					for i := 0; i < ppu.spriteCount; i++ {
						offset := (ppu.Cycle - 1) - int(ppu.spritePositions[i])
						if offset < 0 || offset > 7 {
							continue
						}
						offset = 7 - offset
						color := byte((ppu.spritePatterns[i] >> byte(offset*4)) & 0x0F)
						if color%4 == 0 {
							continue
						}
						return byte(i), color
					}
					return 0, 0
				}
				i, sprite := spritePixel()

				if x < 8 && ppu.flagShowLeftBackground == 0 {
					background = 0
				}
				if x < 8 && ppu.flagShowLeftSprites == 0 {
					sprite = 0
				}
				b := background%4 != 0
				s := sprite%4 != 0
				var color byte
				if !b && !s {
					color = 0
				} else if !b && s {
					color = sprite | 0x10
				} else if b && !s {
					color = background
				} else {
					if ppu.spriteIndexes[i] == 0 && x < 255 {
						ppu.flagSpriteZeroHit = 1
					}
					if ppu.spritePriorities[i] == 0 {
						color = sprite | 0x10
					} else {
						color = background
					}
				}
				c := Palette[readPalette(ppu, uint16(color))%64]
				ppu.back.SetRGBA(x, y, c)
			}
			if renderLine && fetchCycle {
				ppu.tileData <<= 4
				switch ppu.Cycle % 8 {
				case 1:
					// fetch name table byte
					v := ppu.v
					address := 0x2000 | (v & 0x0FFF)
					ppu.nameTableByte = readPPU(console, address)
				case 3:
					// fetch attribute table byte
					v := ppu.v
					address := 0x23C0 | (v & 0x0C00) | ((v >> 4) & 0x38) | ((v >> 2) & 0x07)
					shift := ((v >> 4) & 4) | (v & 2)
					ppu.attributeTableByte = ((readPPU(console, address) >> shift) & 3) << 2
				case 5:
					// fetch low tile byte
					fineY := (ppu.v >> 12) & 7
					table := ppu.flagBackgroundTable
					tile := ppu.nameTableByte
					address := 0x1000*uint16(table) + uint16(tile)*16 + fineY
					ppu.lowTileByte = readPPU(console, address)
				case 7:
					// fetch high tile byte
					fineY := (ppu.v >> 12) & 7
					table := ppu.flagBackgroundTable
					tile := ppu.nameTableByte
					address := 0x1000*uint16(table) + uint16(tile)*16 + fineY
					ppu.highTileByte = readPPU(console, address+8)
				case 0:
					// store tile data
					var data uint32
					for i := 0; i < 8; i++ {
						a := ppu.attributeTableByte
						p1 := (ppu.lowTileByte & 0x80) >> 7
						p2 := (ppu.highTileByte & 0x80) >> 6
						ppu.lowTileByte <<= 1
						ppu.highTileByte <<= 1
						data <<= 4
						data |= uint32(a | p1 | p2)
					}
					ppu.tileData |= uint64(data)
				}
			}
			if preLine && ppu.Cycle >= 280 && ppu.Cycle <= 304 {
				// copy Y
				// vert(v) = vert(t)
				// v: .IHGF.ED CBA..... = t: .IHGF.ED CBA.....
				ppu.v = (ppu.v & 0x841F) | (ppu.t & 0x7BE0)
			}
			if renderLine {
				if fetchCycle && ppu.Cycle%8 == 0 {
					// increment X
					// increment hori(v)
					// if coarse X == 31
					if ppu.v&0x001F == 31 {
						// coarse X = 0
						ppu.v &= 0xFFE0
						// switch horizontal nametable
						ppu.v ^= 0x0400
					} else {
						// increment coarse X
						ppu.v++
					}
				}
				if ppu.Cycle == 256 {
					// increment Y
					// increment vert(v)
					// if fine Y < 7
					if ppu.v&0x7000 != 0x7000 {
						// increment fine Y
						ppu.v += 0x1000
					} else {
						// fine Y = 0
						ppu.v &= 0x8FFF
						// let y = coarse Y
						y := (ppu.v & 0x03E0) >> 5
						if y == 29 {
							// coarse Y = 0
							y = 0
							// switch vertical nametable
							ppu.v ^= 0x0800
						} else if y == 31 {
							// coarse Y = 0, nametable not switched
							y = 0
						} else {
							// increment coarse Y
							y++
						}
						// put coarse Y back into v
						ppu.v = (ppu.v & 0xFC1F) | (y << 5)
					}
				}
				if ppu.Cycle == 257 {
					// copy X
					// hori(v) = hori(t)
					// v: .....F.. ...EDCBA = t: .....F.. ...EDCBA
					ppu.v = (ppu.v & 0xFBE0) | (ppu.t & 0x041F)
				}
			}
		}

		// sprite logic
		if renderingEnabled && ppu.Cycle == 257 {
			if visibleLine {
				// evaluate sprites
				var h int
				if ppu.flagSpriteSize == 0 {
					h = 8
				} else {
					h = 16
				}
				count := 0
				for i := 0; i < 64; i++ {
					y := ppu.oamData[i*4+0]
					a := ppu.oamData[i*4+2]
					x := ppu.oamData[i*4+3]
					row := ppu.ScanLine - int(y)
					if row < 0 || row >= h {
						continue
					}
					if count < 8 {
						// fetch sprite pattern
						var spritePattern uint32
						{
							tile := ppu.oamData[i*4+1]
							attributes := ppu.oamData[i*4+2]
							var address uint16
							if ppu.flagSpriteSize == 0 {
								if attributes&0x80 == 0x80 {
									row = 7 - row
								}
								table := ppu.flagSpriteTable
								address = 0x1000*uint16(table) + uint16(tile)*16 + uint16(row)
							} else {
								if attributes&0x80 == 0x80 {
									row = 15 - row
								}
								table := tile & 1
								tile &= 0xFE
								if row > 7 {
									tile++
									row -= 8
								}
								address = 0x1000*uint16(table) + uint16(tile)*16 + uint16(row)
							}
							atts := (attributes & 3) << 2
							lowTileByte := readPPU(console, address)
							highTileByte := readPPU(console, address+8)

							for i := 0; i < 8; i++ {
								var p1, p2 byte
								if attributes&0x40 == 0x40 {
									p1 = (lowTileByte & 1) << 0
									p2 = (highTileByte & 1) << 1
									lowTileByte >>= 1
									highTileByte >>= 1
								} else {
									p1 = (lowTileByte & 0x80) >> 7
									p2 = (highTileByte & 0x80) >> 6
									lowTileByte <<= 1
									highTileByte <<= 1
								}
								spritePattern <<= 4
								spritePattern |= uint32(atts | p1 | p2)
							}
						}
						ppu.spritePatterns[count] = spritePattern
						ppu.spritePositions[count] = x
						ppu.spritePriorities[count] = (a >> 5) & 1
						ppu.spriteIndexes[count] = byte(i)
					}
					count++
				}
				if count > 8 {
					count = 8
					ppu.flagSpriteOverflow = 1
				}
				ppu.spriteCount = count
			} else {
				ppu.spriteCount = 0
			}
		}

		// vblank logic
		if ppu.ScanLine == 241 && ppu.Cycle == 1 {
			// set vertical blank
			ppu.front, ppu.back = ppu.back, ppu.front
			ppu.nmiOccurred = true
			nmiChangePPU(ppu)
		}
		if preLine && ppu.Cycle == 1 {
			// clear vertical blank
			ppu.nmiOccurred = false
			nmiChangePPU(ppu)

			ppu.flagSpriteZeroHit = 0
			ppu.flagSpriteOverflow = 0
		}
	}

	stepAPU := func(apu *APU) {
		stepEnvelope := func(apu *APU) {
			pulseStepEnvelope := func(p *Pulse) {
				if p.envelopeStart {
					p.envelopeVolume = 15
					p.envelopeValue = p.envelopePeriod
					p.envelopeStart = false
				} else if p.envelopeValue > 0 {
					p.envelopeValue--
				} else {
					if p.envelopeVolume > 0 {
						p.envelopeVolume--
					} else if p.envelopeLoop {
						p.envelopeVolume = 15
					}
					p.envelopeValue = p.envelopePeriod
				}
			}
			pulseStepEnvelope(&apu.pulse1)
			pulseStepEnvelope(&apu.pulse2)

			t := &apu.triangle
			if t.counterReload {
				t.counterValue = t.counterPeriod
			} else if t.counterValue > 0 {
				t.counterValue--
			}
			if t.lengthEnabled {
				t.counterReload = false
			}

			n := &apu.noise
			if n.envelopeStart {
				n.envelopeVolume = 15
				n.envelopeValue = n.envelopePeriod
				n.envelopeStart = false
			} else if n.envelopeValue > 0 {
				n.envelopeValue--
			} else {
				if n.envelopeVolume > 0 {
					n.envelopeVolume--
				} else if n.envelopeLoop {
					n.envelopeVolume = 15
				}
				n.envelopeValue = n.envelopePeriod
			}
		}

		stepLength := func(apu *APU) {
			if apu.pulse1.lengthEnabled && apu.pulse1.lengthValue > 0 {
				apu.pulse1.lengthValue--
			}
			if apu.pulse2.lengthEnabled && apu.pulse2.lengthValue > 0 {
				apu.pulse2.lengthValue--
			}
			if apu.triangle.lengthEnabled && apu.triangle.lengthValue > 0 {
				apu.triangle.lengthValue--
			}
			if apu.noise.lengthEnabled && apu.noise.lengthValue > 0 {
				apu.noise.lengthValue--
			}
		}

		cycle1 := apu.cycle
		apu.cycle++
		cycle2 := apu.cycle

		// step timers
		{
			if apu.cycle%2 == 0 {
				stepPulseTimer := func(p *Pulse) {
					if p.timerValue == 0 {
						p.timerValue = p.timerPeriod
						p.dutyValue = (p.dutyValue + 1) % 8
					} else {
						p.timerValue--
					}
				}
				stepPulseTimer(&apu.pulse1)
				stepPulseTimer(&apu.pulse2)

				n := &apu.noise
				if n.timerValue == 0 {
					n.timerValue = n.timerPeriod
					var shift byte
					if n.mode {
						shift = 6
					} else {
						shift = 1
					}
					b1 := n.shiftRegister & 1
					b2 := (n.shiftRegister >> shift) & 1
					n.shiftRegister >>= 1
					n.shiftRegister |= (b1 ^ b2) << 14
				} else {
					n.timerValue--
				}

				d := &apu.dmc
				if d.enabled {
					// step reader
					if d.currentLength > 0 && d.bitCount == 0 {
						console.CPU.stall += 4
						d.shiftRegister = readByte(console, d.currentAddress)
						d.bitCount = 8
						d.currentAddress++
						if d.currentAddress == 0 {
							d.currentAddress = 0x8000
						}
						d.currentLength--
						if d.currentLength == 0 && d.loop {
							dmcRestart(d)
						}
					}

					if d.tickValue == 0 {
						d.tickValue = d.tickPeriod

						// step shifter
						if d.bitCount != 0 {
							if d.shiftRegister&1 == 1 {
								if d.value <= 125 {
									d.value += 2
								}
							} else {
								if d.value >= 2 {
									d.value -= 2
								}
							}
							d.shiftRegister >>= 1
							d.bitCount--
						}
					} else {
						d.tickValue--
					}
				}
			}

			t := &apu.triangle
			if t.timerValue == 0 {
				t.timerValue = t.timerPeriod
				if t.lengthValue > 0 && t.counterValue > 0 {
					t.dutyValue = (t.dutyValue + 1) % 32
				}
			} else {
				t.timerValue--
			}
		}

		f1 := int(float64(cycle1) / frameCounterRate)
		f2 := int(float64(cycle2) / frameCounterRate)
		if f1 != f2 {
			// step frame counters:

			stepSweep := func(apu *APU) {
				pulseStepSweep := func(p *Pulse) {
					sweep := func(p *Pulse) {
						delta := p.timerPeriod >> p.sweepShift
						if p.sweepNegate {
							p.timerPeriod -= delta
							if p.channel == 1 {
								p.timerPeriod--
							}
						} else {
							p.timerPeriod += delta
						}
					}

					if p.sweepReload {
						if p.sweepEnabled && p.sweepValue == 0 {
							sweep(p)
						}
						p.sweepValue = p.sweepPeriod
						p.sweepReload = false
					} else if p.sweepValue > 0 {
						p.sweepValue--
					} else {
						if p.sweepEnabled {
							sweep(p)
						}
						p.sweepValue = p.sweepPeriod
					}
				}
				pulseStepSweep(&apu.pulse1)
				pulseStepSweep(&apu.pulse2)
			}

			// mode 0:    mode 1:       function
			// ---------  -----------  -----------------------------
			//  - - - f    - - - - -    IRQ (if bit 6 is clear)
			//  - l - l    l - l - -    Length counter and sweep
			//  e e e e    e e e e -    Envelope and linear counter
			switch apu.framePeriod {
			case 4:
				apu.frameValue = (apu.frameValue + 1) % 4
				switch apu.frameValue {
				case 0, 2:
					stepEnvelope(apu)
				case 1:
					stepEnvelope(apu)
					stepSweep(apu)
					stepLength(apu)
				case 3:
					stepEnvelope(apu)
					stepSweep(apu)
					stepLength(apu)
					// fire IRQ
					if apu.frameIRQ {
						triggerIRQ(console.CPU)
					}
				}
			case 5:
				apu.frameValue = (apu.frameValue + 1) % 5
				switch apu.frameValue {
				case 1, 3:
					stepEnvelope(apu)
				case 0, 2:
					stepEnvelope(apu)
					stepSweep(apu)
					stepLength(apu)
				}
			}
		}
		s1 := int(float64(cycle1) / sampleRate)
		s2 := int(float64(cycle2) / sampleRate)
		if s1 != s2 {
			// pulse output
			pulseOutput := func(p *Pulse) byte {
				if !p.enabled || p.lengthValue == 0 || dutyTable[p.dutyMode][p.dutyValue] == 0 || p.timerPeriod < 8 || p.timerPeriod > 0x7FF {
					return 0
				} else if p.envelopeEnabled {
					return p.envelopeVolume
				} else {
					return p.constantVolume
				}
			}
			p1Out := pulseOutput(&apu.pulse1)
			p2Out := pulseOutput(&apu.pulse2)

			// triangle output
			t := &apu.triangle
			var tOut byte
			if !t.enabled || t.lengthValue == 0 || t.counterValue == 0 {
				tOut = 0
			} else {
				tOut = triangleTable[t.dutyValue]
			}

			// noise output
			n := &apu.noise
			var nOut byte
			if !n.enabled || n.lengthValue == 0 || (n.shiftRegister&1) == 1 {
				nOut = 0
			} else if n.envelopeEnabled {
				nOut = n.envelopeVolume
			} else {
				nOut = n.constantVolume
			}

			// dmc output
			dOut := apu.dmc.value

			output := tndTable[(3*tOut)+(2*nOut)+dOut] + pulseTable[p1Out+p2Out]
			select {
			case apu.channel <- output:
			default:
			}
		}
	}

	cycles := int(CPUFrequency * seconds)
	for cycles > 0 {
		// step cpu
		var cpuCycles int
		{
			cpu := console.CPU
			if cpu.stall > 0 {
				cpu.stall--
				cpuCycles = 1
			} else {
				startCycles := cpu.Cycles

				switch cpu.interrupt {
				case interruptNMI:
					// non-maskable interrupt
					cpu := console.CPU
					push16(console, cpu.PC)
					php(console)
					cpu.PC = read16(console, 0xFFFA)
					cpu.I = 1
					cpu.Cycles += 7
				case interruptIRQ:
					cpu := console.CPU
					push16(console, cpu.PC)
					php(console)
					cpu.PC = read16(console, 0xFFFE)
					cpu.I = 1
					cpu.Cycles += 7
				}
				cpu.interrupt = interruptNone
				opcode := readByte(console, cpu.PC)
				executeInstruction(console, opcode)
				cpuCycles = int(cpu.Cycles - startCycles)
			}
		}

		ppuCycles := cpuCycles * 3
		for i := 0; i < ppuCycles; i++ {
			stepPPU(console.PPU)

			switch m := console.Mapper.(type) {
			case *Mapper1, *Mapper2, *Mapper3, *Mapper7:
				// do nothing
			case *Mapper4:
				ppu := console.PPU
				if ppu.Cycle == 280 &&
					(ppu.ScanLine <= 239 || ppu.ScanLine >= 261) &&
					(ppu.flagShowBackground != 0 || ppu.flagShowSprites != 0) {
					if m.counter == 0 {
						m.counter = m.reload
					} else {
						m.counter--
						if m.counter == 0 && m.irqEnable {
							triggerIRQ(console.CPU)
						}
					}
				}
			}
		}
		for i := 0; i < cpuCycles; i++ {
			stepAPU(console.APU)
		}
		cycles -= cpuCycles
	}
}

func nmiChangePPU(ppu *PPU) {
	nmi := ppu.nmiOutput && ppu.nmiOccurred
	if nmi && !ppu.nmiPrevious {
		// TODO: this fixes some games but the delay shouldn't have to be so
		// long, so the timings are off somewhere
		ppu.nmiDelay = 15
	}
	ppu.nmiPrevious = nmi
}

func dmcRestart(d *DMC) {
	d.currentAddress = d.sampleAddress
	d.currentLength = d.sampleLength
}

func Buffer(console *Console) *image.RGBA {
	return console.PPU.front
}

func SetButtons1(console *Console, buttons [8]bool) {
	console.Controller1.buttons = buttons
}

func SetButtons2(console *Console, buttons [8]bool) {
	console.Controller2.buttons = buttons
}

func SetAudioChannel(console *Console, channel chan float32) {
	console.APU.channel = channel
}
