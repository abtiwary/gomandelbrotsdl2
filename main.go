package main

import (
	"math"
	"os"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/veandco/go-sdl2/sdl"
)

type Point struct {
	X     float64
	Y     float64
	Red   uint8
	Green uint8
	Blue  uint8
}

type Settings struct {
	Width         float64
	Height        float64
	Min           float64
	Max           float64
	MaxIterations int64
	Center        Point
}

type MandelbrotImage struct {
	mu       sync.Mutex
	Width    float64
	Height   float64
	Pixels   []byte
	Settings *Settings
	Jobs     chan Point
}

func NewMandelbrotImage(width, height float64, settings *Settings) *MandelbrotImage {
	return &MandelbrotImage{
		Width:    width,
		Height:   height,
		Pixels:   make([]byte, int(width*height*4)),
		Settings: settings,
		Jobs:     make(chan Point),
	}
}

func (mi *MandelbrotImage) Init() {
	var i uint64
	for i = 0; i < uint64(mi.Width*mi.Height); i += 4 {
		mi.Pixels[i] = 0
		mi.Pixels[i+1] = 0
		mi.Pixels[i+2] = 0
		mi.Pixels[i+3] = 0
	}
}

func (mi *MandelbrotImage) DrawPoint(point Point) {
	mi.mu.Lock()
	defer mi.mu.Unlock()
	idx := (int(point.Y) * int(mi.Width) * 4) + (int(point.X) * 4)

	mi.Pixels[idx] = point.Red
	mi.Pixels[idx+1] = point.Green
	mi.Pixels[idx+2] = point.Blue
	mi.Pixels[idx+3] = 255
}

func (mi *MandelbrotImage) ForceRender() {
	var wg sync.WaitGroup
	var i int64
	var j int64
	for i = 0; i < int64(mi.Width); i++ {
		for j = 0; j < int64(mi.Height); j++ {
			pt := Point{
				X: float64(i),
				Y: float64(j),
			}
			wg.Add(1)
			go mandelbrotWorker(&wg, pt, mi.Jobs, mi.Settings)
		}
	}

	go func() {
		wg.Wait()
	}()

}

func (mi *MandelbrotImage) Close() {
	close(mi.Jobs)
}

func mapToRange(val, in_min, in_max, out_min, out_max float64) float64 {
	return (val-in_min)*(out_max-out_min)/(in_max-in_min) + out_min
}

func imageWriter(mi *MandelbrotImage, jobs chan Point) {
	for {
		select {
		case pt := <-jobs:
			{
				mi.DrawPoint(pt)
			}

		default:
			{
			}
		}
	}
}

func mandelbrotWorker(wg *sync.WaitGroup, pt Point, jobs chan Point, settings *Settings) {
	defer wg.Done()

	i := pt.X
	j := pt.Y

	x := mapToRange(float64(i), 0, settings.Width, settings.Min, settings.Max)
	y := mapToRange(float64(j), 0, settings.Height, settings.Min, settings.Max)

	x = x - settings.Center.X
	y = y - settings.Center.Y

	x0 := x
	y0 := y

	var iters int64
	var z int64
	for z = 0; z < settings.MaxIterations; z++ {
		x1 := x*x - y*y
		y1 := 2 * x * y
		x = x1 + x0
		y = y1 + y0

		if x+y > 2 {
			break
		}
		iters += 1
	}

	col := mapToRange(float64(iters), 0, float64(settings.MaxIterations), 0, 255)
	if iters == settings.MaxIterations || col < 20 {
		col = 0
	}

	red := mapToRange(col*col, 0, 255*255, 0, 255)
	green := mapToRange(col/2, 0, 255/2, 0, 255)
	blue := mapToRange(math.Sqrt(col), 0, math.Sqrt(255), 0, 255)
	outpt := Point{
		X:     i,
		Y:     j,
		Red:   uint8(red),
		Green: uint8(green),
		Blue:  uint8(blue),
	}
	jobs <- outpt
	return
}

func main() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)

	err := sdl.Init(sdl.INIT_EVERYTHING)
	if err != nil {
		log.WithError(err).Panic("could not init SDL2")
	}
	defer sdl.Quit()

	settings := Settings{
		Width:         800,
		Height:        800,
		Min:           -2.84,
		Max:           2.0,
		MaxIterations: 200,
		Center: Point{
			X: 0.5,
			Y: 0.0,
		},
	}

	window, err := sdl.CreateWindow("Mandelbrot Set",
		sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED,
		1280, 720, sdl.WINDOW_SHOWN)
	if err != nil {
		log.WithError(err).Panic("error creating a window")
	}
	defer window.Destroy()

	renderer, err := sdl.CreateRenderer(window, -1, sdl.RENDERER_ACCELERATED)
	if err != nil {
		log.WithError(err).Panic("error creating a renderer")
	}
	defer renderer.Destroy()

	err = renderer.SetLogicalSize(int32(settings.Width), int32(settings.Height))
	if err != nil {
		log.WithError(err).Panic("error setting logical size on the renderer")
	}

	texture, err := renderer.CreateTexture(
		sdl.PIXELFORMAT_ARGB8888, sdl.TEXTUREACCESS_STATIC,
		int32(settings.Width), int32(settings.Height))
	if err != nil {
		log.WithError(err).Panic("error creating a texture on the renderer")
	}
	defer texture.Destroy()

	mandelbrotImg := NewMandelbrotImage(settings.Width, settings.Height, &settings)
	defer mandelbrotImg.Close()

	go imageWriter(mandelbrotImg, mandelbrotImg.Jobs)

	mandelbrotImg.Init()
	mandelbrotImg.ForceRender()

	running := true
	updateTexture := false
	for running {
		for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
			switch t := event.(type) {
			case *sdl.QuitEvent:
				running = false
			case *sdl.KeyboardEvent:
				keyCode := t.Keysym.Sym

				if keyCode == 113 {
					running = false
				}

				// move the set in x and y
				if keyCode == sdl.K_LEFT {
					settings.Center.X -= 0.05
					updateTexture = true
				}
				if keyCode == sdl.K_RIGHT {
					settings.Center.X += 0.05
					updateTexture = true
				}
				if keyCode == sdl.K_DOWN {
					settings.Center.Y += 0.05
					updateTexture = true
				}
				if keyCode == sdl.K_UP {
					settings.Center.Y -= 0.05
					updateTexture = true
				}

				// zoom in and out
				if keyCode == sdl.K_EQUALS {
					settings.Min += 0.15
					settings.Max -= 0.1
					settings.MaxIterations += 5
					updateTexture = true
				}
				if keyCode == sdl.K_MINUS {
					settings.Min -= 0.15
					settings.Max += 0.1
					settings.MaxIterations -= 5
					updateTexture = true
				}
			}
		}

		texture.Update(nil, mandelbrotImg.Pixels[:], 3200)
		window.UpdateSurface()

		if updateTexture {
			mandelbrotImg.ForceRender()
			updateTexture = false
		}

		renderer.Clear()
		renderer.Copy(texture, nil, nil)

		sdl.Delay(500)
		renderer.Present()
	}
}
