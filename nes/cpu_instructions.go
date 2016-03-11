package nes

func executeInstruction(console *Console, opcode byte) {

    instruction := instructions[opcode]
    mode := instruction.Mode
    cpu := console.CPU

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

    cpu.PC += uint16(instruction.Size)
    cpu.Cycles += uint64(instruction.Cycles)
    if pageCrossed {
        cpu.Cycles += uint64(instruction.PageCycles)
    }

    pc := cpu.PC


    // OPCODE functions

    // ADC - Add with Carry
    adc := func () {
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
    and := func () {
        cpu.A = cpu.A & ReadByte(console, address)
        setZN(cpu, cpu.A)
    }

    // ASL - Arithmetic Shift Left
    asl := func () {
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

    // BIT - Bit Test
    bit := func () {
        value := ReadByte(console, address)
        cpu.V = (value >> 6) & 1
        setZ(cpu, value & cpu.A)
        setN(cpu, value)
    }

    // CMP - Compare
    cmp := func () {
        value := ReadByte(console, address)
        compare(cpu, cpu.A, value)
    }

    // CPX - Compare X Register
    cpx := func () {
        value := ReadByte(console, address)
        compare(cpu, cpu.X, value)
    }

    // CPY - Compare Y Register
    cpy := func () {
        value := ReadByte(console, address)
        compare(cpu, cpu.Y, value)
    }

    // DEC - Decrement Memory
    dec := func () {
        value := ReadByte(console, address) - 1
        WriteByte(console, address, value)
        setZN(cpu, value)
    }


    // EOR - Exclusive OR
    eor := func () {
        cpu.A = cpu.A ^ ReadByte(console, address)
        setZN(cpu, cpu.A)
    }

    // INC - Increment Memory
    inc := func () {
        value := ReadByte(console, address) + 1
        WriteByte(console, address, value)
        setZN(cpu, value)
    }

    // JMP - Jump
    jmp := func () {
        cpu.PC = address
    }

    // LDA - Load Accumulator
    lda := func () {
        cpu.A = ReadByte(console, address)
        setZN(cpu, cpu.A)
    }

    // LDX - Load X Register
    ldx := func () {
        cpu.X = ReadByte(console, address)
        setZN(cpu, cpu.X)
    }

    // LDY - Load Y Register
    ldy := func () {
        cpu.Y = ReadByte(console, address)
        setZN(cpu, cpu.Y)
    }

    // LSR - Logical Shift Right
    lsr := func () {
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


    // ORA - Logical Inclusive OR
    ora := func () {
        cpu.A = cpu.A | ReadByte(console, address)
        setZN(cpu, cpu.A)
    }


    // PHP - Push Processor Status
    php := func () {
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

    // ROL - Rotate Left
    rol := func () {
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
    ror := func () {
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


    // SBC - Subtract with Carry
    sbc := func () {
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

    // SEI - Set Interrupt Disable
    sei := func () {
        cpu.I = 1
    }

    // STA - Store Accumulator
    sta := func () {
        WriteByte(console, address, cpu.A)
    }

    // STX - Store X Register
    stx := func () {
        WriteByte(console, address, cpu.X)
    }

    // STY - Store Y Register
    sty := func () {
        WriteByte(console, address, cpu.Y)
    }



    switch opcode {
    case 0:
        // BRK - Force Interrupt
        push16(console, cpu.PC)
        php()
        sei()
        cpu.PC = read16(console, 0xFFFE)
    case 1:
        ora()
    case 2: // KIL
    case 3: // SLO
    case 4: // NOP
    case 5:
        ora()
    case 6:
        asl()
    case 7: // SLO
    case 8:
        php()
    case 9:
        ora()
    case 10:
        asl()
    case 11: // ANC
    case 12: // NOP
    case 13:
        ora()
    case 14:
        asl()
    case 15: // SLO
    case 16:
        // BPL - Branch if Positive
        if cpu.N == 0 {
            cpu.PC = address
            addBranchCycles(cpu, address, pc)
        }
    case 17:
        ora()
    case 18: // KIL
    case 19: // SLO
    case 20: // NOP
    case 21:
        ora()
    case 22:
        asl()
    case 23: // SLO
    case 24:
        // CLC - Clear Carry Flag
        cpu.C = 0
    case 25:
        ora()
    case 26: // NOP
    case 27: // SLO
    case 28: // NOP
    case 29:
        ora()
    case 30:
        asl()
    case 31: // SLO
    case 32:
        // JSR - Jump to Subroutine    
        push16(console, cpu.PC - 1)
        cpu.PC = address
    case 33:
        and()
    case 34: // KIL
    case 35: // RLA
    case 36:
        bit()
    case 37:
        and()
    case 38:
        rol()
    case 39: // RLA
    case 40:
        // PLP - Pull Processor Status
        setFlags(cpu, pull(console)&0xEF | 0x20)
    case 41:
        and()
    case 42:
        rol()
    case 43: // ANC
    case 44:
        bit()
    case 45:
        and()
    case 46:
        rol()
    case 47: // RLA
    case 48:
        // BMI - Branch if Minus
        if cpu.N != 0 {
            cpu.PC = address
            addBranchCycles(cpu, address, pc)
        }
    case 49:
        and()
    case 50: // KIL
    case 51: // RLA
    case 52: // NOP
    case 53:
        and()
    case 54:
        rol()
    case 55: // RLA
    case 56:
        // SEC - Set Carry Flag
        cpu.C = 1
    case 57:
        and()
    case 58: // NOP
    case 59: // RLA
    case 60: // NOP
    case 61:
        and()
    case 62:
        rol()
    case 63: // RLA
    case 64:
        // RTI - Return from Interrupt
        setFlags(cpu, pull(console)&0xEF | 0x20)
        cpu.PC = pull16(console)
    case 65:
        eor()
    case 66: // KIL
    case 67: // SRE
    case 68: // NOP
    case 69:
        eor()
    case 70:
        lsr()
    case 71: // SRE
    case 72:
        // PHA - Push Accumulator
        push(console, cpu.A)
    case 73:
        eor()
    case 74:
        lsr()
    case 75: // ALR
    case 76:
        jmp()
    case 77:
        eor()
    case 78:
        lsr()
    case 79: // SRE
    case 80:
        // BVC - Branch if Overflow Clear
        if cpu.V == 0 {
            cpu.PC = address
            addBranchCycles(cpu, address, pc)
        }
    case 81:
        eor()
    case 82: // KIL
    case 83: // SRE
    case 84: // NOP
    case 85:
        eor()
    case 86:
        lsr()
    case 87: // SRE
    case 88:
        // CLI - Clear Interrupt Disable
        cpu.I = 0
    case 89:
        eor()
    case 90: // NOP
    case 91: // SRE
    case 92: // NOP
    case 93:
        eor()
    case 94:
        lsr()
    case 95: // SRE
    case 96:
        // RTS - Return from Subroutine
        cpu.PC = pull16(console) + 1
    case 97:
        adc()
    case 98: // KIL
    case 99: // RRA
    case 100: // NOP
    case 101:
        adc()
    case 102:
        ror()
    case 103: // RRA
    case 104:
        // PLA - Pull Accumulator
        cpu.A = pull(console)
        setZN(cpu, cpu.A)
    case 105:
        adc()
    case 106:
        ror()
    case 107: // ARR
    case 108:
        jmp()
    case 109:
        adc()
    case 110:
        ror()
    case 111: // RRA
    case 112:
        // BVS - Branch if Overflow Set
        if cpu.V != 0 {
            cpu.PC = address
            addBranchCycles(cpu, address, pc)
        }
    case 113:
        adc()
    case 114: // KIL
    case 115: // RRA
    case 116: // NOP
    case 117:
        adc()
    case 118:
        ror()
    case 119: // RRA
    case 120: // SEI
        sei()
    case 121:
        adc()
    case 122: // NOP
    case 123: // RRA
    case 124: // NOP
    case 125:
        adc()
    case 126:
        ror()
    case 127: // RRA
    case 128: // NOP
    case 129: // STA
        sta()
    case 130: // NOP
    case 131: // SAX
    case 132: // STY
        sty()
    case 133: // STA
        sta()
    case 134: // STX
        stx()
    case 135: // SAX
    case 136:
        // DEY - Decrement Y Register
        cpu.Y--
        setZN(cpu, cpu.Y)
    case 137: // NOP
    case 138: // TXA
        // TXA - Transfer X to Accumulator
        cpu.A = cpu.X
        setZN(cpu, cpu.A)
    case 139: // XAA
    case 140: // STY
        sty()
    case 141: // STA
        sta()
    case 142: // STX
        stx()
    case 143: // SAX
    case 144:
        // BCC - Branch if Carry Clear
        if cpu.C == 0 {
            cpu.PC = address
            addBranchCycles(cpu, address, pc)
        }
    case 145: // STA
        sta()
    case 146: // KIL
    case 147: // AHX
    case 148: // STY
        sty()
    case 149: // STA
        sta()
    case 150: // STX
        stx()
    case 151: // SAX
    case 152: // TYA
        // TYA - Transfer Y to Accumulator
        cpu.A = cpu.Y
        setZN(cpu, cpu.A)
    case 153: // STA
        sta()
    case 154:
        // TXS - Transfer X to Stack Pointer
        cpu.SP = cpu.X
    case 155: // TAS
    case 156: // SHY
    case 157: // STA
        sta()
    case 158: // SHX
    case 159: // AHX
    case 160:
        ldy()
    case 161:
        lda()
    case 162:
        ldx()
    case 163: // LAX
    case 164:
        ldy()
    case 165:
        lda()
    case 166:
        ldx()
    case 167: // LAX
    case 168:
        // TAY - Transfer Accumulator to Y
        cpu.Y = cpu.A
        setZN(cpu, cpu.Y)
    case 169:
        lda()
    case 170:
        // TAX - Transfer Accumulator to X
        cpu.X = cpu.A
        setZN(cpu, cpu.X)
    case 171: // LAX
    case 172:
        ldy()
    case 173:
        lda()
    case 174:
        ldx()
    case 175: // LAX
    case 176:
        // BCS - Branch if Carry Set
        if cpu.C != 0 {
            cpu.PC = address
            addBranchCycles(cpu, address, pc)
        }
    case 177:
        lda()
    case 178: // KIL
    case 179: // LAX
    case 180:
        ldy()
    case 181:
        lda()
    case 182:
        ldx()
    case 183: // LAX
    case 184:
        // CLV - Clear Overflow Flag
        cpu.V = 0
    case 185:
        lda()
    case 186:
        // TSX - Transfer Stack Pointer to X    
        cpu.X = cpu.SP
        setZN(cpu, cpu.X)
    case 187: // LAS
    case 188:
        ldy()
    case 189:
        lda()
    case 190:
        ldx()
    case 191: // LAX
    case 192:
        cpy()
    case 193:
        cmp()
    case 194: // NOP
    case 195: // DCP
    case 196:
        cpy()
    case 197:
        cmp()
    case 198:
        dec()
    case 199: // DCP
    case 200:
        // INY - Increment Y Register
        cpu.Y++
        setZN(cpu, cpu.Y)
    case 201:
        cmp()
    case 202:
        // DEX - Decrement X Register
        cpu.X--
        setZN(cpu, cpu.X)
    case 203: // AXS
    case 204:
        cpy()
    case 205:
        cmp()
    case 206:
        dec()
    case 207: // DCP
    case 208:
        // BNE - Branch if Not Equal
        if cpu.Z == 0 {
            cpu.PC = address
            addBranchCycles(cpu, address, pc)
        }
    case 209:
        cmp()
    case 210: // KIL
    case 211: // DCP
    case 212: // NOP
    case 213:
        cmp()
    case 214:
        dec()
    case 215: // DCP
    case 216:
        // CLD - Clear Decimal Mode
        cpu.D = 0
    case 217:
        cmp()
    case 218: // NOP
    case 219: // DCP
    case 220: // NOP
    case 221:
        cmp()
    case 222:
        dec()
    case 223: // DCP
    case 224:
        cpx()
    case 225:
        sbc()
    case 226: // NOP
    case 227: // ISC
    case 228:
        cpx()
    case 229:
        sbc()
    case 230:
        inc()
    case 231: // ISC
    case 232:
        // INX - Increment X Register
        cpu.X++
        setZN(cpu, cpu.X)
    case 233:
        sbc()
    case 234: // NOP
    case 235:
        sbc()
    case 236:
        cpx()
    case 237:
        sbc()
    case 238:
        inc()
    case 239: // ISC
    case 240:
        // BEQ - Branch if Equal
        if cpu.Z != 0 {
            cpu.PC = address
            addBranchCycles(cpu, address, pc)
        }
    case 241:
        sbc()
    case 242: // KIL
    case 243: // ISC
    case 244: // NOP
    case 245:
        sbc()
    case 246:
        inc()
    case 247: // ISC
    case 248:
        // SED - Set Decimal Flag
        cpu.D = 1
    case 249:
        sbc()
    case 250: // NOP
    case 251: // ISC
    case 252: // NOP
    case 253:
        sbc()
    case 254:
        inc()
    case 255: // ISC

    }

}


// Reset resets the CPU to its initial powerup state
func Reset(console *Console) {
    cpu := console.CPU
    cpu.PC = read16(console, 0xFFFC)
    cpu.SP = 0xFD
    setFlags(cpu, 0x24)
}

// instruction helper functions

func compare(cpu *CPU, a, b byte) {
    setZN(cpu, a - b)
    if a >= b {
        cpu.C = 1
    } else {
        cpu.C = 0
    }
}

// addBranchCycles adds a cycle for taking a branch and adds another cycle
// if the branch jumps to a new page
func addBranchCycles(cpu *CPU, address uint16, pc uint16) {
    cpu.Cycles++
    if pagesDiffer(pc, address) {
        cpu.Cycles++
    }
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

// read16 reads two bytes using Read to return a double-word value
func read16(console *Console, address uint16) uint16 {
    lo := uint16(ReadByte(console, address))
    hi := uint16(ReadByte(console, address + 1))
    return hi<<8 | lo
}

// pagesDiffer returns true if the two addresses reference different pages
func pagesDiffer(a, b uint16) bool {
    return a&0xFF00 != b&0xFF00
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

// sets the processor status flags
func setFlags(cpu *CPU, flags byte) {
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

// PHP - Push Processor Status
func php(console *Console) {
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