package nes

// Reset resets the CPU to its initial powerup state
func Reset(console *Console) {
	cpu := console.CPU
	cpu.PC = Read16(console, 0xFFFC)
	cpu.SP = 0xFD
	SetFlags(cpu, 0x24)
}

// pagesDiffer returns true if the two addresses reference different pages
func pagesDiffer(a, b uint16) bool {
	return a&0xFF00 != b&0xFF00
}

// addBranchCycles adds a cycle for taking a branch and adds another cycle
// if the branch jumps to a new page
func addBranchCycles(cpu *CPU, address uint16, pc uint16) {
	cpu.Cycles++
	if pagesDiffer(pc, address) {
		cpu.Cycles++
	}
}

func compare(cpu *CPU, a, b byte) {
	setZN(cpu, a - b)
	if a >= b {
		cpu.C = 1
	} else {
		cpu.C = 0
	}
}

// Read16 reads two bytes using Read to return a double-word value
func Read16(console *Console, address uint16) uint16 {
	lo := uint16(ReadByte(console, address))
	hi := uint16(ReadByte(console, address + 1))
	return hi<<8 | lo
}

// read16bug emulates a 6502 bug that caused the low byte to wrap without
// incrementing the high byte
func read16bug(console *Console, address uint16) uint16 {
	a := address
	b := (a & 0xFF00) | uint16(byte(a)+1)
	lo := ReadByte(console, a)
	hi := ReadByte(console, b)
	return uint16(hi)<<8 | uint16(lo)
}

// push pushes a byte onto the stack
func push(console *Console, value byte) {
	cpu := console.CPU
	WriteByte(console, 0x100|uint16(cpu.SP), value)
	cpu.SP--
}

// pull pops a byte from the stack
func pull(console *Console) byte {
	cpu := console.CPU
	cpu.SP++
	return ReadByte(console, 0x100 | uint16(cpu.SP))
}

// push16 pushes two bytes onto the stack
func push16(console *Console, value uint16) {
	hi := byte(value >> 8)
	lo := byte(value & 0xFF)
	push(console, hi)
	push(console, lo)
}

// pull16 pops two bytes from the stack
func pull16(console *Console) uint16 {
	lo := uint16(pull(console))
	hi := uint16(pull(console))
	return hi<<8 | lo
}

