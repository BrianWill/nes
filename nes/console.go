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
		m.prgOffsets[1] = m.prgBankOffset(-1)
		console.Mapper = &m
	case 2:
		prgBanks := len(cartridge.PRG) / 0x4000
		console.Mapper = &Mapper2{cartridge, prgBanks, 0, prgBanks - 1}
	case 3:
		prgBanks := len(cartridge.PRG) / 0x4000
		console.Mapper = &Mapper3{cartridge, 0, 0, prgBanks - 1}
	case 4:
		m := Mapper4{Cartridge: cartridge, console: &console}
		m.prgOffsets[0] = m.prgBankOffset(0)
		m.prgOffsets[1] = m.prgBankOffset(1)
		m.prgOffsets[2] = m.prgBankOffset(-2)
		m.prgOffsets[3] = m.prgBankOffset(-1)
		console.Mapper = &m
	case 7:
		console.Mapper = &Mapper7{cartridge, 0}
	default:
		return nil, fmt.Errorf("unsupported mapper: %d", cartridge.Mapper)
	}

	cpu := CPU{Memory: &cpuMemory{&console}}
	{ // TODO we'll get rid of the lookup table
		c := &cpu
		c.table = [256]func(*stepInfo){
			c.brk, c.ora, c.kil, c.slo, c.nop, c.ora, c.asl, c.slo,
			c.php, c.ora, c.asl, c.anc, c.nop, c.ora, c.asl, c.slo,
			c.bpl, c.ora, c.kil, c.slo, c.nop, c.ora, c.asl, c.slo,
			c.clc, c.ora, c.nop, c.slo, c.nop, c.ora, c.asl, c.slo,
			c.jsr, c.and, c.kil, c.rla, c.bit, c.and, c.rol, c.rla,
			c.plp, c.and, c.rol, c.anc, c.bit, c.and, c.rol, c.rla,
			c.bmi, c.and, c.kil, c.rla, c.nop, c.and, c.rol, c.rla,
			c.sec, c.and, c.nop, c.rla, c.nop, c.and, c.rol, c.rla,
			c.rti, c.eor, c.kil, c.sre, c.nop, c.eor, c.lsr, c.sre,
			c.pha, c.eor, c.lsr, c.alr, c.jmp, c.eor, c.lsr, c.sre,
			c.bvc, c.eor, c.kil, c.sre, c.nop, c.eor, c.lsr, c.sre,
			c.cli, c.eor, c.nop, c.sre, c.nop, c.eor, c.lsr, c.sre,
			c.rts, c.adc, c.kil, c.rra, c.nop, c.adc, c.ror, c.rra,
			c.pla, c.adc, c.ror, c.arr, c.jmp, c.adc, c.ror, c.rra,
			c.bvs, c.adc, c.kil, c.rra, c.nop, c.adc, c.ror, c.rra,
			c.sei, c.adc, c.nop, c.rra, c.nop, c.adc, c.ror, c.rra,
			c.nop, c.sta, c.nop, c.sax, c.sty, c.sta, c.stx, c.sax,
			c.dey, c.nop, c.txa, c.xaa, c.sty, c.sta, c.stx, c.sax,
			c.bcc, c.sta, c.kil, c.ahx, c.sty, c.sta, c.stx, c.sax,
			c.tya, c.sta, c.txs, c.tas, c.shy, c.sta, c.shx, c.ahx,
			c.ldy, c.lda, c.ldx, c.lax, c.ldy, c.lda, c.ldx, c.lax,
			c.tay, c.lda, c.tax, c.lax, c.ldy, c.lda, c.ldx, c.lax,
			c.bcs, c.lda, c.kil, c.lax, c.ldy, c.lda, c.ldx, c.lax,
			c.clv, c.lda, c.tsx, c.las, c.ldy, c.lda, c.ldx, c.lax,
			c.cpy, c.cmp, c.nop, c.dcp, c.cpy, c.cmp, c.dec, c.dcp,
			c.iny, c.cmp, c.dex, c.axs, c.cpy, c.cmp, c.dec, c.dcp,
			c.bne, c.cmp, c.kil, c.dcp, c.nop, c.cmp, c.dec, c.dcp,
			c.cld, c.cmp, c.nop, c.dcp, c.nop, c.cmp, c.dec, c.dcp,
			c.cpx, c.sbc, c.nop, c.isc, c.cpx, c.sbc, c.inc, c.isc,
			c.inx, c.sbc, c.nop, c.sbc, c.cpx, c.sbc, c.inc, c.isc,
			c.beq, c.sbc, c.kil, c.isc, c.nop, c.sbc, c.inc, c.isc,
			c.sed, c.sbc, c.nop, c.isc, c.nop, c.sbc, c.inc, c.isc,
		}
	}
	cpu.Reset()
	console.CPU = &cpu
	
	apu := APU{
		console: &console,
	}
	apu.noise.shiftRegister = 1
	apu.pulse1.channel = 1
	apu.pulse2.channel = 2
	apu.dmc.cpu = console.CPU
	console.APU = &apu

	ppu := PPU{
		Memory: &ppuMemory{&console},
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

func (console *Console) Reset() {
	console.CPU.Reset()
}

func (console *Console) StepSeconds(seconds float64) {
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
					cpu.nmi()
				case interruptIRQ:
					cpu.irq()
				}
				cpu.interrupt = interruptNone

				opcode := cpu.Read(cpu.PC)
				mode := instructionModes[opcode]

				var address uint16
				var pageCrossed bool
				switch mode {
				case modeAbsolute:
					address = cpu.Read16(cpu.PC + 1)
				case modeAbsoluteX:
					address = cpu.Read16(cpu.PC+1) + uint16(cpu.X)
					pageCrossed = pagesDiffer(address-uint16(cpu.X), address)
				case modeAbsoluteY:
					address = cpu.Read16(cpu.PC+1) + uint16(cpu.Y)
					pageCrossed = pagesDiffer(address-uint16(cpu.Y), address)
				case modeAccumulator:
					address = 0
				case modeImmediate:
					address = cpu.PC + 1
				case modeImplied:
					address = 0
				case modeIndexedIndirect:
					address = cpu.read16bug(uint16(cpu.Read(cpu.PC+1) + cpu.X))
				case modeIndirect:
					address = cpu.read16bug(cpu.Read16(cpu.PC + 1))
				case modeIndirectIndexed:
					address = cpu.read16bug(uint16(cpu.Read(cpu.PC+1))) + uint16(cpu.Y)
					pageCrossed = pagesDiffer(address-uint16(cpu.Y), address)
				case modeRelative:
					offset := uint16(cpu.Read(cpu.PC + 1))
					if offset < 0x80 {
						address = cpu.PC + 2 + offset
					} else {
						address = cpu.PC + 2 + offset - 0x100
					}
				case modeZeroPage:
					address = uint16(cpu.Read(cpu.PC + 1))
				case modeZeroPageX:
					address = uint16(cpu.Read(cpu.PC+1) + cpu.X)
				case modeZeroPageY:
					address = uint16(cpu.Read(cpu.PC+1) + cpu.Y)
				}

				cpu.PC += uint16(instructionSizes[opcode])
				cpu.Cycles += uint64(instructionCycles[opcode])
				if pageCrossed {
					cpu.Cycles += uint64(instructionPageCycles[opcode])
				}
				info := &stepInfo{address, cpu.PC, mode}
				cpu.table[opcode](info)

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
							m.console.CPU.triggerIRQ()
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

func (console *Console) Buffer() *image.RGBA {
	return console.PPU.front
}

func (console *Console) SetButtons1(buttons [8]bool) {
	console.Controller1.buttons = buttons
}

func (console *Console) SetButtons2(buttons [8]bool) {
	console.Controller2.buttons = buttons
}

func (console *Console) SetAudioChannel(channel chan float32) {
	console.APU.channel = channel
}
