package ui

import (
	"path"
	"strings"

	"github.com/BrianWill/nes/nes"
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
			// highlight
			view.scroll = index/view.nx - (view.ny-1)/2
			view.clampScroll(false)
			view.i = index % view.nx
			view.j = (index-view.i)/view.nx - view.scroll
			return
		}
	}
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
