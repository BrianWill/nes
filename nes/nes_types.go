package nes

import (
    "image/color"
    "image"
)

type APU struct {
    console     *Console
    channel     chan float32
    pulse1      Pulse
    pulse2      Pulse
    triangle    Triangle
    noise       Noise
    dmc         DMC
    cycle       uint64
    framePeriod byte
    frameValue  byte
    frameIRQ    bool
}

// Delta Modulation Channel
type DMC struct {
    cpu            *CPU
    enabled        bool
    value          byte
    sampleAddress  uint16
    sampleLength   uint16
    currentAddress uint16
    currentLength  uint16
    shiftRegister  byte
    bitCount       byte
    tickPeriod     byte
    tickValue      byte
    loop           bool
    irq            bool
}

type Pulse struct {
    enabled         bool
    channel         byte
    lengthEnabled   bool
    lengthValue     byte
    timerPeriod     uint16
    timerValue      uint16
    dutyMode        byte
    dutyValue       byte
    sweepReload     bool
    sweepEnabled    bool
    sweepNegate     bool
    sweepShift      byte
    sweepPeriod     byte
    sweepValue      byte
    envelopeEnabled bool
    envelopeLoop    bool
    envelopeStart   bool
    envelopePeriod  byte
    envelopeValue   byte
    envelopeVolume  byte
    constantVolume  byte
}

type Triangle struct {
    enabled       bool
    lengthEnabled bool
    lengthValue   byte
    timerPeriod   uint16
    timerValue    uint16
    dutyValue     byte
    counterPeriod byte
    counterValue  byte
    counterReload bool
}

type Noise struct {
    enabled         bool
    mode            bool
    shiftRegister   uint16
    lengthEnabled   bool
    lengthValue     byte
    timerPeriod     uint16
    timerValue      uint16
    envelopeEnabled bool
    envelopeLoop    bool
    envelopeStart   bool
    envelopePeriod  byte
    envelopeValue   byte
    envelopeVolume  byte
    constantVolume  byte
}

type Cartridge struct {
    PRG []byte // PRG-ROM banks
    CHR []byte // CHR-ROM banks
    SRAM []byte // Save RAM
    Mapper byte   // mapper type
    Mirror byte   // mirroring mode
    Battery byte   // battery present
}

type Console struct {
    CPU *CPU
    APU *APU
    PPU *PPU
    Cartridge *Cartridge
    Controller1 *Controller
    Controller2 *Controller
    Mapper Mapper
    RAM []byte
}

const (
    ButtonA = iota
    ButtonB
    ButtonSelect
    ButtonStart
    ButtonUp
    ButtonDown
    ButtonLeft
    ButtonRight
)

type Controller struct {
    buttons [8]bool
    index   byte
    strobe  byte
}

type CPU struct {
    Cycles    uint64 // number of cycles
    PC        uint16 // program counter
    SP        byte   // stack pointer
    A         byte   // accumulator
    X         byte   // x register
    Y         byte   // y register
    C         byte   // carry flag
    Z         byte   // zero flag
    I         byte   // interrupt disable flag
    D         byte   // decimal mode flag
    B         byte   // break command flag
    U         byte   // unused flag
    V         byte   // overflow flag
    N         byte   // negative flag
    interrupt byte   // interrupt type to perform
    stall     int    // number of cycles to stall
}

type PPU struct {
    console *Console // reference to parent object

    Cycle    int    // 0-340
    ScanLine int    // 0-261, 0-239=visible, 240=post, 241-260=vblank, 261=pre
    Frame    uint64 // frame counter

    // storage variables
    paletteData   [32]byte
    nameTableData [2048]byte
    oamData       [256]byte
    front         *image.RGBA
    back          *image.RGBA

    // PPU registers
    v uint16 // current vram address (15 bit)
    t uint16 // temporary vram address (15 bit)
    x byte   // fine x scroll (3 bit)
    w byte   // write toggle (1 bit)
    f byte   // even/odd frame flag (1 bit)

    register byte

    // NMI flags
    nmiOccurred bool
    nmiOutput   bool
    nmiPrevious bool
    nmiDelay    byte

    // background temporary variables
    nameTableByte      byte
    attributeTableByte byte
    lowTileByte        byte
    highTileByte       byte
    tileData           uint64

    // sprite temporary variables
    spriteCount      int
    spritePatterns   [8]uint32
    spritePositions  [8]byte
    spritePriorities [8]byte
    spriteIndexes    [8]byte

    // $2000 PPUCTRL
    flagNameTable       byte // 0: $2000; 1: $2400; 2: $2800; 3: $2C00
    flagIncrement       byte // 0: add 1; 1: add 32
    flagSpriteTable     byte // 0: $0000; 1: $1000; ignored in 8x16 mode
    flagBackgroundTable byte // 0: $0000; 1: $1000
    flagSpriteSize      byte // 0: 8x8; 1: 8x16
    flagMasterSlave     byte // 0: read EXT; 1: write EXT

    // $2001 PPUMASK
    flagGrayscale          byte // 0: color; 1: grayscale
    flagShowLeftBackground byte // 0: hide; 1: show
    flagShowLeftSprites    byte // 0: hide; 1: show
    flagShowBackground     byte // 0: hide; 1: show
    flagShowSprites        byte // 0: hide; 1: show
    flagRedTint            byte // 0: normal; 1: emphasized
    flagGreenTint          byte // 0: normal; 1: emphasized
    flagBlueTint           byte // 0: normal; 1: emphasized

    // $2002 PPUSTATUS
    flagSpriteZeroHit  byte
    flagSpriteOverflow byte

    // $2003 OAMADDR
    oamAddress byte

    // $2007 PPUDATA
    bufferedData byte // for buffered reads
}

