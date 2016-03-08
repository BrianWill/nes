package nes

import (
    "image/color"
    "image"
)

type Memory interface {
    Read(address uint16) byte
    Write(address uint16, value byte)
}

type cpuMemory struct {
    console *Console
}

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
    Memory           // memory interface
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
    table     [256]func(*stepInfo)
}

type PPU struct {
    Memory           // memory interface
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
    Magic    uint32  // iNES magic number
    NumPRG   byte    // number of PRG-ROM banks (16KB each)
    NumCHR   byte    // number of CHR-ROM banks (8KB each)
    Control1 byte    // control bits
    Control2 byte    // control bits
    NumRAM   byte    // PRG-RAM size (x 8KB)
    _        [7]byte // unused padding (necessary for properly reading ROM file)
}

// stepInfo contains information that the instruction functions use
type stepInfo struct {
    address uint16
    pc      uint16
    mode    byte
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

// instructionModes indicates the addressing mode for each instruction
var instructionModes = [256]byte{
    6, 7, 6, 7, 11, 11, 11, 11, 6, 5, 4, 5, 1, 1, 1, 1,
    10, 9, 6, 9, 12, 12, 12, 12, 6, 3, 6, 3, 2, 2, 2, 2,
    1, 7, 6, 7, 11, 11, 11, 11, 6, 5, 4, 5, 1, 1, 1, 1,
    10, 9, 6, 9, 12, 12, 12, 12, 6, 3, 6, 3, 2, 2, 2, 2,
    6, 7, 6, 7, 11, 11, 11, 11, 6, 5, 4, 5, 1, 1, 1, 1,
    10, 9, 6, 9, 12, 12, 12, 12, 6, 3, 6, 3, 2, 2, 2, 2,
    6, 7, 6, 7, 11, 11, 11, 11, 6, 5, 4, 5, 8, 1, 1, 1,
    10, 9, 6, 9, 12, 12, 12, 12, 6, 3, 6, 3, 2, 2, 2, 2,
    5, 7, 5, 7, 11, 11, 11, 11, 6, 5, 6, 5, 1, 1, 1, 1,
    10, 9, 6, 9, 12, 12, 13, 13, 6, 3, 6, 3, 2, 2, 3, 3,
    5, 7, 5, 7, 11, 11, 11, 11, 6, 5, 6, 5, 1, 1, 1, 1,
    10, 9, 6, 9, 12, 12, 13, 13, 6, 3, 6, 3, 2, 2, 3, 3,
    5, 7, 5, 7, 11, 11, 11, 11, 6, 5, 6, 5, 1, 1, 1, 1,
    10, 9, 6, 9, 12, 12, 12, 12, 6, 3, 6, 3, 2, 2, 2, 2,
    5, 7, 5, 7, 11, 11, 11, 11, 6, 5, 6, 5, 1, 1, 1, 1,
    10, 9, 6, 9, 12, 12, 12, 12, 6, 3, 6, 3, 2, 2, 2, 2,
}

// instructionSizes indicates the size of each instruction in bytes
var instructionSizes = [256]byte{
    1, 2, 0, 0, 2, 2, 2, 0, 1, 2, 1, 0, 3, 3, 3, 0,
    2, 2, 0, 0, 2, 2, 2, 0, 1, 3, 1, 0, 3, 3, 3, 0,
    3, 2, 0, 0, 2, 2, 2, 0, 1, 2, 1, 0, 3, 3, 3, 0,
    2, 2, 0, 0, 2, 2, 2, 0, 1, 3, 1, 0, 3, 3, 3, 0,
    1, 2, 0, 0, 2, 2, 2, 0, 1, 2, 1, 0, 3, 3, 3, 0,
    2, 2, 0, 0, 2, 2, 2, 0, 1, 3, 1, 0, 3, 3, 3, 0,
    1, 2, 0, 0, 2, 2, 2, 0, 1, 2, 1, 0, 3, 3, 3, 0,
    2, 2, 0, 0, 2, 2, 2, 0, 1, 3, 1, 0, 3, 3, 3, 0,
    2, 2, 0, 0, 2, 2, 2, 0, 1, 0, 1, 0, 3, 3, 3, 0,
    2, 2, 0, 0, 2, 2, 2, 0, 1, 3, 1, 0, 0, 3, 0, 0,
    2, 2, 2, 0, 2, 2, 2, 0, 1, 2, 1, 0, 3, 3, 3, 0,
    2, 2, 0, 0, 2, 2, 2, 0, 1, 3, 1, 0, 3, 3, 3, 0,
    2, 2, 0, 0, 2, 2, 2, 0, 1, 2, 1, 0, 3, 3, 3, 0,
    2, 2, 0, 0, 2, 2, 2, 0, 1, 3, 1, 0, 3, 3, 3, 0,
    2, 2, 0, 0, 2, 2, 2, 0, 1, 2, 1, 0, 3, 3, 3, 0,
    2, 2, 0, 0, 2, 2, 2, 0, 1, 3, 1, 0, 3, 3, 3, 0,
}

// instructionCycles indicates the number of cycles used by each instruction,
// not including conditional cycles
var instructionCycles = [256]byte{
    7, 6, 2, 8, 3, 3, 5, 5, 3, 2, 2, 2, 4, 4, 6, 6,
    2, 5, 2, 8, 4, 4, 6, 6, 2, 4, 2, 7, 4, 4, 7, 7,
    6, 6, 2, 8, 3, 3, 5, 5, 4, 2, 2, 2, 4, 4, 6, 6,
    2, 5, 2, 8, 4, 4, 6, 6, 2, 4, 2, 7, 4, 4, 7, 7,
    6, 6, 2, 8, 3, 3, 5, 5, 3, 2, 2, 2, 3, 4, 6, 6,
    2, 5, 2, 8, 4, 4, 6, 6, 2, 4, 2, 7, 4, 4, 7, 7,
    6, 6, 2, 8, 3, 3, 5, 5, 4, 2, 2, 2, 5, 4, 6, 6,
    2, 5, 2, 8, 4, 4, 6, 6, 2, 4, 2, 7, 4, 4, 7, 7,
    2, 6, 2, 6, 3, 3, 3, 3, 2, 2, 2, 2, 4, 4, 4, 4,
    2, 6, 2, 6, 4, 4, 4, 4, 2, 5, 2, 5, 5, 5, 5, 5,
    2, 6, 2, 6, 3, 3, 3, 3, 2, 2, 2, 2, 4, 4, 4, 4,
    2, 5, 2, 5, 4, 4, 4, 4, 2, 4, 2, 4, 4, 4, 4, 4,
    2, 6, 2, 8, 3, 3, 5, 5, 2, 2, 2, 2, 4, 4, 6, 6,
    2, 5, 2, 8, 4, 4, 6, 6, 2, 4, 2, 7, 4, 4, 7, 7,
    2, 6, 2, 8, 3, 3, 5, 5, 2, 2, 2, 2, 4, 4, 6, 6,
    2, 5, 2, 8, 4, 4, 6, 6, 2, 4, 2, 7, 4, 4, 7, 7,
}

// instructionPageCycles indicates the number of cycles used by each
// instruction when a page is crossed
var instructionPageCycles = [256]byte{
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
    1, 1, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 1, 1, 0, 0,
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
    1, 1, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 1, 1, 0, 0,
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
    1, 1, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 1, 1, 0, 0,
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
    1, 1, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 1, 1, 0, 0,
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
    1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
    1, 1, 0, 1, 0, 0, 0, 0, 0, 1, 0, 1, 1, 1, 1, 1,
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
    1, 1, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 1, 1, 0, 0,
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
    1, 1, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 1, 1, 0, 0,
}

// instructionNames indicates the name of each instruction
var instructionNames = [256]string{
    "BRK", "ORA", "KIL", "SLO", "NOP", "ORA", "ASL", "SLO",
    "PHP", "ORA", "ASL", "ANC", "NOP", "ORA", "ASL", "SLO",
    "BPL", "ORA", "KIL", "SLO", "NOP", "ORA", "ASL", "SLO",
    "CLC", "ORA", "NOP", "SLO", "NOP", "ORA", "ASL", "SLO",
    "JSR", "AND", "KIL", "RLA", "BIT", "AND", "ROL", "RLA",
    "PLP", "AND", "ROL", "ANC", "BIT", "AND", "ROL", "RLA",
    "BMI", "AND", "KIL", "RLA", "NOP", "AND", "ROL", "RLA",
    "SEC", "AND", "NOP", "RLA", "NOP", "AND", "ROL", "RLA",
    "RTI", "EOR", "KIL", "SRE", "NOP", "EOR", "LSR", "SRE",
    "PHA", "EOR", "LSR", "ALR", "JMP", "EOR", "LSR", "SRE",
    "BVC", "EOR", "KIL", "SRE", "NOP", "EOR", "LSR", "SRE",
    "CLI", "EOR", "NOP", "SRE", "NOP", "EOR", "LSR", "SRE",
    "RTS", "ADC", "KIL", "RRA", "NOP", "ADC", "ROR", "RRA",
    "PLA", "ADC", "ROR", "ARR", "JMP", "ADC", "ROR", "RRA",
    "BVS", "ADC", "KIL", "RRA", "NOP", "ADC", "ROR", "RRA",
    "SEI", "ADC", "NOP", "RRA", "NOP", "ADC", "ROR", "RRA",
    "NOP", "STA", "NOP", "SAX", "STY", "STA", "STX", "SAX",
    "DEY", "NOP", "TXA", "XAA", "STY", "STA", "STX", "SAX",
    "BCC", "STA", "KIL", "AHX", "STY", "STA", "STX", "SAX",
    "TYA", "STA", "TXS", "TAS", "SHY", "STA", "SHX", "AHX",
    "LDY", "LDA", "LDX", "LAX", "LDY", "LDA", "LDX", "LAX",
    "TAY", "LDA", "TAX", "LAX", "LDY", "LDA", "LDX", "LAX",
    "BCS", "LDA", "KIL", "LAX", "LDY", "LDA", "LDX", "LAX",
    "CLV", "LDA", "TSX", "LAS", "LDY", "LDA", "LDX", "LAX",
    "CPY", "CMP", "NOP", "DCP", "CPY", "CMP", "DEC", "DCP",
    "INY", "CMP", "DEX", "AXS", "CPY", "CMP", "DEC", "DCP",
    "BNE", "CMP", "KIL", "DCP", "NOP", "CMP", "DEC", "DCP",
    "CLD", "CMP", "NOP", "DCP", "NOP", "CMP", "DEC", "DCP",
    "CPX", "SBC", "NOP", "ISC", "CPX", "SBC", "INC", "ISC",
    "INX", "SBC", "NOP", "SBC", "CPX", "SBC", "INC", "ISC",
    "BEQ", "SBC", "KIL", "ISC", "NOP", "SBC", "INC", "ISC",
    "SED", "SBC", "NOP", "ISC", "NOP", "SBC", "INC", "ISC",
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