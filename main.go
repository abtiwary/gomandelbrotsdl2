package main

import (
	"math"
	"os"
	"sync"
	"sync/atomic"

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
	Renderer *sdl.Renderer
}

func NewMandelbrotImage(renderer *sdl.Renderer) *MandelbrotImage {
	return &MandelbrotImage{
		Renderer: renderer,
	}
}

func (mi *MandelbrotImage) DrawPoint(point Point) {
	mi.mu.Lock()
	defer mi.mu.Unlock()
	err := mi.Renderer.SetDrawColor(point.Red, point.Green, point.Blue, 255)
	if err != nil {
		log.WithError(err).Debug("error setting draw color on renderer")
	}
	mi.Renderer.DrawPoint(
		int32(point.X),
		int32(point.Y),
	)
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

func mandelbrotWorker(wg *sync.WaitGroup, count *uint64, pt Point, jobs chan Point, settings *Settings) {
	defer wg.Done()

	i := pt.X
	j := pt.Y
	//fmt.Println(i, j)

	x := mapToRange(float64(i), 0, settings.Width, settings.Min, settings.Max)
	y := mapToRange(float64(j), 0, settings.Height, settings.Min, settings.Max)

	x = x - settings.Center.X
	y = y - settings.Center.Y

	x0 := x
	y0 := y

	var iters int64
	var z int64
	for z = 0; z < settings.MaxIterations; z++ {
		//log.WithField("max iterations", settings.MaxIterations).Debug()
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
	atomic.AddUint64(count, 1)
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
		Min:           -1.00,
		Max:           1.00,
		MaxIterations: 200,
		Center: Point{
			X: 0.5,
			Y: 0.0,
		},
	}

	var wg sync.WaitGroup
	var jobCount uint64

	jobs := make(chan Point)

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

	mandelbrotImg := NewMandelbrotImage(renderer)
	go imageWriter(mandelbrotImg, jobs)

	running := true
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
				if keyCode == 1073741904 {
					settings.Center.X -= 0.05
				}
				if keyCode == 1073741903 {
					settings.Center.X += 0.05
				}
				if keyCode == 1073741905 {
					settings.Center.Y += 0.05
				}
				if keyCode == 1073741906 {
					settings.Center.Y -= 0.05
				}

				if keyCode == 61 {
					settings.Min += 0.15
					settings.Max -= 0.1
					settings.MaxIterations += 5
				}
				if keyCode == 45 {
					settings.Min -= 0.15
					settings.Max += 0.1
					settings.MaxIterations -= 5
				}
			}
		}

		var i int64
		var j int64
		for i = 0; i < int64(settings.Width); i++ {
			for j = 0; j < int64(settings.Height); j++ {
				pt := Point{
					X: float64(i),
					Y: float64(j),
				}
				wg.Add(1)
				go mandelbrotWorker(&wg, &jobCount, pt, jobs, &settings)
			}
		}

		wg.Wait()
		renderer.Present()
	}
}
