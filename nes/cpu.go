package nes

// Reset resets the CPU to its initial powerup state
func (cpu *CPU) Reset() {
	cpu.PC = cpu.Read16(0xFFFC)
	cpu.SP = 0xFD
	cpu.SetFlags(0x24)
}

// pagesDiffer returns true if the two addresses reference different pages
func pagesDiffer(a, b uint16) bool {
	return a&0xFF00 != b&0xFF00
}

// addBranchCycles adds a cycle for taking a branch and adds another cycle
// if the branch jumps to a new page
func (cpu *CPU) addBranchCycles(info *stepInfo) {
	cpu.Cycles++
	if pagesDiffer(info.pc, info.address) {
		cpu.Cycles++
	}
}

func (cpu *CPU) compare(a, b byte) {
	cpu.setZN(a - b)
	if a >= b {
		cpu.C = 1
	} else {
		cpu.C = 0
	}
}

// Read16 reads two bytes using Read to return a double-word value
func (cpu *CPU) Read16(address uint16) uint16 {
	lo := uint16(cpu.Read(address))
	hi := uint16(cpu.Read(address + 1))
	return hi<<8 | lo
}

// read16bug emulates a 6502 bug that caused the low byte to wrap without
// incrementing the high byte
func (cpu *CPU) read16bug(address uint16) uint16 {
	a := address
	b := (a & 0xFF00) | uint16(byte(a)+1)
	lo := cpu.Read(a)
	hi := cpu.Read(b)
	return uint16(hi)<<8 | uint16(lo)
}

// push pushes a byte onto the stack
func (cpu *CPU) push(value byte) {
	cpu.Write(0x100|uint16(cpu.SP), value)
	cpu.SP--
}

// pull pops a byte from the stack
func (cpu *CPU) pull() byte {
	cpu.SP++
	return cpu.Read(0x100 | uint16(cpu.SP))
}

// push16 pushes two bytes onto the stack
func (cpu *CPU) push16(value uint16) {
	hi := byte(value >> 8)
	lo := byte(value & 0xFF)
	cpu.push(hi)
	cpu.push(lo)
}

// pull16 pops two bytes from the stack
func (cpu *CPU) pull16() uint16 {
	lo := uint16(cpu.pull())
	hi := uint16(cpu.pull())
	return hi<<8 | lo
}

// SetFlags sets the processor status flags
func (cpu *CPU) SetFlags(flags byte) {
	cpu.C = (flags >> 0) & 1
	cpu.Z = (flags >> 1) & 1
	cpu.I = (flags >> 2) & 1
	cpu.D = (flags >> 3) & 1
	cpu.B = (flags >> 4) & 1
	cpu.U = (flags >> 5) & 1
	cpu.V = (flags >> 6) & 1
	cpu.N = (flags >> 7) & 1
}

// setZ sets the zero flag if the argument is zero
func (cpu *CPU) setZ(value byte) {
	if value == 0 {
		cpu.Z = 1
	} else {
		cpu.Z = 0
	}
}

// setN sets the negative flag if the argument is negative (high bit is set)
func (cpu *CPU) setN(value byte) {
	if value&0x80 != 0 {
		cpu.N = 1
	} else {
		cpu.N = 0
	}
}

// setZN sets the zero flag and the negative flag
func (cpu *CPU) setZN(value byte) {
	cpu.setZ(value)
	cpu.setN(value)
}

// triggerIRQ causes an IRQ interrupt to occur on the next cycle
func (cpu *CPU) triggerIRQ() {
	if cpu.I == 0 {
		cpu.interrupt = interruptIRQ
	}
}

// Step executes a single CPU instruction

// NMI - Non-Maskable Interrupt
func (cpu *CPU) nmi() {
	cpu.push16(cpu.PC)
	cpu.php(nil)
	cpu.PC = cpu.Read16(0xFFFA)
	cpu.I = 1
	cpu.Cycles += 7
}

// IRQ - IRQ Interrupt
func (cpu *CPU) irq() {
	cpu.push16(cpu.PC)
	cpu.php(nil)
	cpu.PC = cpu.Read16(0xFFFE)
	cpu.I = 1
	cpu.Cycles += 7
}

