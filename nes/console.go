package nes

import (
	"image"
	"fmt"
	"encoding/binary"
	"errors"
	"io"
	"os"
)

func NewConsole(path string) (*Console, error) {
	// read an iNES file (.nes) and returns a Cartridge on success.
	// http://wiki.nesdev.com/w/index.php/INES
	// http://nesdev.com/NESDoc.pdf (page 28)
	cartridge, err := (func () (*Cartridge, error) {
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
		console.Mapper = &Mapper2{cartridge, prgBanks, 0, prgBanks - 1}
	case 1:
		m := Mapper1{Cartridge: cartridge, shiftRegister: 0x10}
		m.prgOffsets[1] = prgBankOffset1(&m, -1)
		console.Mapper = &m
	case 2:
		prgBanks := len(cartridge.PRG) / 0x4000
		console.Mapper = &Mapper2{cartridge, prgBanks, 0, prgBanks - 1}
	case 3:
		prgBanks := len(cartridge.PRG) / 0x4000
		console.Mapper = &Mapper3{cartridge, 0, 0, prgBanks - 1}
	case 4:
		m := Mapper4{Cartridge: cartridge, console: &console}
		m.prgOffsets[0] = prgBankOffset4(&m, 0)
		m.prgOffsets[1] = prgBankOffset4(&m, 1)
		m.prgOffsets[2] = prgBankOffset4(&m, -2)
		m.prgOffsets[3] = prgBankOffset4(&m, -1)
		console.Mapper = &m
	case 7:
		console.Mapper = &Mapper7{cartridge, 0}
	default:
		return nil, fmt.Errorf("unsupported mapper: %d", cartridge.Mapper)
	}

	cpu := CPU{}
	console.CPU = &cpu
	Reset(&console)
	
	apu := APU{
		console: &console,
	}
	apu.noise.shiftRegister = 1
	apu.pulse1.channel = 1
	apu.pulse2.channel = 2
	apu.dmc.cpu = console.CPU
	console.APU = &apu

	ppu := PPU{
		console: &console, 
		front: image.NewRGBA(image.Rect(0, 0, 256, 240)), 
		back: image.NewRGBA(image.Rect(0, 0, 256, 240)),
		Cycle: 340,
		ScanLine: 250,
		Frame: 0,
	}
	ppu.writeControl(0)
	ppu.writeMask(0)
	ppu.writeOAMAddress(0)
	console.PPU = &ppu

	return &console, nil
}

func StepSeconds(console *Console, seconds float64) {
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
				cycles := cpu.Cycles

				switch cpu.interrupt {
				case interruptNMI:
					// non-maskable interrupt
					cpu := console.CPU
					push16(console, cpu.PC)
					php(console, 0, 0, 0)
					cpu.PC = read16(console, 0xFFFA)
					cpu.I = 1
					cpu.Cycles += 7
				case interruptIRQ:
					cpu := console.CPU
					push16(console, cpu.PC)
					php(console, 0, 0, 0)
					cpu.PC = read16(console, 0xFFFE)
					cpu.I = 1
					cpu.Cycles += 7
				}
				cpu.interrupt = interruptNone

				opcode := ReadByte(console, cpu.PC)
				mode := instructions[opcode].Mode

				var address uint16
				var pageCrossed bool
				switch mode {
				case modeAbsolute:
					address = read16(console, cpu.PC + 1)
				case modeAbsoluteX:
					address = read16(console, cpu.PC+1) + uint16(cpu.X)
					pageCrossed = pagesDiffer(address-uint16(cpu.X), address)
				case modeAbsoluteY:
					address = read16(console, cpu.PC+1) + uint16(cpu.Y)
					pageCrossed = pagesDiffer(address-uint16(cpu.Y), address)
				case modeAccumulator:
					address = 0
				case modeImmediate:
					address = cpu.PC + 1
				case modeImplied:
					address = 0
				case modeIndexedIndirect:
					address = read16bug(console, uint16(ReadByte(console, cpu.PC+1) + cpu.X))
				case modeIndirect:
					address = read16bug(console, read16(console, cpu.PC + 1))
				case modeIndirectIndexed:
					address = read16bug(console, uint16(ReadByte(console, cpu.PC+1))) + uint16(cpu.Y)
					pageCrossed = pagesDiffer(address-uint16(cpu.Y), address)
				case modeRelative:
					offset := uint16(ReadByte(console, cpu.PC + 1))
					if offset < 0x80 {
						address = cpu.PC + 2 + offset
					} else {
						address = cpu.PC + 2 + offset - 0x100
					}
				case modeZeroPage:
					address = uint16(ReadByte(console, cpu.PC + 1))
				case modeZeroPageX:
					address = uint16(ReadByte(console, cpu.PC+1) + cpu.X)
				case modeZeroPageY:
					address = uint16(ReadByte(console, cpu.PC+1) + cpu.Y)
				}
				instruction := instructions[opcode]
				cpu.PC += uint16(instruction.Size)
				cpu.Cycles += uint64(instruction.Cycles)
				if pageCrossed {
					cpu.Cycles += uint64(instruction.PageCycles)
				}
				instruction.Function(console, address, cpu.PC, mode)

				cpuCycles = int(cpu.Cycles - cycles)
			}
		}

		// triggerIRQ causes an IRQ interrupt to occur on the next cycle
		triggerIRQ := func (cpu *CPU) {
			if cpu.I == 0 {
				cpu.interrupt = interruptIRQ
			}
		}

		ppuCycles := cpuCycles * 3
		for i := 0; i < ppuCycles; i++ {
			console.PPU.Step()
			switch m := console.Mapper.(type) {
			case *Mapper1, *Mapper2, *Mapper3, *Mapper7:
				// do nothing
			case *Mapper4:
				ppu := m.console.PPU
				if ppu.Cycle == 280 &&
						(ppu.ScanLine <= 239 || ppu.ScanLine >= 261) && 
						(ppu.flagShowBackground != 0 || ppu.flagShowSprites != 0) {
					if m.counter == 0 {
						m.counter = m.reload
					} else {
						m.counter--
						if m.counter == 0 && m.irqEnable {
							triggerIRQ(m.console.CPU)
						}
					}
				}
			}
		}
		for i := 0; i < cpuCycles; i++ {
			// step APU
			apu := console.APU

			stepEnvelope := func (apu *APU) {
				pulseStepEvelope := func (p *Pulse) {
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
				pulseStepEvelope(&apu.pulse1)
				pulseStepEvelope(&apu.pulse2)

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

			stepLength := func (apu *APU)  {
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
				if apu.cycle % 2 == 0 {
					stepPulseTimer := func (p *Pulse) {
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
							d.cpu.stall += 4
							d.shiftRegister = ReadByte(apu.console, d.currentAddress)
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

				stepSweep := func (apu *APU) {
					pulseStepSweep := func (p *Pulse)  {
						sweep := func (p *Pulse) {
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
							triggerIRQ(apu.console.CPU)
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
				pulseOutput := func (p *Pulse) byte {
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
				if !n.enabled || n.lengthValue == 0 || (n.shiftRegister & 1) == 1 {
					nOut = 0
				} else if n.envelopeEnabled {
					nOut = n.envelopeVolume
				} else {
					nOut = n.constantVolume
				}

				// dmc output
				dOut := apu.dmc.value

				output := tndTable[(3 * tOut) + (2 * nOut) + dOut] + pulseTable[p1Out + p2Out]
				select {
				case apu.channel <- output:
				default:
				}
			}
		}
		cycles -= cpuCycles
	}
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
