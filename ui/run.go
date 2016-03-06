package ui

import (
	"log"
	"runtime"

	"github.com/go-gl/gl/v2.1/gl"
	"github.com/go-gl/glfw/v3.1/glfw"
	"github.com/gordonklaus/portaudio"
)

const (
	width  = 256
	height = 240
	scale  = 3
	title  = "NES"
)

func init() {
	// we need a parallel OS thread to avoid audio stuttering
	runtime.GOMAXPROCS(2)

	// we need to keep OpenGL calls on a single thread
	runtime.LockOSThread()
}

func Run(paths []string) {
	// initialize audio
	portaudio.Initialize()
	defer portaudio.Terminate()

	audio := NewAudio()
	if err := audio.Start(); err != nil {
		log.Fatalln(err)
	}
	defer audio.Stop()

	// initialize glfw
	if err := glfw.Init(); err != nil {
		log.Fatalln(err)
	}
	defer glfw.Terminate()

	// create window
	glfw.WindowHint(glfw.ContextVersionMajor, 2)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)
	window, err := glfw.CreateWindow(width*scale, height*scale, title, nil, nil)
	if err != nil {
		log.Fatalln(err)
	}
	window.MakeContextCurrent()

	// initialize gl
	if err := gl.Init(); err != nil {
		log.Fatalln(err)
	}
	gl.Enable(gl.TEXTURE_2D)

	// run director
	d := &Director{}
	director.window = window
	director.audio = audio

	d.menuView = NewMenuView(d, paths)
	if len(paths) == 1 {
		d.PlayGame(paths[0])
	} else {
		d.SetView(d.menuView)
	}

	// main loop
	for !d.window.ShouldClose() {
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
				// purge texture
				for {
					select {
					case path := <-v.texture.ch:
						delete(v.texture.lookup, path)
					default:
						return
					}
				}
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
				gl.BindTexture(gl.TEXTURE_2D, v.texture.texture)
				for j := 0; j < ny; j++ {
					for i := 0; i < nx; i++ {
						x := float32(ox + i*sx)
						y := float32(oy + j*sy)
						index := nx*(j+v.scroll) + i
						if index >= len(v.paths) {
							continue
						}
						path := v.paths[index]

						var index int
						if idx, ok := v.texture.lookup[path]; ok {
							index = idx
						} else {
							index = v.texture.load(path)
						}
						// texture coords
						tx := float32(index % textureDim) / textureDim
						ty := float32(index / textureDim) / textureDim
						tw := 1.0 / textureDim
						th := tw * 240 / 256
						
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
				gl.BindTexture(gl.TEXTURE_2D, 0)  // unbind
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
		d.window.SwapBuffers()
		glfw.PollEvents()
	}
	d.SetView(nil)
}