// ADC - Add with Carry
func (cpu *CPU) adc(info *stepInfo) {
	a := cpu.A
	b := cpu.Read(info.address)
	c := cpu.C
	cpu.A = a + b + c
	cpu.setZN(cpu.A)
	if int(a)+int(b)+int(c) > 0xFF {
		cpu.C = 1
	} else {
		cpu.C = 0
	}
	if (a^b)&0x80 == 0 && (a^cpu.A)&0x80 != 0 {
		cpu.V = 1
	} else {
		cpu.V = 0
	}
}

// AND - Logical AND
func (cpu *CPU) and(info *stepInfo) {
	cpu.A = cpu.A & cpu.Read(info.address)
	cpu.setZN(cpu.A)
}

// ASL - Arithmetic Shift Left
func (cpu *CPU) asl(info *stepInfo) {
	if info.mode == modeAccumulator {
		cpu.C = (cpu.A >> 7) & 1
		cpu.A <<= 1
		cpu.setZN(cpu.A)
	} else {
		value := cpu.Read(info.address)
		cpu.C = (value >> 7) & 1
		value <<= 1
		cpu.Write(info.address, value)
		cpu.setZN(value)
	}
}

// BCC - Branch if Carry Clear
func (cpu *CPU) bcc(info *stepInfo) {
	if cpu.C == 0 {
		cpu.PC = info.address
		cpu.addBranchCycles(info)
	}
}

// BCS - Branch if Carry Set
func (cpu *CPU) bcs(info *stepInfo) {
	if cpu.C != 0 {
		cpu.PC = info.address
		cpu.addBranchCycles(info)
	}
}

// BEQ - Branch if Equal
func (cpu *CPU) beq(info *stepInfo) {
	if cpu.Z != 0 {
		cpu.PC = info.address
		cpu.addBranchCycles(info)
	}
}

// BIT - Bit Test
func (cpu *CPU) bit(info *stepInfo) {
	value := cpu.Read(info.address)
	cpu.V = (value >> 6) & 1
	cpu.setZ(value & cpu.A)
	cpu.setN(value)
}

// BMI - Branch if Minus
func (cpu *CPU) bmi(info *stepInfo) {
	if cpu.N != 0 {
		cpu.PC = info.address
		cpu.addBranchCycles(info)
	}
}

// BNE - Branch if Not Equal
func (cpu *CPU) bne(info *stepInfo) {
	if cpu.Z == 0 {
		cpu.PC = info.address
		cpu.addBranchCycles(info)
	}
}

// BPL - Branch if Positive
func (cpu *CPU) bpl(info *stepInfo) {
	if cpu.N == 0 {
		cpu.PC = info.address
		cpu.addBranchCycles(info)
	}
}

// BRK - Force Interrupt
func (cpu *CPU) brk(info *stepInfo) {
	cpu.push16(cpu.PC)
	cpu.php(info)
	cpu.sei(info)
	cpu.PC = cpu.Read16(0xFFFE)
}

// BVC - Branch if Overflow Clear
func (cpu *CPU) bvc(info *stepInfo) {
	if cpu.V == 0 {
		cpu.PC = info.address
		cpu.addBranchCycles(info)
	}
}

// BVS - Branch if Overflow Set
func (cpu *CPU) bvs(info *stepInfo) {
	if cpu.V != 0 {
		cpu.PC = info.address
		cpu.addBranchCycles(info)
	}
}

// CLC - Clear Carry Flag
func (cpu *CPU) clc(info *stepInfo) {
	cpu.C = 0
}

// CLD - Clear Decimal Mode
func (cpu *CPU) cld(info *stepInfo) {
	cpu.D = 0
}

// CLI - Clear Interrupt Disable
func (cpu *CPU) cli(info *stepInfo) {
	cpu.I = 0
}

// CLV - Clear Overflow Flag
func (cpu *CPU) clv(info *stepInfo) {
	cpu.V = 0
}

// CMP - Compare
func (cpu *CPU) cmp(info *stepInfo) {
	value := cpu.Read(info.address)
	cpu.compare(cpu.A, value)
}

// CPX - Compare X Register
func (cpu *CPU) cpx(info *stepInfo) {
	value := cpu.Read(info.address)
	cpu.compare(cpu.X, value)
}

// CPY - Compare Y Register
func (cpu *CPU) cpy(info *stepInfo) {
	value := cpu.Read(info.address)
	cpu.compare(cpu.Y, value)
}