type Mapper interface {
    Mapper()
}

func (_ *Mapper1) Mapper() {}
func (_ *Mapper2) Mapper() {}
func (_ *Mapper3) Mapper() {}
func (_ *Mapper4) Mapper() {}
func (_ *Mapper7) Mapper() {}

type Mapper1 struct {
    *Cartridge
    shiftRegister byte
    control       byte
    prgMode       byte
    chrMode       byte
    prgBank       byte
    chrBank0      byte
    chrBank1      byte
    prgOffsets    [2]int
    chrOffsets    [2]int
}

type Mapper2 struct {
    *Cartridge
    prgBanks int
    prgBank1 int
    prgBank2 int
}

type Mapper3 struct {
    *Cartridge
    chrBank  int
    prgBank1 int
    prgBank2 int
}

type Mapper4 struct {
    *Cartridge
    console    *Console    // btw: shouldn't have a console! needless recursion: console has mapper and mapper has console
    register   byte
    registers  [8]byte
    prgMode    byte
    chrMode    byte
    prgOffsets [4]int
    chrOffsets [8]int
    reload     byte
    counter    byte
    irqEnable  bool
}

type Mapper7 struct {
    *Cartridge
    prgBank int
}

type iNESFileHeader struct {
    Magic uint32  // iNES magic number
    NumPRG byte   // number of PRG-ROM banks (16KB each)
    NumCHR byte   // number of CHR-ROM banks (8KB each)
    Control1 byte // control bits
    Control2 byte // control bits
    NumRAM byte   // PRG-RAM size (x 8KB)
    _ [7]byte     // unused padding (necessary for properly reading ROM file)
}

type Instruction struct {
    Opcode byte
    Name string
    Function func (*Console, uint16, uint16, byte)
    Mode byte        // the addressing mode
    Size byte        // the size in bytes
    Cycles byte      // the number of cycles used (not including conditional cycles)
    PageCycles byte  // the number of cycles used when a page is crossed
}

const iNESFileMagic = 0x1a53454e

var pulseTable [31]float32
var tndTable [203]float32

var Palette [64]color.RGBA

const frameCounterRate = CPUFrequency / 240.0
const sampleRate = CPUFrequency / 44100.0 / 2

var lengthTable = []byte{
    10, 254, 20, 2, 40, 4, 80, 6, 160, 8, 60, 10, 14, 12, 26, 14,
    12, 16, 24, 18, 48, 20, 96, 22, 192, 24, 72, 26, 16, 28, 32, 30,
}

var dutyTable = [][]byte{
    {0, 1, 0, 0, 0, 0, 0, 0},
    {0, 1, 1, 0, 0, 0, 0, 0},
    {0, 1, 1, 1, 1, 0, 0, 0},
    {1, 0, 0, 1, 1, 1, 1, 1},
}

var triangleTable = []byte{
    15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0,
    0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
}

var noiseTable = []uint16{
    4, 8, 16, 32, 64, 96, 128, 160, 202, 254, 380, 508, 762, 1016, 2034, 4068,
}

var dmcTable = []byte{
    214, 190, 170, 160, 143, 127, 113, 107, 95, 80, 71, 64, 53, 42, 36, 27,
}

const CPUFrequency = 1789773

// interrupt types
const (
    _ = iota
    interruptNone
    interruptNMI
    interruptIRQ
)

// addressing modes
const (
    _ = iota
    modeAbsolute
    modeAbsoluteX
    modeAbsoluteY
    modeAccumulator
    modeImmediate
    modeImplied
    modeIndexedIndirect
    modeIndirect
    modeIndirectIndexed
    modeRelative
    modeZeroPage
    modeZeroPageX
    modeZeroPageY
)

