package ui

import (
	"path"
	"strings"

	"github.com/BrianWill/nes/nes"
	"github.com/go-gl/gl/v2.1/gl"
	"github.com/go-gl/glfw/v3.1/glfw"
)

const (
	border       = 10
	margin       = 10
	initialDelay = 0.3
	repeatDelay  = 0.1
	typeDelay    = 0.5
)

type MenuView struct {
	director     *Director
	paths        []string
	texture      *Texture
	nx, ny, i, j int
	scroll       int
	t            float64
	buttons      [8]bool
	times        [8]float64
	typeBuffer   string
	typeTime     float64
}

func NewMenuView(director *Director, paths []string) View {
	view := MenuView{}
	view.director = director
	view.paths = paths
	view.texture = NewTexture()
	return &view
}

func (view *MenuView) checkButtons() {
	window := view.director.window
	k1 := readKeys(window, false)
	j1 := readJoystick(glfw.Joystick1, false)
	j2 := readJoystick(glfw.Joystick2, false)
	buttons := combineButtons(combineButtons(j1, j2), k1)
	now := glfw.GetTime()
	for i := range buttons {
		if buttons[i] && !view.buttons[i] {
			view.times[i] = now + initialDelay
			view.onPress(i)
		} else if !buttons[i] && view.buttons[i] {
			view.onRelease(i)
		} else if buttons[i] && now >= view.times[i] {
			view.times[i] = now + repeatDelay
			view.onPress(i)
		}
	}
	view.buttons = buttons
}

func (view *MenuView) onPress(index int) {
	switch index {
	case nes.ButtonUp:
		view.j--
	case nes.ButtonDown:
		view.j++
	case nes.ButtonLeft:
		view.i--
	case nes.ButtonRight:
		view.i++
	default:
		return
	}
	view.t = glfw.GetTime()
}

func (view *MenuView) onRelease(index int) {
	switch index {
	case nes.ButtonStart:
		view.onSelect()
	}
}

func (view *MenuView) onSelect() {
	index := view.nx*(view.j+view.scroll) + view.i
	if index >= len(view.paths) {
		return
	}
	view.director.PlayGame(view.paths[index])
}

func (view *MenuView) onChar(window *glfw.Window, char rune) {
	now := glfw.GetTime()
	if now > view.typeTime {
		view.typeBuffer = ""
	}
	view.typeTime = now + typeDelay
	view.typeBuffer = strings.ToLower(view.typeBuffer + string(char))
	for index, p := range view.paths {
		_, p = path.Split(strings.ToLower(p))
		if p >= view.typeBuffer {
			view.highlight(index)
			return
		}
	}
}

func (view *MenuView) highlight(index int) {
	view.scroll = index/view.nx - (view.ny-1)/2
	view.clampScroll(false)
	view.i = index % view.nx
	view.j = (index-view.i)/view.nx - view.scroll
}


func (view *MenuView) clampSelection() {
	if view.i < 0 {
		view.i = view.nx - 1
	}
	if view.i >= view.nx {
		view.i = 0
	}
	if view.j < 0 {
		view.j = 0
		view.scroll--
	}
	if view.j >= view.ny {
		view.j = view.ny - 1
		view.scroll++
	}
	view.clampScroll(true)
}

func (view *MenuView) clampScroll(wrap bool) {
	n := len(view.paths)
	rows := n / view.nx
	if n%view.nx > 0 {
		rows++
	}
	maxScroll := rows - view.ny
	if view.scroll < 0 {
		if wrap {
			view.scroll = maxScroll
			view.j = view.ny - 1
		} else {
			view.scroll = 0
			view.j = 0
		}
	}
	if view.scroll > maxScroll {
		if wrap {
			view.scroll = 0
			view.j = 0
		} else {
			view.scroll = maxScroll
			view.j = view.ny - 1
		}
	}
}
