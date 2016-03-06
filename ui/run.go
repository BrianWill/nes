package ui

import (
	"log"
	"runtime"
	"image"

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

const padding = 0

type GameView struct {
	director *Director
	console  *nes.Console
	title    string
	hash     string
	texture  uint32
	record   bool
	frames   []image.Image
}

func (view *GameView) onKey(window *glfw.Window,
	key glfw.Key, scancode int, action glfw.Action, mods glfw.ModifierKey) {
	if action == glfw.Press {
		switch key {
		case glfw.KeySpace:
			screenshot(view.console.Buffer())
		case glfw.KeyR:
			view.console.Reset()
		case glfw.KeyTab:
			if view.record {
				view.record = false
				animation(view.frames)
				view.frames = nil
			} else {
				view.record = true
			}
		}
	}
}

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

	// initialize fontMask
	{
		im, err := png.Decode(bytes.NewBuffer(fontData))
		if err != nil {
			log.Fatalln(err)
		}
		size := im.Bounds().Size()
		mask := image.NewRGBA(im.Bounds())
		for y := 0; y < size.Y; y++ {
			for x := 0; x < size.X; x++ {
				r, _, _, _ := im.At(x, y).RGBA()
				if r > 0 {
					mask.Set(x, y, color.Opaque)
				}
			}
		}
		fontMask = mask
	}

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
	d := &Director{window: window, audio: audio}
	d.menuView := &MenuView{director: d, paths: paths, texture: NewTexture()}

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
				// update controllers
				{
					turbo := console.PPU.Frame%6 < 3
					k1 := readKeys(window, turbo)
					j1 := readJoystick(glfw.Joystick1, turbo)
					j2 := readJoystick(glfw.Joystick2, turbo)
					console.SetButtons1(combineButtons(k1, j1))
					console.SetButtons2(j2)
				}
				console.StepSeconds(dt)
				gl.BindTexture(gl.TEXTURE_2D, v.texture)
				setTexture(console.Buffer())
				// draw buffer
				{
					window := v.director.window
					w, h := window.GetFramebufferSize()
					s1 := float32(w) / 256
					s2 := float32(h) / 240
					f := float32(1 - padding)
					var x, y float32
					if s1 >= s2 {
						x = f * s2 / s1
						y = f
					} else {
						x = f
						y = f * s1 / s2
					}
					gl.Begin(gl.QUADS)
					gl.TexCoord2f(0, 1)
					gl.Vertex2f(-x, -y)
					gl.TexCoord2f(1, 1)
					gl.Vertex2f(x, -y)
					gl.TexCoord2f(1, 0)
					gl.Vertex2f(x, y)
					gl.TexCoord2f(0, 0)
					gl.Vertex2f(-x, y)
					gl.End()
				}

				gl.BindTexture(gl.TEXTURE_2D, 0)
				if v.record {
					v.frames = append(v.frames, copyImage(console.Buffer()))
				}
			case *MenuView:
				// check buttons
				{
					window := v.director.window
					k1 := readKeys(window, false)
					j1 := readJoystick(glfw.Joystick1, false)
					j2 := readJoystick(glfw.Joystick2, false)
					buttons := combineButtons(combineButtons(j1, j2), k1)
					now := glfw.GetTime()
					for i := range buttons {
						if buttons[i] && !v.buttons[i] {
							v.times[i] = now + initialDelay
							v.onPress(i)
						} else if !buttons[i] && v.buttons[i] {
							v.onRelease(i)
						} else if buttons[i] && now >= v.times[i] {
							v.times[i] = now + repeatDelay
							v.onPress(i)
						}
					}
					v.buttons = buttons
				}
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

				// clamp selection
				{
					if v.i < 0 {
						v.i = v.nx - 1
					}
					if v.i >= v.nx {
						v.i = 0
					}
					if v.j < 0 {
						v.j = 0
						v.scroll--
					}
					if v.j >= v.ny {
						v.j = v.ny - 1
						v.scroll++
					}
					v.clampScroll(true)
				}

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
