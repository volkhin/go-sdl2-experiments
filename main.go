package main

import (
	"fmt"
	"log"
	"math/rand"
	"runtime"
	"sync"
	"time"

	"github.com/lucasb-eyer/go-colorful"
	"github.com/veandco/go-sdl2/sdl"
	"gopkg.in/tomb.v2"
)

const (
	winTitle             = "go-sdl2-experiments"
	cellWidth, cellHeigh = 20, 20
	width, height        = 20, 20
)

var (
	cellPadding  = 0
	newCellDelay = 10 * time.Millisecond
)

type Color struct {
	colorful.Color
}

type Cell struct {
	Color Color
}

func (c Cell) String() string {
	return fmt.Sprintf("[%v]", c.Color)
}

type Grid [height][width]*Cell

func (g Grid) String() string {
	result := ""
	for i, row := range g {
		if i > 0 {
			result += "\n"
		}
		for j, c := range row {
			if j > 0 {
				result += " "
			}
			result += fmt.Sprintf("%s", c)
		}
	}
	return result
}

type App struct {
	window   *sdl.Window
	renderer *sdl.Renderer
	m        sync.Mutex
}

func (a *App) DrawGrid(g *Grid) error {
	a.m.Lock()
	defer a.m.Unlock()

	a.renderer.SetDrawColor(0, 0, 0, 255)
	a.renderer.Clear()

	for i, row := range g {
		for j, c := range row {
			rect := sdl.Rect{
				int32(j*cellWidth + cellPadding),
				int32(i*cellHeigh + cellPadding),
				int32(cellWidth - 2*cellPadding),
				int32(cellHeigh - 2*cellPadding),
			}

			var r, g, b uint8
			if c != nil {
				r, g, b = c.Color.RGB255()
			}
			a.renderer.SetDrawColor(r, g, b, 128)
			a.renderer.FillRect(&rect)
		}
	}

	a.renderer.Present()
	return nil
}

func RandomColor() Color {
	hue := rand.Float64()*180 + 240
	if hue >= 360 {
		hue -= 360
	}
	return Color{colorful.Hsv(hue, 0.5, 0.8)}
}

func GenerateGrid() *Grid {
	grid := &Grid{}
	for i, row := range grid {
		for j := range row {
			grid[i][j] = &Cell{Color: RandomColor()}
		}
	}
	return grid
}

type point struct {
	x, y int
}

func revealOneCell(grid *Grid) {
	var nilCells []point
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if grid[y][x] == nil {
				nilCells = append(nilCells, point{x: x, y: y})
			}
		}
	}

	var pos point
	if nilCells == nil {
		pos = point{x: rand.Intn(width), y: rand.Intn(height)}
	} else {
		pos = nilCells[rand.Intn(len(nilCells))]
	}

	grid[pos.y][pos.x] = &Cell{Color: RandomColor()}
}

func main() {
	runtime.LockOSThread()

	window, err := sdl.CreateWindow(
		winTitle,
		sdl.WINDOWPOS_UNDEFINED,
		sdl.WINDOWPOS_UNDEFINED,
		cellWidth*width,
		cellHeigh*height,
		sdl.WINDOW_SHOWN,
	)
	if err != nil {
		log.Fatalf("Failed to create window: %s", err)
	}
	defer window.Destroy()

	renderer, err := sdl.CreateRenderer(window, -1, sdl.RENDERER_ACCELERATED)
	if err != nil {
		log.Fatalf("Failed to create renderer: %s", err)
	}
	defer renderer.Destroy()

	renderer.Clear()

	app := &App{window: window, renderer: renderer}

	gridUpdate := make(chan *Grid, 5)
	events := make(chan sdl.Event, 100)

	appTomb := tomb.Tomb{}
	var grid Grid

	appTomb.Go(func() error {
		for {
			select {
			case <-appTomb.Dying():
				return tomb.ErrDying
			default:
				revealOneCell(&grid)
				select {
				case gridUpdate <- &grid:
				default:
				}
				time.Sleep(newCellDelay)
			}
		}
	})

	appTomb.Go(func() error {
		for event := range events {
			switch t := event.(type) {
			case *sdl.QuitEvent:
				appTomb.Killf("quit event")
			case *sdl.MouseMotionEvent:
				fmt.Printf("[%d ms] MouseMotion\ttype:%d\tid:%d\tx:%d\ty:%d\txrel:%d\tyrel:%d\n",
					t.Timestamp, t.Type, t.Which, t.X, t.Y, t.XRel, t.YRel)
			case *sdl.MouseButtonEvent:
				fmt.Printf("[%d ms] MouseButton\ttype:%d\tid:%d\tx:%d\ty:%d\tbutton:%d\tstate:%d\n",
					t.Timestamp, t.Type, t.Which, t.X, t.Y, t.Button, t.State)
			case *sdl.MouseWheelEvent:
				fmt.Printf("[%d ms] MouseWheel\ttype:%d\tid:%d\tx:%d\ty:%d\n",
					t.Timestamp, t.Type, t.Which, t.X, t.Y)
			case *sdl.KeyUpEvent:
				fmt.Printf("[%d ms] Keyboard up\ttype:%d\tsym:%c\tmodifiers:%d\tstate:%d\trepeat:%d\n",
					t.Timestamp, t.Type, t.Keysym.Sym, t.Keysym.Mod, t.State, t.Repeat)
			case *sdl.KeyDownEvent:
				fmt.Printf("[%d ms] Keyboard down\ttype:%d\tsym:%c\tmodifiers:%d\tstate:%d\trepeat:%d\n",
					t.Timestamp, t.Type, t.Keysym.Sym, t.Keysym.Mod, t.State, t.Repeat)
				if t.Keysym.Sym == 'q' {
					appTomb.Killf("q pressed")
				}
			}
		}

		return nil
	})

	for {
		for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
			events <- event
		}

		select {
		case grid := <-gridUpdate:
			app.DrawGrid(grid)
		case <-appTomb.Dying():
			close(events)
			return
		default:
		}

		time.Sleep(10 * time.Millisecond)
	}
}
