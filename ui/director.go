package ui

import (
	"log"

	"github.com/BrianWill/nes/nes"
	"github.com/go-gl/gl/v2.1/gl"
	"github.com/go-gl/glfw/v3.1/glfw"
)

type View interface {
	View()
}

func (_ *GameView) View() {}
func (_ *MenuView) View() {}

type Director struct {
	window    *glfw.Window
	audio     *Audio
	view      View
	menuView  View
	timestamp float64
}

func NewDirector(window *glfw.Window, audio *Audio) *Director {
	director := Director{}
	director.window = window
	director.audio = audio
	return &director
}


func (d *Director) SetView(view View) {
	if d.view != nil {
		// exit view
		switch v := d.view.(type) {
		case *GameView:
			v.director.window.SetKeyCallback(nil)
			v.console.SetAudioChannel(nil)
			v.console.SetAudioSampleRate(0)
			// save sram
			cartridge := v.console.Cartridge
			if cartridge.Battery != 0 {
				writeSRAM(sramPath(v.hash), cartridge.SRAM)
			}
			// save state
			v.console.SaveState(savePath(v.hash))
		case *MenuView:
			v.director.window.SetCharCallback(nil)
		}
		//d.view.Exit()
	}
	d.view = view
	if d.view != nil {
		// enter view
		switch v := d.view.(type) {
		case *GameView:
			gl.ClearColor(0, 0, 0, 1)
			d.window.SetTitle(view.title)
			v.console.SetAudioChannel(v.director.audio.channel)
			v.console.SetAudioSampleRate(v.director.audio.sampleRate)
			v.director.window.SetKeyCallback(v.onKey)
			// load state
			if err := v.console.LoadState(savePath(v.hash)); err == nil {
				return
			} else {
				v.console.Reset()
			}
			// load sram
			cartridge := v.console.Cartridge
			if cartridge.Battery != 0 {
				if sram, err := readSRAM(sramPath(v.hash)); err == nil {
					cartridge.SRAM = sram
				}
			}
		case *MenuView:
			gl.ClearColor(0.333, 0.333, 0.333, 1)
			d.window.SetTitle("Select Game")
			v.director.window.SetCharCallback(v.onChar)
		}
	}
	d.timestamp = glfw.GetTime()
}

func (d *Director) Step() {
	gl.Clear(gl.COLOR_BUFFER_BIT)
	timestamp := glfw.GetTime()
	dt := timestamp - d.timestamp
	d.timestamp = timestamp
	if d.view != nil {
		// update view
		switch v := d.view.(type) {
		case *GameView:
			if dt > 1 {
				dt = 0
			}
			window := v.director.window
			console := v.console
			if joystickReset(glfw.Joystick1) || joystickReset(glfw.Joystick2) || readKey(window, glfw.KeyEscape) {
				director.SetView(director.menuView)
			}
			updateControllers(window, console)
			console.StepSeconds(dt)
			gl.BindTexture(gl.TEXTURE_2D, v.texture)
			setTexture(console.Buffer())
			drawBuffer(v.director.window)
			gl.BindTexture(gl.TEXTURE_2D, 0)
			if v.record {
				v.frames = append(v.frames, copyImage(console.Buffer()))
			}
		case *MenuView:
			v.checkButtons()
			v.texture.Purge()
			window := v.director.window
			w, h := window.GetFramebufferSize()
			sx := 256 + margin*2
			sy := 240 + margin*2
			nx := (w - border*2) / sx
			ny := (h - border*2) / sy
			ox := (w-nx*sx)/2 + margin
			oy := (h-ny*sy)/2 + margin
			if nx < 1 {
				nx = 1
			}
			if ny < 1 {
				ny = 1
			}
			v.nx = nx
			v.ny = ny
			v.clampSelection()
			gl.PushMatrix()
			gl.Ortho(0, float64(w), float64(h), 0, -1, 1)
			v.texture.Bind()
			for j := 0; j < ny; j++ {
				for i := 0; i < nx; i++ {
					x := float32(ox + i*sx)
					y := float32(oy + j*sy)
					index := nx*(j+v.scroll) + i
					if index >= len(v.paths) {
						continue
					}
					path := v.paths[index]
					tx, ty, tw, th := v.texture.Lookup(path)
					
					// draw thumbnail
					sx := x + 4
					sy := y + 4
					gl.Disable(gl.TEXTURE_2D)
					gl.Color3f(0.2, 0.2, 0.2)
					gl.Begin(gl.QUADS)
					gl.Vertex2f(sx, sy)
					gl.Vertex2f(sx+256, sy)
					gl.Vertex2f(sx+256, sy+240)
					gl.Vertex2f(sx, sy+240)
					gl.End()
					gl.Enable(gl.TEXTURE_2D)
					gl.Color3f(1, 1, 1)
					gl.Begin(gl.QUADS)
					gl.TexCoord2f(tx, ty)
					gl.Vertex2f(x, y)
					gl.TexCoord2f(tx+tw, ty)
					gl.Vertex2f(x+256, y)
					gl.TexCoord2f(tx+tw, ty+th)
					gl.Vertex2f(x+256, y+240)
					gl.TexCoord2f(tx, ty+th)
					gl.Vertex2f(x, y+240)
					gl.End()
				}
			}
			v.texture.Unbind()
			if int((timestamp - v.t)*4)%2 == 0 {
				x := float32(ox + v.i*sx)
				y := float32(oy + v.j*sy)
				p, w := 8, 4

				// draw selection highlight border
				gl.LineWidth(w)
				gl.Begin(gl.LINE_STRIP)
				gl.Vertex2f(x-p, y-p)
				gl.Vertex2f(x+256+p, y-p)
				gl.Vertex2f(x+256+p, y+240+p)
				gl.Vertex2f(x-p, y+240+p)
				gl.Vertex2f(x-p, y-p)
				gl.End()				
			}
			gl.PopMatrix()
		}
	}
}

func (d *Director) PlayGame(path string) {
	hash, err := hashFile(path)
	if err != nil {
		log.Fatalln(err)
	}
	console, err := nes.NewConsole(path)
	if err != nil {
		log.Fatalln(err)
	}
	d.SetView(NewGameView(d, console, path, hash))
}