// DEC - Decrement Memory
func (cpu *CPU) dec(info *stepInfo) {
	value := cpu.Read(info.address) - 1
	cpu.Write(info.address, value)
	cpu.setZN(value)
}

// DEX - Decrement X Register
func (cpu *CPU) dex(info *stepInfo) {
	cpu.X--
	cpu.setZN(cpu.X)
}

// DEY - Decrement Y Register
func (cpu *CPU) dey(info *stepInfo) {
	cpu.Y--
	cpu.setZN(cpu.Y)
}

// EOR - Exclusive OR
func (cpu *CPU) eor(info *stepInfo) {
	cpu.A = cpu.A ^ cpu.Read(info.address)
	cpu.setZN(cpu.A)
}

// INC - Increment Memory
func (cpu *CPU) inc(info *stepInfo) {
	value := cpu.Read(info.address) + 1
	cpu.Write(info.address, value)
	cpu.setZN(value)
}

// INX - Increment X Register
func (cpu *CPU) inx(info *stepInfo) {
	cpu.X++
	cpu.setZN(cpu.X)
}

// INY - Increment Y Register
func (cpu *CPU) iny(info *stepInfo) {
	cpu.Y++
	cpu.setZN(cpu.Y)
}

// JMP - Jump
func (cpu *CPU) jmp(info *stepInfo) {
	cpu.PC = info.address
}

// JSR - Jump to Subroutine
func (cpu *CPU) jsr(info *stepInfo) {
	cpu.push16(cpu.PC - 1)
	cpu.PC = info.address
}

// LDA - Load Accumulator
func (cpu *CPU) lda(info *stepInfo) {
	cpu.A = cpu.Read(info.address)
	cpu.setZN(cpu.A)
}

// LDX - Load X Register
func (cpu *CPU) ldx(info *stepInfo) {
	cpu.X = cpu.Read(info.address)
	cpu.setZN(cpu.X)
}

// LDY - Load Y Register
func (cpu *CPU) ldy(info *stepInfo) {
	cpu.Y = cpu.Read(info.address)
	cpu.setZN(cpu.Y)
}

// LSR - Logical Shift Right
func (cpu *CPU) lsr(info *stepInfo) {
	if info.mode == modeAccumulator {
		cpu.C = cpu.A & 1
		cpu.A >>= 1
		cpu.setZN(cpu.A)
	} else {
		value := cpu.Read(info.address)
		cpu.C = value & 1
		value >>= 1
		cpu.Write(info.address, value)
		cpu.setZN(value)
	}
}

// NOP - No Operation
func (cpu *CPU) nop(info *stepInfo) {
}

// ORA - Logical Inclusive OR
func (cpu *CPU) ora(info *stepInfo) {
	cpu.A = cpu.A | cpu.Read(info.address)
	cpu.setZN(cpu.A)
}

// PHA - Push Accumulator
func (cpu *CPU) pha(info *stepInfo) {
	cpu.push(cpu.A)
}

// PHP - Push Processor Status
func (cpu *CPU) php(info *stepInfo) {
	var flags byte
	flags |= cpu.C << 0
	flags |= cpu.Z << 1
	flags |= cpu.I << 2
	flags |= cpu.D << 3
	flags |= cpu.B << 4
	flags |= cpu.U << 5
	flags |= cpu.V << 6
	flags |= cpu.N << 7
	cpu.push(flags | 0x10)
}

// PLA - Pull Accumulator
func (cpu *CPU) pla(info *stepInfo) {
	cpu.A = cpu.pull()
	cpu.setZN(cpu.A)
}

// PLP - Pull Processor Status
func (cpu *CPU) plp(info *stepInfo) {
	cpu.SetFlags(cpu.pull()&0xEF | 0x20)
}

// ROL - Rotate Left
func (cpu *CPU) rol(info *stepInfo) {
	if info.mode == modeAccumulator {
		c := cpu.C
		cpu.C = (cpu.A >> 7) & 1
		cpu.A = (cpu.A << 1) | c
		cpu.setZN(cpu.A)
	} else {
		c := cpu.C
		value := cpu.Read(info.address)
		cpu.C = (value >> 7) & 1
		value = (value << 1) | c
		cpu.Write(info.address, value)
		cpu.setZN(value)
	}
}