// SetFlags sets the processor status flags
func SetFlags(cpu *CPU, flags byte) {
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
func setZ(cpu *CPU, value byte) {
	if value == 0 {
		cpu.Z = 1
	} else {
		cpu.Z = 0
	}
}

// setN sets the negative flag if the argument is negative (high bit is set)
func setN(cpu *CPU, value byte) {
	if value&0x80 != 0 {
		cpu.N = 1
	} else {
		cpu.N = 0
	}
}

// setZN sets the zero flag and the negative flag
func setZN(cpu *CPU, value byte) {
	setZ(cpu, value)
	setN(cpu, value)
}

// triggerIRQ causes an IRQ interrupt to occur on the next cycle
func triggerIRQ(cpu *CPU) {
	if cpu.I == 0 {
		cpu.interrupt = interruptIRQ
	}
}

// Step executes a single CPU instruction

// NMI - Non-Maskable Interrupt
func nmi(console *Console) {
	cpu := console.CPU
	push16(console, cpu.PC)
	php(console, 0, 0, 0)
	cpu.PC = Read16(console, 0xFFFA)
	cpu.I = 1
	cpu.Cycles += 7
}

// IRQ - IRQ Interrupt
func irq(console *Console) {
	cpu := console.CPU
	push16(console, cpu.PC)
	php(console, 0, 0, 0)
	cpu.PC = Read16(console, 0xFFFE)
	cpu.I = 1
	cpu.Cycles += 7
}


// OPCODES

// ADC - Add with Carry
func adc(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	a := cpu.A
	b := ReadByte(console, address)
	c := cpu.C
	cpu.A = a + b + c
	setZN(cpu, cpu.A)
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
func and(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	cpu.A = cpu.A & ReadByte(console, address)
	setZN(cpu, cpu.A)
}

// ASL - Arithmetic Shift Left
func asl(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	if mode == modeAccumulator {
		cpu.C = (cpu.A >> 7) & 1
		cpu.A <<= 1
		setZN(cpu, cpu.A)
	} else {
		value := ReadByte(console, address)
		cpu.C = (value >> 7) & 1
		value <<= 1
		WriteByte(console, address, value)
		setZN(cpu, value)
	}
}

// BCC - Branch if Carry Clear
func bcc(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	if cpu.C == 0 {
		cpu.PC = address
		addBranchCycles(cpu, address, pc)
	}
}

// BCS - Branch if Carry Set
func bcs(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	if cpu.C != 0 {
		cpu.PC = address
		addBranchCycles(cpu, address, pc)
	}
}

// BEQ - Branch if Equal
func beq(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	if cpu.Z != 0 {
		cpu.PC = address
		addBranchCycles(cpu, address, pc)
	}
}

// BIT - Bit Test
func bit(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	value := ReadByte(console, address)
	cpu.V = (value >> 6) & 1
	setZ(cpu, value & cpu.A)
	setN(cpu, value)
}

// BMI - Branch if Minus
func bmi(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	if cpu.N != 0 {
		cpu.PC = address
		addBranchCycles(cpu, address, pc)
	}
}

// BNE - Branch if Not Equal
func bne(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	if cpu.Z == 0 {
		cpu.PC = address
		addBranchCycles(cpu, address, pc)
	}
}

// BPL - Branch if Positive
func bpl(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	if cpu.N == 0 {
		cpu.PC = address
		addBranchCycles(cpu, address, pc)
	}
}

// BRK - Force Interrupt
func brk(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	push16(console, cpu.PC)
	php(console, address, pc, mode)
	sei(console, address, pc, mode)
	cpu.PC = Read16(console, 0xFFFE)
}

// BVC - Branch if Overflow Clear
func bvc(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	if cpu.V == 0 {
		cpu.PC = address
		addBranchCycles(cpu, address, pc)
	}
}

// BVS - Branch if Overflow Set
func bvs(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	if cpu.V != 0 {
		cpu.PC = address
		addBranchCycles(cpu, address, pc)
	}
}

// CLC - Clear Carry Flag
func clc(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	cpu.C = 0
}

// CLD - Clear Decimal Mode
func cld(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	cpu.D = 0
}

// CLI - Clear Interrupt Disable
func cli(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	cpu.I = 0
}

// CLV - Clear Overflow Flag
func clv(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	cpu.V = 0
}

// CMP - Compare
func cmp(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	value := ReadByte(console, address)
	compare(cpu, cpu.A, value)
}

// CPX - Compare X Register
func cpx(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	value := ReadByte(console, address)
	compare(cpu, cpu.X, value)
}

// CPY - Compare Y Register
func cpy(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	value := ReadByte(console, address)
	compare(cpu, cpu.Y, value)
}

// DEC - Decrement Memory
func dec(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	value := ReadByte(console, address) - 1
	WriteByte(console, address, value)
	setZN(cpu, value)
}

// DEX - Decrement X Register
func dex(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	cpu.X--
	setZN(cpu, cpu.X)
}

// DEY - Decrement Y Register
func dey(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	cpu.Y--
	setZN(cpu, cpu.Y)
}

// EOR - Exclusive OR
func eor(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	cpu.A = cpu.A ^ ReadByte(console, address)
	setZN(cpu, cpu.A)
}

// INC - Increment Memory
func inc(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	value := ReadByte(console, address) + 1
	WriteByte(console, address, value)
	setZN(cpu, value)
}

// INX - Increment X Register
func inx(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	cpu.X++
	setZN(cpu, cpu.X)
}

// INY - Increment Y Register
func iny(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	cpu.Y++
	setZN(cpu, cpu.Y)
}

// JMP - Jump
func jmp(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	cpu.PC = address
}

// JSR - Jump to Subroutine
func jsr(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	push16(console, cpu.PC - 1)
	cpu.PC = address
}

// LDA - Load Accumulator
func lda(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	cpu.A = ReadByte(console, address)
	setZN(cpu, cpu.A)
}

// LDX - Load X Register
func ldx(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	cpu.X = ReadByte(console, address)
	setZN(cpu, cpu.X)
}

// LDY - Load Y Register
func ldy(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	cpu.Y = ReadByte(console, address)
	setZN(cpu, cpu.Y)
}

// LSR - Logical Shift Right
func lsr(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	if mode == modeAccumulator {
		cpu.C = cpu.A & 1
		cpu.A >>= 1
		setZN(cpu, cpu.A)
	} else {
		value := ReadByte(console, address)
		cpu.C = value & 1
		value >>= 1
		WriteByte(console, address, value)
		setZN(cpu, value)
	}
}

// NOP - No Operation
func nop(console *Console, address uint16, pc uint16, mode byte) {
}

// ORA - Logical Inclusive OR
func ora(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	cpu.A = cpu.A | ReadByte(console, address)
	setZN(cpu, cpu.A)
}

// PHA - Push Accumulator
func pha(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	push(console, cpu.A)
}

// PHP - Push Processor Status
func php(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	var flags byte
	flags |= cpu.C << 0
	flags |= cpu.Z << 1
	flags |= cpu.I << 2
	flags |= cpu.D << 3
	flags |= cpu.B << 4
	flags |= cpu.U << 5
	flags |= cpu.V << 6
	flags |= cpu.N << 7
	push(console, flags | 0x10)
}

// PLA - Pull Accumulator
func pla(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	cpu.A = pull(console)
	setZN(cpu, cpu.A)
}

// PLP - Pull Processor Status
func plp(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	SetFlags(cpu, pull(console)&0xEF | 0x20)
}

// ROL - Rotate Left
func rol(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	if mode == modeAccumulator {
		c := cpu.C
		cpu.C = (cpu.A >> 7) & 1
		cpu.A = (cpu.A << 1) | c
		setZN(cpu, cpu.A)
	} else {
		c := cpu.C
		value := ReadByte(console, address)
		cpu.C = (value >> 7) & 1
		value = (value << 1) | c
		WriteByte(console, address, value)
		setZN(cpu, value)
	}
}

// ROR - Rotate Right
func ror(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	if mode == modeAccumulator {
		c := cpu.C
		cpu.C = cpu.A & 1
		cpu.A = (cpu.A >> 1) | (c << 7)
		setZN(cpu, cpu.A)
	} else {
		c := cpu.C
		value := ReadByte(console, address)
		cpu.C = value & 1
		value = (value >> 1) | (c << 7)
		WriteByte(console, address, value)
		setZN(cpu, value)
	}
}

// RTI - Return from Interrupt
func rti(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	SetFlags(cpu, pull(console)&0xEF | 0x20)
	cpu.PC = pull16(console)
}

// RTS - Return from Subroutine
func rts(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	cpu.PC = pull16(console) + 1
}

// SBC - Subtract with Carry
func sbc(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	a := cpu.A
	b := ReadByte(console, address)
	c := cpu.C
	cpu.A = a - b - (1 - c)
	setZN(cpu, cpu.A)
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
func sec(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	cpu.C = 1
}

// SED - Set Decimal Flag
func sed(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	cpu.D = 1
}

// SEI - Set Interrupt Disable
func sei(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	cpu.I = 1
}

// STA - Store Accumulator
func sta(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	WriteByte(console, address, cpu.A)
}

// STX - Store X Register
func stx(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	WriteByte(console, address, cpu.X)
}

// STY - Store Y Register
func sty(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	WriteByte(console, address, cpu.Y)
}

// TAX - Transfer Accumulator to X
func tax(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	cpu.X = cpu.A
	setZN(cpu, cpu.X)
}

// TAY - Transfer Accumulator to Y
func tay(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	cpu.Y = cpu.A
	setZN(cpu, cpu.Y)
}

// TSX - Transfer Stack Pointer to X
func tsx(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	cpu.X = cpu.SP
	setZN(cpu, cpu.X)
}

// TXA - Transfer X to Accumulator
func txa(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	cpu.A = cpu.X
	setZN(cpu, cpu.A)
}

// TXS - Transfer X to Stack Pointer
func txs(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	cpu.SP = cpu.X
}

// TYA - Transfer Y to Accumulator
func tya(console *Console, address uint16, pc uint16, mode byte) {
	cpu := console.CPU
	cpu.A = cpu.Y
	setZN(cpu, cpu.A)
}

// illegal opcodes below

func ahx(console *Console, address uint16, pc uint16, mode byte) {
}

func alr(console *Console, address uint16, pc uint16, mode byte) {
}

func anc(console *Console, address uint16, pc uint16, mode byte) {
}

func arr(console *Console, address uint16, pc uint16, mode byte) {
}

func axs(console *Console, address uint16, pc uint16, mode byte) {
}

func dcp(console *Console, address uint16, pc uint16, mode byte) {
}

func isc(console *Console, address uint16, pc uint16, mode byte) {
}

func kil(console *Console, address uint16, pc uint16, mode byte) {
}

func las(console *Console, address uint16, pc uint16, mode byte) {
}

func lax(console *Console, address uint16, pc uint16, mode byte) {
}

func rla(console *Console, address uint16, pc uint16, mode byte) {
}

func rra(console *Console, address uint16, pc uint16, mode byte) {
}

func sax(console *Console, address uint16, pc uint16, mode byte) {
}

func shx(console *Console, address uint16, pc uint16, mode byte) {
}

func shy(console *Console, address uint16, pc uint16, mode byte) {
}

func slo(console *Console, address uint16, pc uint16, mode byte) {
}

func sre(console *Console, address uint16, pc uint16, mode byte) {
}

func tas(console *Console, address uint16, pc uint16, mode byte) {
}

func xaa(console *Console, address uint16, pc uint16, mode byte) {
}
