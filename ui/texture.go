package ui

import (
	"image"
	"image/draw"
	"image/color"
	"io"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/go-gl/gl/v2.1/gl"
)

const textureSize = 4096
const textureDim = textureSize / 256
const textureCount = textureDim * textureDim


type Texture struct {
	texture uint32
	lookup  map[string]int
	reverse [textureCount]string
	access  [textureCount]int
	counter int
	ch      chan string
}

func NewTexture() *Texture {
	texture := createTexture()
	gl.BindTexture(gl.TEXTURE_2D, texture)
	gl.TexImage2D(
		gl.TEXTURE_2D, 0, gl.RGBA,
		textureSize, textureSize,
		0, gl.RGBA, gl.UNSIGNED_BYTE, nil)
	gl.BindTexture(gl.TEXTURE_2D, 0)

	t := Texture{}
	t.texture = texture
	t.lookup = make(map[string]int)
	t.ch = make(chan string, 1024)
	return &t
}

func loadTexture(t *Texture, romPath string) int {
	// lru (least recently used)
	minIndex := 0
	minValue := t.counter + 1
	for i, n := range t.access {
		if n < minValue {
			minIndex = i
			minValue = n
		}
	}
	index := minIndex

	delete(t.lookup, t.reverse[index])
	
	// mark the texture
	t.counter++
	t.access[index] = t.counter

	t.lookup[romPath] = index
	t.reverse[index] = romPath
	x := int32((index % textureDim) * 256)
	y := int32((index / textureDim) * 256)

	// load thumbnail texture
	_, name := path.Split(romPath)
	name = strings.TrimSuffix(name, ".nes")
	name = strings.Replace(name, "_", " ", -1)
	name = strings.Title(name)
	
	// create thumbnail
	imRGBA := image.NewRGBA(image.Rect(0, 0, 256, 240))
	draw.Draw(imRGBA, imRGBA.Rect, &image.Uniform{color.Black}, image.ZP, draw.Src)
	DrawCenteredText(imRGBA, name, 1, 2, color.RGBA{128, 128, 128, 255})
	DrawCenteredText(imRGBA, name, 0, 0, color.White)
	im := image.Image(imRGBA)

	hash, err := hashFile(romPath)
	if err != nil {
		// just use existing value of im
	} else {
		filename := thumbnailPath(hash)
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			// download thumbnail
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
				// just use existing value of im
			} else {
				im = thumbnail
			}
		}
	}

	//
	imRGBA= copyImage(im)
	size := imRGBA.Rect.Size()
	gl.TexSubImage2D(
		gl.TEXTURE_2D, 0, x, y, int32(size.X), int32(size.Y),
		gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(imRGBA.Pix))
	return index
}