// ROR - Rotate Right
func (cpu *CPU) ror(info *stepInfo) {
	if info.mode == modeAccumulator {
		c := cpu.C
		cpu.C = cpu.A & 1
		cpu.A = (cpu.A >> 1) | (c << 7)
		cpu.setZN(cpu.A)
	} else {
		c := cpu.C
		value := cpu.Read(info.address)
		cpu.C = value & 1
		value = (value >> 1) | (c << 7)
		cpu.Write(info.address, value)
		cpu.setZN(value)
	}
}

// RTI - Return from Interrupt
func (cpu *CPU) rti(info *stepInfo) {
	cpu.SetFlags(cpu.pull()&0xEF | 0x20)
	cpu.PC = cpu.pull16()
}

// RTS - Return from Subroutine
func (cpu *CPU) rts(info *stepInfo) {
	cpu.PC = cpu.pull16() + 1
}

// SBC - Subtract with Carry
func (cpu *CPU) sbc(info *stepInfo) {
	a := cpu.A
	b := cpu.Read(info.address)
	c := cpu.C
	cpu.A = a - b - (1 - c)
	cpu.setZN(cpu.A)
	if int(a)-int(b)-int(1-c) >= 0 {
		cpu.C = 1
	} else {
		cpu.C = 0
	}
	if (a^b)&0x80 != 0 && (a^cpu.A)&0x80 != 0 {
		cpu.V = 1
	} else {
		cpu.V = 0
	}
}

// SEC - Set Carry Flag
func (cpu *CPU) sec(info *stepInfo) {
	cpu.C = 1
}

// SED - Set Decimal Flag
func (cpu *CPU) sed(info *stepInfo) {
	cpu.D = 1
}

// SEI - Set Interrupt Disable
func (cpu *CPU) sei(info *stepInfo) {
	cpu.I = 1
}

// STA - Store Accumulator
func (cpu *CPU) sta(info *stepInfo) {
	cpu.Write(info.address, cpu.A)
}

// STX - Store X Register
func (cpu *CPU) stx(info *stepInfo) {
	cpu.Write(info.address, cpu.X)
}

// STY - Store Y Register
func (cpu *CPU) sty(info *stepInfo) {
	cpu.Write(info.address, cpu.Y)
}

// TAX - Transfer Accumulator to X
func (cpu *CPU) tax(info *stepInfo) {
	cpu.X = cpu.A
	cpu.setZN(cpu.X)
}

// TAY - Transfer Accumulator to Y
func (cpu *CPU) tay(info *stepInfo) {
	cpu.Y = cpu.A
	cpu.setZN(cpu.Y)
}

// TSX - Transfer Stack Pointer to X
func (cpu *CPU) tsx(info *stepInfo) {
	cpu.X = cpu.SP
	cpu.setZN(cpu.X)
}

// TXA - Transfer X to Accumulator
func (cpu *CPU) txa(info *stepInfo) {
	cpu.A = cpu.X
	cpu.setZN(cpu.A)
}

// TXS - Transfer X to Stack Pointer
func (cpu *CPU) txs(info *stepInfo) {
	cpu.SP = cpu.X
}

// TYA - Transfer Y to Accumulator
func (cpu *CPU) tya(info *stepInfo) {
	cpu.A = cpu.Y
	cpu.setZN(cpu.A)
}

// illegal opcodes below

func (cpu *CPU) ahx(info *stepInfo) {
}

func (cpu *CPU) alr(info *stepInfo) {
}

func (cpu *CPU) anc(info *stepInfo) {
}

func (cpu *CPU) arr(info *stepInfo) {
}

func (cpu *CPU) axs(info *stepInfo) {
}

func (cpu *CPU) dcp(info *stepInfo) {
}

func (cpu *CPU) isc(info *stepInfo) {
}

func (cpu *CPU) kil(info *stepInfo) {
}

func (cpu *CPU) las(info *stepInfo) {
}

func (cpu *CPU) lax(info *stepInfo) {
}

func (cpu *CPU) rla(info *stepInfo) {
}

func (cpu *CPU) rra(info *stepInfo) {
}

func (cpu *CPU) sax(info *stepInfo) {
}

func (cpu *CPU) shx(info *stepInfo) {
}

func (cpu *CPU) shy(info *stepInfo) {
}

func (cpu *CPU) slo(info *stepInfo) {
}

func (cpu *CPU) sre(info *stepInfo) {
}

func (cpu *CPU) tas(info *stepInfo) {
}

func (cpu *CPU) xaa(info *stepInfo) {
}
