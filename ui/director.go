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

func (d *Director) PlayGame(path string) {
	hash, err := hashFile(path)
	if err != nil {
		log.Fatalln(err)
	}
	console, err := nes.NewConsole(path)
	if err != nil {
		log.Fatalln(err)
	}
	d.SetView(&GameView{d, console, path, hash, createTexture(), false, nil})
}