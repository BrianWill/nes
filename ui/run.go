package ui

import (
	"log"
	"runtime"
	"image"
	"image/color"
	"image/png"
	"image/draw"
	"bytes"
	"net/http"
	"io"
	"os"
	"path"
	"strings"

	"github.com/BrianWill/nes/nes"
	"github.com/go-gl/gl/v2.1/gl"
	"github.com/go-gl/glfw/v3.1/glfw"
	"github.com/gordonklaus/portaudio"
)

func init() {
	// we need a parallel OS thread to avoid audio stuttering
	runtime.GOMAXPROCS(2)

	// we need to keep OpenGL calls on a single thread
	runtime.LockOSThread()
}

func Run(paths []string) {
	clampScroll := func (v *MenuView, wrap bool) {
		n := len(v.paths)
		rows := n / v.nx
		if n%v.nx > 0 {
			rows++
		}
		maxScroll := rows - v.ny
		if v.scroll < 0 {
			if wrap {
				v.scroll = maxScroll
				v.j = v.ny - 1
			} else {
				v.scroll = 0
				v.j = 0
			}
		}
		if v.scroll > maxScroll {
			if wrap {
				v.scroll = 0
				v.j = 0
			} else {
				v.scroll = maxScroll
				v.j = v.ny - 1
			}
		}
	}

	setView := func (d *Director, view View) {
		if d.view != nil {
			switch v:= d.view.(type) {
			case *GameView:
				d.window.SetKeyCallback(nil)
				v.console.SetAudioChannel(nil)
				// save sram
				cartridge := v.console.Cartridge
				if cartridge.Battery != 0 {
					writeSRAM(sramPath(v.hash), cartridge.SRAM)
				}
			case *MenuView:
				d.window.SetCharCallback(nil)
			}
		}
		d.view = view
		if d.view != nil {
			switch v := d.view.(type) {
			case *GameView:
				gl.ClearColor(0, 0, 0, 1)
				d.window.SetTitle(v.title)
				v.console.SetAudioChannel(d.audio.channel)
				d.window.SetKeyCallback(
					func (window *glfw.Window, key glfw.Key, scancode int, action glfw.Action, mods glfw.ModifierKey) {
						if action == glfw.Press {
							switch key {
							case glfw.KeySpace:
								screenshot(v.console.Buffer())
							case glfw.KeyR:
								v.console.Reset()
							case glfw.KeyTab:
								if v.record {
									v.record = false
									animation(v.frames)
									v.frames = nil
								} else {
									v.record = true
								}
							}
						}
					},
				)
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
				d.window.SetCharCallback(
					func (window *glfw.Window, char rune) {
						now := glfw.GetTime()
						if now > v.typeTime {
							v.typeBuffer = ""
						}
						v.typeTime = now + typeDelay
						v.typeBuffer = strings.ToLower(v.typeBuffer + string(char))
						for index, p := range v.paths {
							_, p = path.Split(strings.ToLower(p))
							if p >= v.typeBuffer {
								v.scroll = index/v.nx - (v.ny-1)/2
								clampScroll(v, false)
								v.i = index % v.nx
								v.j = (index-v.i)/v.nx - v.scroll
								return
							}
						}
					},
				)
			}
		}
		d.timestamp = glfw.GetTime()
	}

	// returns index in .lookup
	loadTexture := func (t *Texture, romPath string) int {
		drawCenteredText := func (dst draw.Image, text string, dx, dy int, c color.Color) {
			// split text into rows
			const maxRowChars = 15
			var rows []string
			words := strings.Fields(text)
			if len(words) != 0 {
				row := words[0]
				for _, word := range words[1:] {
					newRow := row + " " + word
					if len(newRow) <= maxRowChars {
						row = newRow
					} else {
						rows = append(rows, row)
						row = word
					}
				}
				rows = append(rows, row)	
			}
			
			// draw all rows
			for i, row := range rows {
				x := 128 - len(row)*8 + dx
				y := 120 - len(rows)*12 + i*24 + dy
				// draw individual row
				//DrawText(dst, x+dx, y+dy, row, c)
				for _, ch := range row {
					// draw character
					if !(ch < 32 || ch > 128) {
						cx := int((ch-32)%16) * 16
						cy := int((ch-32)/16) * 16
						r := image.Rect(x, y, x+16, y+16)
						src := &image.Uniform{c}
						sp := image.Pt(cx, cy)
						draw.DrawMask(dst, r, src, sp, fontMask, sp, draw.Over)	
					}
					
					x += 16
				}
			}
		}

		// get index of lru (least recently used)
		var index int
		{
			minIndex := 0
			minValue := t.counter + 1
			for i, n := range t.access {
				if n < minValue {
					minIndex = i
					minValue = n
				}
			}
			index = minIndex
		}

		delete(t.lookup, t.reverse[index])

		// mark texture [btw: to keep track of least frequently used?]
		t.counter++
		t.access[index] = t.counter
		
		t.lookup[romPath] = index
		t.reverse[index] = romPath
		x := int32((index % textureDim) * 256)
		y := int32((index / textureDim) * 256)

		// load thumbnail
		var im image.Image
		{
			_, name := path.Split(romPath)
			name = strings.TrimSuffix(name, ".nes")
			name = strings.Replace(name, "_", " ", -1)
			name = strings.Title(name)

			// create generic thumbnail
			imRGBA := image.NewRGBA(image.Rect(0, 0, 256, 240))
			draw.Draw(imRGBA, imRGBA.Rect, &image.Uniform{color.Black}, image.ZP, draw.Src)
			drawCenteredText(imRGBA, name, 1, 2, color.RGBA{128, 128, 128, 255})
			drawCenteredText(imRGBA, name, 0, 0, color.White)
			im = image.Image(imRGBA)

			//
			hash, err := hashFile(romPath)
			if err != nil {
				// [btw: should't there be error handling?]
			} else {
				filename := thumbnailPath(hash)
				if _, err := os.Stat(filename); os.IsNotExist(err) {
					go (func (t *Texture, romPath, hash string) error {
						url := thumbnailURL(hash)
						filename := thumbnailPath(hash)
						dir, _ := path.Split(filename)

						resp, err := http.Get(url)
						if err != nil {
							return err
						}
						defer resp.Body.Close()

						if err := os.MkdirAll(dir, 0755); err != nil {
							return err
						}

						file, err := os.Create(filename)
						if err != nil {
							return err
						}
						defer file.Close()

						if _, err := io.Copy(file, resp.Body); err != nil {
							return err
						}

						t.ch <- romPath

						return nil
					})(t, romPath, hash)
				} else {
					thumbnail, err := loadPNG(filename)
					if err != nil {
						// [btw: should't there be error handling?]
					} else {
						im = thumbnail	
					}
				}
			}
		}

		imRGBA := copyImage(im)
		size := imRGBA.Rect.Size()
		gl.TexSubImage2D(
			gl.TEXTURE_2D, 0, x, y, int32(size.X), int32(size.Y),
			gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(imRGBA.Pix))
		return index
	}

	playGame := func (d *Director, path string) {
		hash, err := hashFile(path)
		if err != nil {
			log.Fatalln(err)
		}
		console, err := nes.NewConsole(path)
		if err != nil {
			log.Fatalln(err)
		}
		setView(d, &GameView{console, path, hash, createTexture(), false, nil})
	}


	// initialize audio
	portaudio.Initialize()
	defer portaudio.Terminate()
	audio := &Audio{channel: make(chan float32, 44100)}
	host, err := portaudio.DefaultHostApi()
	if err != nil {
		log.Fatalln(err)
	}
	stream, err := portaudio.OpenStream(
		portaudio.HighLatencyParameters(nil, host.DefaultOutputDevice),
		func (out []float32) {
			for i := range out {
				select {
				case sample := <-audio.channel:
					out[i] = sample
				default:
					out[i] = 0
				}
			}
		},
	)
	if err != nil {
		log.Fatalln(err)
	}
	if err := stream.Start(); err != nil {
		log.Fatalln(err)
	}
	audio.stream = stream
	defer audio.stream.Close()

	// initialize glfw
	if err := glfw.Init(); err != nil {
		log.Fatalln(err)
	}
	defer glfw.Terminate()

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
	{
		menuView := &MenuView{paths: paths}	
		texture := createTexture()
		gl.BindTexture(gl.TEXTURE_2D, texture)
		gl.TexImage2D(
			gl.TEXTURE_2D, 0, gl.RGBA,
			textureSize, textureSize,
			0, gl.RGBA, gl.UNSIGNED_BYTE, nil)
		gl.BindTexture(gl.TEXTURE_2D, 0)
		menuView.texture = &Texture{texture: texture, lookup: make(map[string]int), ch: make(chan string, 1024)}
		d.menuView = menuView
	}

	if len(paths) == 1 {
		playGame(d, paths[0])
	} else {
		setView(d, d.menuView)
	}
	// main loop
	for !d.window.ShouldClose() {
		gl.Clear(gl.COLOR_BUFFER_BIT)
		timestamp := glfw.GetTime()
		dt := timestamp - d.timestamp
		d.timestamp = timestamp
		if d.view != nil {
			switch v := d.view.(type) {
			case *GameView:
				if dt > 1 {
					dt = 0
				}
				if joystickReset(glfw.Joystick1) || joystickReset(glfw.Joystick2) || readKey(d.window, glfw.KeyEscape) {
					setView(d, d.menuView)
				}
				// update controllers
				{
					turbo := v.console.PPU.Frame%6 < 3
					k1 := readKeys(d.window, turbo)
					j1 := readJoystick(glfw.Joystick1, turbo)
					j2 := readJoystick(glfw.Joystick2, turbo)
					v.console.SetButtons1(combineButtons(k1, j1))
					v.console.SetButtons2(j2)
				}
				v.console.StepSeconds(dt)
				gl.BindTexture(gl.TEXTURE_2D, v.texture)
				setTexture(v.console.Buffer())
				// draw buffer
				{
					w, h := d.window.GetFramebufferSize()
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
					v.frames = append(v.frames, copyImage(v.console.Buffer()))
				}
			case *MenuView:
				// check buttons
				{
					k1 := readKeys(d.window, false)
					j1 := readJoystick(glfw.Joystick1, false)
					j2 := readJoystick(glfw.Joystick2, false)
					buttons := combineButtons(combineButtons(j1, j2), k1)
					now := glfw.GetTime()
					onPress := func (index int) {
						switch index {
						case nes.ButtonUp:
							v.j--
						case nes.ButtonDown:
							v.j++
						case nes.ButtonLeft:
							v.i--
						case nes.ButtonRight:
							v.i++
						default:
							return
						}
						v.t = glfw.GetTime()
					}
					for i := range buttons {
						if buttons[i] && !v.buttons[i] {
							v.times[i] = now + initialDelay
							onPress(i)
						} else if !buttons[i] && v.buttons[i] {
							// on release
							switch i {
							case nes.ButtonStart:
								// on select [btw: for button start???]
								index := v.nx*(v.j+v.scroll) + v.i
								if index < len(v.paths) {
									playGame(d, v.paths[index])
								}
							}
						} else if buttons[i] && now >= v.times[i] {
							v.times[i] = now + repeatDelay
							onPress(i)
						}
					}
					v.buttons = buttons
				}
				// purge texture [btw: what texture though?]
				purge: 
				for {
					select {
					case path := <-v.texture.ch:
						delete(v.texture.lookup, path)
					default:
						break purge
					}
				}

				w, h := d.window.GetFramebufferSize()
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
					clampScroll(v, true)
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

						if i, ok := v.texture.lookup[path]; ok {
							index = i
						} else {
							index = loadTexture(v.texture, path)
						}
						tx := float32(index%textureDim) / textureDim
						ty := float32(index/textureDim) / textureDim
						tw := float32(1.0) / textureDim
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
				gl.BindTexture(gl.TEXTURE_2D, 0)
				if int((timestamp - v.t)*4)%2 == 0 {
					// draw selection
					x := float32(ox + v.i*sx)
					y := float32(oy + v.j*sy)
					var p, w float32 = 8, 4
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
	setView(d, nil)
}