// probably more efficient to put these data into parallel arrays, but seems like a non-issue for now
var instructions = [256]Instruction{
    Instruction{Opcode: 0, Name: "BRK", Function: brk, Mode: 6, Size: 1, Cycles: 7, PageCycles: 0},
    Instruction{Opcode: 1, Name: "ORA", Function: ora, Mode: 7, Size: 2, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 2, Name: "KIL", Function: kil, Mode: 6, Size: 0, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 3, Name: "SLO", Function: slo, Mode: 7, Size: 0, Cycles: 8, PageCycles: 0},
    Instruction{Opcode: 4, Name: "NOP", Function: nop, Mode: 11, Size: 2, Cycles: 3, PageCycles: 0},
    Instruction{Opcode: 5, Name: "ORA", Function: ora, Mode: 11, Size: 2, Cycles: 3, PageCycles: 0},
    Instruction{Opcode: 6, Name: "ASL", Function: asl, Mode: 11, Size: 2, Cycles: 5, PageCycles: 0},
    Instruction{Opcode: 7, Name: "SLO", Function: slo, Mode: 11, Size: 0, Cycles: 5, PageCycles: 0},
    Instruction{Opcode: 8, Name: "PHP", Function: php, Mode: 6, Size: 1, Cycles: 3, PageCycles: 0},
    Instruction{Opcode: 9, Name: "ORA", Function: ora, Mode: 5, Size: 2, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 10, Name: "ASL", Function: asl, Mode: 4, Size: 1, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 11, Name: "ANC", Function: anc, Mode: 5, Size: 0, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 12, Name: "NOP", Function: nop, Mode: 1, Size: 3, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 13, Name: "ORA", Function: ora, Mode: 1, Size: 3, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 14, Name: "ASL", Function: asl, Mode: 1, Size: 3, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 15, Name: "SLO", Function: slo, Mode: 1, Size: 0, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 16, Name: "BPL", Function: bpl, Mode: 10, Size: 2, Cycles: 2, PageCycles: 1},
    Instruction{Opcode: 17, Name: "ORA", Function: ora, Mode: 9, Size: 2, Cycles: 5, PageCycles: 1},
    Instruction{Opcode: 18, Name: "KIL", Function: kil, Mode: 6, Size: 0, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 19, Name: "SLO", Function: slo, Mode: 9, Size: 0, Cycles: 8, PageCycles: 0},
    Instruction{Opcode: 20, Name: "NOP", Function: nop, Mode: 12, Size: 2, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 21, Name: "ORA", Function: ora, Mode: 12, Size: 2, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 22, Name: "ASL", Function: asl, Mode: 12, Size: 2, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 23, Name: "SLO", Function: slo, Mode: 12, Size: 0, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 24, Name: "CLC", Function: clc, Mode: 6, Size: 1, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 25, Name: "ORA", Function: ora, Mode: 3, Size: 3, Cycles: 4, PageCycles: 1},
    Instruction{Opcode: 26, Name: "NOP", Function: nop, Mode: 6, Size: 1, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 27, Name: "SLO", Function: slo, Mode: 3, Size: 0, Cycles: 7, PageCycles: 0},
    Instruction{Opcode: 28, Name: "NOP", Function: nop, Mode: 2, Size: 3, Cycles: 4, PageCycles: 1},
    Instruction{Opcode: 29, Name: "ORA", Function: ora, Mode: 2, Size: 3, Cycles: 4, PageCycles: 1},
    Instruction{Opcode: 30, Name: "ASL", Function: asl, Mode: 2, Size: 3, Cycles: 7, PageCycles: 0},
    Instruction{Opcode: 31, Name: "SLO", Function: slo, Mode: 2, Size: 0, Cycles: 7, PageCycles: 0},
    Instruction{Opcode: 32, Name: "JSR", Function: jsr, Mode: 1, Size: 3, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 33, Name: "AND", Function: and, Mode: 7, Size: 2, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 34, Name: "KIL", Function: kil, Mode: 6, Size: 0, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 35, Name: "RLA", Function: rla, Mode: 7, Size: 0, Cycles: 8, PageCycles: 0},
    Instruction{Opcode: 36, Name: "BIT", Function: bit, Mode: 11, Size: 2, Cycles: 3, PageCycles: 0},
    Instruction{Opcode: 37, Name: "AND", Function: and, Mode: 11, Size: 2, Cycles: 3, PageCycles: 0},
    Instruction{Opcode: 38, Name: "ROL", Function: rol, Mode: 11, Size: 2, Cycles: 5, PageCycles: 0},
    Instruction{Opcode: 39, Name: "RLA", Function: rla, Mode: 11, Size: 0, Cycles: 5, PageCycles: 0},
    Instruction{Opcode: 40, Name: "PLP", Function: plp, Mode: 6, Size: 1, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 41, Name: "AND", Function: and, Mode: 5, Size: 2, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 42, Name: "ROL", Function: rol, Mode: 4, Size: 1, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 43, Name: "ANC", Function: anc, Mode: 5, Size: 0, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 44, Name: "BIT", Function: bit, Mode: 1, Size: 3, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 45, Name: "AND", Function: and, Mode: 1, Size: 3, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 46, Name: "ROL", Function: rol, Mode: 1, Size: 3, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 47, Name: "RLA", Function: rla, Mode: 1, Size: 0, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 48, Name: "BMI", Function: bmi, Mode: 10, Size: 2, Cycles: 2, PageCycles: 1},
    Instruction{Opcode: 49, Name: "AND", Function: and, Mode: 9, Size: 2, Cycles: 5, PageCycles: 1},
    Instruction{Opcode: 50, Name: "KIL", Function: kil, Mode: 6, Size: 0, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 51, Name: "RLA", Function: rla, Mode: 9, Size: 0, Cycles: 8, PageCycles: 0},
    Instruction{Opcode: 52, Name: "NOP", Function: nop, Mode: 12, Size: 2, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 53, Name: "AND", Function: and, Mode: 12, Size: 2, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 54, Name: "ROL", Function: rol, Mode: 12, Size: 2, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 55, Name: "RLA", Function: rla, Mode: 12, Size: 0, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 56, Name: "SEC", Function: sec, Mode: 6, Size: 1, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 57, Name: "AND", Function: and, Mode: 3, Size: 3, Cycles: 4, PageCycles: 1},
    Instruction{Opcode: 58, Name: "NOP", Function: nop, Mode: 6, Size: 1, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 59, Name: "RLA", Function: rla, Mode: 3, Size: 0, Cycles: 7, PageCycles: 0},
    Instruction{Opcode: 60, Name: "NOP", Function: nop, Mode: 2, Size: 3, Cycles: 4, PageCycles: 1},
    Instruction{Opcode: 61, Name: "AND", Function: and, Mode: 2, Size: 3, Cycles: 4, PageCycles: 1},
    Instruction{Opcode: 62, Name: "ROL", Function: rol, Mode: 2, Size: 3, Cycles: 7, PageCycles: 0},
    Instruction{Opcode: 63, Name: "RLA", Function: rla, Mode: 2, Size: 0, Cycles: 7, PageCycles: 0},
    Instruction{Opcode: 64, Name: "RTI", Function: rti, Mode: 6, Size: 1, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 65, Name: "EOR", Function: eor, Mode: 7, Size: 2, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 66, Name: "KIL", Function: kil, Mode: 6, Size: 0, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 67, Name: "SRE", Function: sre, Mode: 7, Size: 0, Cycles: 8, PageCycles: 0},
    Instruction{Opcode: 68, Name: "NOP", Function: nop, Mode: 11, Size: 2, Cycles: 3, PageCycles: 0},
    Instruction{Opcode: 69, Name: "EOR", Function: eor, Mode: 11, Size: 2, Cycles: 3, PageCycles: 0},
    Instruction{Opcode: 70, Name: "LSR", Function: lsr, Mode: 11, Size: 2, Cycles: 5, PageCycles: 0},
    Instruction{Opcode: 71, Name: "SRE", Function: sre, Mode: 11, Size: 0, Cycles: 5, PageCycles: 0},
    Instruction{Opcode: 72, Name: "PHA", Function: pha, Mode: 6, Size: 1, Cycles: 3, PageCycles: 0},
    Instruction{Opcode: 73, Name: "EOR", Function: eor, Mode: 5, Size: 2, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 74, Name: "LSR", Function: lsr, Mode: 4, Size: 1, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 75, Name: "ALR", Function: alr, Mode: 5, Size: 0, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 76, Name: "JMP", Function: jmp, Mode: 1, Size: 3, Cycles: 3, PageCycles: 0},
    Instruction{Opcode: 77, Name: "EOR", Function: eor, Mode: 1, Size: 3, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 78, Name: "LSR", Function: lsr, Mode: 1, Size: 3, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 79, Name: "SRE", Function: sre, Mode: 1, Size: 0, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 80, Name: "BVC", Function: bvc, Mode: 10, Size: 2, Cycles: 2, PageCycles: 1},
    Instruction{Opcode: 81, Name: "EOR", Function: eor, Mode: 9, Size: 2, Cycles: 5, PageCycles: 1},
    Instruction{Opcode: 82, Name: "KIL", Function: kil, Mode: 6, Size: 0, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 83, Name: "SRE", Function: sre, Mode: 9, Size: 0, Cycles: 8, PageCycles: 0},
    Instruction{Opcode: 84, Name: "NOP", Function: nop, Mode: 12, Size: 2, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 85, Name: "EOR", Function: eor, Mode: 12, Size: 2, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 86, Name: "LSR", Function: lsr, Mode: 12, Size: 2, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 87, Name: "SRE", Function: sre, Mode: 12, Size: 0, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 88, Name: "CLI", Function: cli, Mode: 6, Size: 1, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 89, Name: "EOR", Function: eor, Mode: 3, Size: 3, Cycles: 4, PageCycles: 1},
    Instruction{Opcode: 90, Name: "NOP", Function: nop, Mode: 6, Size: 1, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 91, Name: "SRE", Function: sre, Mode: 3, Size: 0, Cycles: 7, PageCycles: 0},
    Instruction{Opcode: 92, Name: "NOP", Function: nop, Mode: 2, Size: 3, Cycles: 4, PageCycles: 1},
    Instruction{Opcode: 93, Name: "EOR", Function: eor, Mode: 2, Size: 3, Cycles: 4, PageCycles: 1},
    Instruction{Opcode: 94, Name: "LSR", Function: lsr, Mode: 2, Size: 3, Cycles: 7, PageCycles: 0},
    Instruction{Opcode: 95, Name: "SRE", Function: sre, Mode: 2, Size: 0, Cycles: 7, PageCycles: 0},
    Instruction{Opcode: 96, Name: "RTS", Function: rts, Mode: 6, Size: 1, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 97, Name: "ADC", Function: adc, Mode: 7, Size: 2, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 98, Name: "KIL", Function: kil, Mode: 6, Size: 0, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 99, Name: "RRA", Function: rra, Mode: 7, Size: 0, Cycles: 8, PageCycles: 0},
    Instruction{Opcode: 100, Name: "NOP", Function: nop, Mode: 11, Size: 2, Cycles: 3, PageCycles: 0},
    Instruction{Opcode: 101, Name: "ADC", Function: adc, Mode: 11, Size: 2, Cycles: 3, PageCycles: 0},
    Instruction{Opcode: 102, Name: "ROR", Function: ror, Mode: 11, Size: 2, Cycles: 5, PageCycles: 0},
    Instruction{Opcode: 103, Name: "RRA", Function: rra, Mode: 11, Size: 0, Cycles: 5, PageCycles: 0},
    Instruction{Opcode: 104, Name: "PLA", Function: pla, Mode: 6, Size: 1, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 105, Name: "ADC", Function: adc, Mode: 5, Size: 2, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 106, Name: "ROR", Function: ror, Mode: 4, Size: 1, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 107, Name: "ARR", Function: arr, Mode: 5, Size: 0, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 108, Name: "JMP", Function: jmp, Mode: 8, Size: 3, Cycles: 5, PageCycles: 0},
    Instruction{Opcode: 109, Name: "ADC", Function: adc, Mode: 1, Size: 3, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 110, Name: "ROR", Function: ror, Mode: 1, Size: 3, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 111, Name: "RRA", Function: rra, Mode: 1, Size: 0, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 112, Name: "BVS", Function: bvs, Mode: 10, Size: 2, Cycles: 2, PageCycles: 1},
    Instruction{Opcode: 113, Name: "ADC", Function: adc, Mode: 9, Size: 2, Cycles: 5, PageCycles: 1},
    Instruction{Opcode: 114, Name: "KIL", Function: kil, Mode: 6, Size: 0, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 115, Name: "RRA", Function: rra, Mode: 9, Size: 0, Cycles: 8, PageCycles: 0},
    Instruction{Opcode: 116, Name: "NOP", Function: nop, Mode: 12, Size: 2, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 117, Name: "ADC", Function: adc, Mode: 12, Size: 2, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 118, Name: "ROR", Function: ror, Mode: 12, Size: 2, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 119, Name: "RRA", Function: rra, Mode: 12, Size: 0, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 120, Name: "SEI", Function: sei, Mode: 6, Size: 1, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 121, Name: "ADC", Function: adc, Mode: 3, Size: 3, Cycles: 4, PageCycles: 1},
    Instruction{Opcode: 122, Name: "NOP", Function: nop, Mode: 6, Size: 1, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 123, Name: "RRA", Function: rra, Mode: 3, Size: 0, Cycles: 7, PageCycles: 0},
    Instruction{Opcode: 124, Name: "NOP", Function: nop, Mode: 2, Size: 3, Cycles: 4, PageCycles: 1},
    Instruction{Opcode: 125, Name: "ADC", Function: adc, Mode: 2, Size: 3, Cycles: 4, PageCycles: 1},
    Instruction{Opcode: 126, Name: "ROR", Function: ror, Mode: 2, Size: 3, Cycles: 7, PageCycles: 0},
    Instruction{Opcode: 127, Name: "RRA", Function: rra, Mode: 2, Size: 0, Cycles: 7, PageCycles: 0},
    Instruction{Opcode: 128, Name: "NOP", Function: nop, Mode: 5, Size: 2, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 129, Name: "STA", Function: sta, Mode: 7, Size: 2, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 130, Name: "NOP", Function: nop, Mode: 5, Size: 0, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 131, Name: "SAX", Function: sax, Mode: 7, Size: 0, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 132, Name: "STY", Function: sty, Mode: 11, Size: 2, Cycles: 3, PageCycles: 0},
    Instruction{Opcode: 133, Name: "STA", Function: sta, Mode: 11, Size: 2, Cycles: 3, PageCycles: 0},
    Instruction{Opcode: 134, Name: "STX", Function: stx, Mode: 11, Size: 2, Cycles: 3, PageCycles: 0},
    Instruction{Opcode: 135, Name: "SAX", Function: sax, Mode: 11, Size: 0, Cycles: 3, PageCycles: 0},
    Instruction{Opcode: 136, Name: "DEY", Function: dey, Mode: 6, Size: 1, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 137, Name: "NOP", Function: nop, Mode: 5, Size: 0, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 138, Name: "TXA", Function: txa, Mode: 6, Size: 1, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 139, Name: "XAA", Function: xaa, Mode: 5, Size: 0, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 140, Name: "STY", Function: sty, Mode: 1, Size: 3, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 141, Name: "STA", Function: sta, Mode: 1, Size: 3, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 142, Name: "STX", Function: stx, Mode: 1, Size: 3, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 143, Name: "SAX", Function: sax, Mode: 1, Size: 0, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 144, Name: "BCC", Function: bcc, Mode: 10, Size: 2, Cycles: 2, PageCycles: 1},
    Instruction{Opcode: 145, Name: "STA", Function: sta, Mode: 9, Size: 2, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 146, Name: "KIL", Function: kil, Mode: 6, Size: 0, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 147, Name: "AHX", Function: ahx, Mode: 9, Size: 0, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 148, Name: "STY", Function: sty, Mode: 12, Size: 2, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 149, Name: "STA", Function: sta, Mode: 12, Size: 2, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 150, Name: "STX", Function: stx, Mode: 13, Size: 2, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 151, Name: "SAX", Function: sax, Mode: 13, Size: 0, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 152, Name: "TYA", Function: tya, Mode: 6, Size: 1, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 153, Name: "STA", Function: sta, Mode: 3, Size: 3, Cycles: 5, PageCycles: 0},
    Instruction{Opcode: 154, Name: "TXS", Function: txs, Mode: 6, Size: 1, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 155, Name: "TAS", Function: tas, Mode: 3, Size: 0, Cycles: 5, PageCycles: 0},
    Instruction{Opcode: 156, Name: "SHY", Function: shy, Mode: 2, Size: 0, Cycles: 5, PageCycles: 0},
    Instruction{Opcode: 157, Name: "STA", Function: sta, Mode: 2, Size: 3, Cycles: 5, PageCycles: 0},
    Instruction{Opcode: 158, Name: "SHX", Function: shx, Mode: 3, Size: 0, Cycles: 5, PageCycles: 0},
    Instruction{Opcode: 159, Name: "AHX", Function: ahx, Mode: 3, Size: 0, Cycles: 5, PageCycles: 0},
    Instruction{Opcode: 160, Name: "LDY", Function: ldy, Mode: 5, Size: 2, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 161, Name: "LDA", Function: lda, Mode: 7, Size: 2, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 162, Name: "LDX", Function: ldx, Mode: 5, Size: 2, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 163, Name: "LAX", Function: lax, Mode: 7, Size: 0, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 164, Name: "LDY", Function: ldy, Mode: 11, Size: 2, Cycles: 3, PageCycles: 0},
    Instruction{Opcode: 165, Name: "LDA", Function: lda, Mode: 11, Size: 2, Cycles: 3, PageCycles: 0},
    Instruction{Opcode: 166, Name: "LDX", Function: ldx, Mode: 11, Size: 2, Cycles: 3, PageCycles: 0},
    Instruction{Opcode: 167, Name: "LAX", Function: lax, Mode: 11, Size: 0, Cycles: 3, PageCycles: 0},
    Instruction{Opcode: 168, Name: "TAY", Function: tay, Mode: 6, Size: 1, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 169, Name: "LDA", Function: lda, Mode: 5, Size: 2, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 170, Name: "TAX", Function: tax, Mode: 6, Size: 1, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 171, Name: "LAX", Function: lax, Mode: 5, Size: 0, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 172, Name: "LDY", Function: ldy, Mode: 1, Size: 3, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 173, Name: "LDA", Function: lda, Mode: 1, Size: 3, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 174, Name: "LDX", Function: ldx, Mode: 1, Size: 3, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 175, Name: "LAX", Function: lax, Mode: 1, Size: 0, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 176, Name: "BCS", Function: bcs, Mode: 10, Size: 2, Cycles: 2, PageCycles: 1},
    Instruction{Opcode: 177, Name: "LDA", Function: lda, Mode: 9, Size: 2, Cycles: 5, PageCycles: 1},
    Instruction{Opcode: 178, Name: "KIL", Function: kil, Mode: 6, Size: 0, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 179, Name: "LAX", Function: lax, Mode: 9, Size: 0, Cycles: 5, PageCycles: 1},
    Instruction{Opcode: 180, Name: "LDY", Function: ldy, Mode: 12, Size: 2, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 181, Name: "LDA", Function: lda, Mode: 12, Size: 2, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 182, Name: "LDX", Function: ldx, Mode: 13, Size: 2, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 183, Name: "LAX", Function: lax, Mode: 13, Size: 0, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 184, Name: "CLV", Function: clv, Mode: 6, Size: 1, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 185, Name: "LDA", Function: lda, Mode: 3, Size: 3, Cycles: 4, PageCycles: 1},
    Instruction{Opcode: 186, Name: "TSX", Function: tsx, Mode: 6, Size: 1, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 187, Name: "LAS", Function: las, Mode: 3, Size: 0, Cycles: 4, PageCycles: 1},
    Instruction{Opcode: 188, Name: "LDY", Function: ldy, Mode: 2, Size: 3, Cycles: 4, PageCycles: 1},
    Instruction{Opcode: 189, Name: "LDA", Function: lda, Mode: 2, Size: 3, Cycles: 4, PageCycles: 1},
    Instruction{Opcode: 190, Name: "LDX", Function: ldx, Mode: 3, Size: 3, Cycles: 4, PageCycles: 1},
    Instruction{Opcode: 191, Name: "LAX", Function: lax, Mode: 3, Size: 0, Cycles: 4, PageCycles: 1},
    Instruction{Opcode: 192, Name: "CPY", Function: cpy, Mode: 5, Size: 2, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 193, Name: "CMP", Function: cmp, Mode: 7, Size: 2, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 194, Name: "NOP", Function: nop, Mode: 5, Size: 0, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 195, Name: "DCP", Function: dcp, Mode: 7, Size: 0, Cycles: 8, PageCycles: 0},
    Instruction{Opcode: 196, Name: "CPY", Function: cpy, Mode: 11, Size: 2, Cycles: 3, PageCycles: 0},
    Instruction{Opcode: 197, Name: "CMP", Function: cmp, Mode: 11, Size: 2, Cycles: 3, PageCycles: 0},
    Instruction{Opcode: 198, Name: "DEC", Function: dec, Mode: 11, Size: 2, Cycles: 5, PageCycles: 0},
    Instruction{Opcode: 199, Name: "DCP", Function: dcp, Mode: 11, Size: 0, Cycles: 5, PageCycles: 0},
    Instruction{Opcode: 200, Name: "INY", Function: iny, Mode: 6, Size: 1, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 201, Name: "CMP", Function: cmp, Mode: 5, Size: 2, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 202, Name: "DEX", Function: dex, Mode: 6, Size: 1, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 203, Name: "AXS", Function: axs, Mode: 5, Size: 0, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 204, Name: "CPY", Function: cpy, Mode: 1, Size: 3, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 205, Name: "CMP", Function: cmp, Mode: 1, Size: 3, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 206, Name: "DEC", Function: dec, Mode: 1, Size: 3, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 207, Name: "DCP", Function: dcp, Mode: 1, Size: 0, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 208, Name: "BNE", Function: bne, Mode: 10, Size: 2, Cycles: 2, PageCycles: 1},
    Instruction{Opcode: 209, Name: "CMP", Function: cmp, Mode: 9, Size: 2, Cycles: 5, PageCycles: 1},
    Instruction{Opcode: 210, Name: "KIL", Function: kil, Mode: 6, Size: 0, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 211, Name: "DCP", Function: dcp, Mode: 9, Size: 0, Cycles: 8, PageCycles: 0},
    Instruction{Opcode: 212, Name: "NOP", Function: nop, Mode: 12, Size: 2, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 213, Name: "CMP", Function: cmp, Mode: 12, Size: 2, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 214, Name: "DEC", Function: dec, Mode: 12, Size: 2, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 215, Name: "DCP", Function: dcp, Mode: 12, Size: 0, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 216, Name: "CLD", Function: cld, Mode: 6, Size: 1, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 217, Name: "CMP", Function: cmp, Mode: 3, Size: 3, Cycles: 4, PageCycles: 1},
    Instruction{Opcode: 218, Name: "NOP", Function: nop, Mode: 6, Size: 1, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 219, Name: "DCP", Function: dcp, Mode: 3, Size: 0, Cycles: 7, PageCycles: 0},
    Instruction{Opcode: 220, Name: "NOP", Function: nop, Mode: 2, Size: 3, Cycles: 4, PageCycles: 1},
    Instruction{Opcode: 221, Name: "CMP", Function: cmp, Mode: 2, Size: 3, Cycles: 4, PageCycles: 1},
    Instruction{Opcode: 222, Name: "DEC", Function: dec, Mode: 2, Size: 3, Cycles: 7, PageCycles: 0},
    Instruction{Opcode: 223, Name: "DCP", Function: dcp, Mode: 2, Size: 0, Cycles: 7, PageCycles: 0},
    Instruction{Opcode: 224, Name: "CPX", Function: cpx, Mode: 5, Size: 2, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 225, Name: "SBC", Function: sbc, Mode: 7, Size: 2, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 226, Name: "NOP", Function: nop, Mode: 5, Size: 0, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 227, Name: "ISC", Function: isc, Mode: 7, Size: 0, Cycles: 8, PageCycles: 0},
    Instruction{Opcode: 228, Name: "CPX", Function: cpx, Mode: 11, Size: 2, Cycles: 3, PageCycles: 0},
    Instruction{Opcode: 229, Name: "SBC", Function: sbc, Mode: 11, Size: 2, Cycles: 3, PageCycles: 0},
    Instruction{Opcode: 230, Name: "INC", Function: inc, Mode: 11, Size: 2, Cycles: 5, PageCycles: 0},
    Instruction{Opcode: 231, Name: "ISC", Function: isc, Mode: 11, Size: 0, Cycles: 5, PageCycles: 0},
    Instruction{Opcode: 232, Name: "INX", Function: inx, Mode: 6, Size: 1, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 233, Name: "SBC", Function: sbc, Mode: 5, Size: 2, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 234, Name: "NOP", Function: nop, Mode: 6, Size: 1, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 235, Name: "SBC", Function: sbc, Mode: 5, Size: 0, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 236, Name: "CPX", Function: cpx, Mode: 1, Size: 3, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 237, Name: "SBC", Function: sbc, Mode: 1, Size: 3, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 238, Name: "INC", Function: inc, Mode: 1, Size: 3, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 239, Name: "ISC", Function: isc, Mode: 1, Size: 0, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 240, Name: "BEQ", Function: beq, Mode: 10, Size: 2, Cycles: 2, PageCycles: 1},
    Instruction{Opcode: 241, Name: "SBC", Function: sbc, Mode: 9, Size: 2, Cycles: 5, PageCycles: 1},
    Instruction{Opcode: 242, Name: "KIL", Function: kil, Mode: 6, Size: 0, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 243, Name: "ISC", Function: isc, Mode: 9, Size: 0, Cycles: 8, PageCycles: 0},
    Instruction{Opcode: 244, Name: "NOP", Function: nop, Mode: 12, Size: 2, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 245, Name: "SBC", Function: sbc, Mode: 12, Size: 2, Cycles: 4, PageCycles: 0},
    Instruction{Opcode: 246, Name: "INC", Function: inc, Mode: 12, Size: 2, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 247, Name: "ISC", Function: isc, Mode: 12, Size: 0, Cycles: 6, PageCycles: 0},
    Instruction{Opcode: 248, Name: "SED", Function: sed, Mode: 6, Size: 1, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 249, Name: "SBC", Function: sbc, Mode: 3, Size: 3, Cycles: 4, PageCycles: 1},
    Instruction{Opcode: 250, Name: "NOP", Function: nop, Mode: 6, Size: 1, Cycles: 2, PageCycles: 0},
    Instruction{Opcode: 251, Name: "ISC", Function: isc, Mode: 3, Size: 0, Cycles: 7, PageCycles: 0},
    Instruction{Opcode: 252, Name: "NOP", Function: nop, Mode: 2, Size: 3, Cycles: 4, PageCycles: 1},
    Instruction{Opcode: 253, Name: "SBC", Function: sbc, Mode: 2, Size: 3, Cycles: 4, PageCycles: 1},
    Instruction{Opcode: 254, Name: "INC", Function: inc, Mode: 2, Size: 3, Cycles: 7, PageCycles: 0},
    Instruction{Opcode: 255, Name: "ISC", Function: isc, Mode: 2, Size: 0, Cycles: 7, PageCycles: 0},
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

func init() {
    for i := 0; i < 31; i++ {
        pulseTable[i] = 95.52 / (8128.0/float32(i) + 100)
    }
    for i := 0; i < 203; i++ {
        tndTable[i] = 163.67 / (24329.0/float32(i) + 100)
    }

    colors := []uint32{
        0x666666, 0x002A88, 0x1412A7, 0x3B00A4, 0x5C007E, 0x6E0040, 0x6C0600, 0x561D00,
        0x333500, 0x0B4800, 0x005200, 0x004F08, 0x00404D, 0x000000, 0x000000, 0x000000,
        0xADADAD, 0x155FD9, 0x4240FF, 0x7527FE, 0xA01ACC, 0xB71E7B, 0xB53120, 0x994E00,
        0x6B6D00, 0x388700, 0x0C9300, 0x008F32, 0x007C8D, 0x000000, 0x000000, 0x000000,
        0xFFFEFF, 0x64B0FF, 0x9290FF, 0xC676FF, 0xF36AFF, 0xFE6ECC, 0xFE8170, 0xEA9E22,
        0xBCBE00, 0x88D800, 0x5CE430, 0x45E082, 0x48CDDE, 0x4F4F4F, 0x000000, 0x000000,
        0xFFFEFF, 0xC0DFFF, 0xD3D2FF, 0xE8C8FF, 0xFBC2FF, 0xFEC4EA, 0xFECCC5, 0xF7D8A5,
        0xE4E594, 0xCFEF96, 0xBDF4AB, 0xB3F3CC, 0xB5EBF2, 0xB8B8B8, 0x000000, 0x000000,
    }
    for i, c := range colors {
        r := byte(c >> 16)
        g := byte(c >> 8)
        b := byte(c)
        Palette[i] = color.RGBA{r, g, b, 0xFF}
    }
}