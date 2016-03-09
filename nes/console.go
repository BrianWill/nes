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
		Memory: &ppuMemory{&console}, console: &console, 
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
					nmi(console)
				case interruptIRQ:
					irq(console)
				}
				cpu.interrupt = interruptNone

				opcode := ReadByte(console, cpu.PC)
				mode := instructions[opcode].Mode

				var address uint16
				var pageCrossed bool
				switch mode {
				case modeAbsolute:
					address = Read16(console, cpu.PC + 1)
				case modeAbsoluteX:
					address = Read16(console, cpu.PC+1) + uint16(cpu.X)
					pageCrossed = pagesDiffer(address-uint16(cpu.X), address)
				case modeAbsoluteY:
					address = Read16(console, cpu.PC+1) + uint16(cpu.Y)
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
					address = read16bug(console, Read16(console, cpu.PC + 1))
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
			console.APU.Step()
		}
		cycles -= cpuCycles
	}
}

func executeInstruction(cpu *CPU, opcode byte, address, pc uint16, mode byte) {
	switch opcode {
	case 0:
	case 1:
	case 2:
	case 3:
	}
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
