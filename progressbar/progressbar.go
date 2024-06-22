package progressbar

import (
	"context"
	"fmt"
	"time"
)

type Progressbar struct {
	last        int
	current     int
	currentname string
	updatech    chan any
	ticker      *time.Ticker
	starttime   time.Time
}

func NewProgressbar(maximum int) *Progressbar {
	return &Progressbar{last: maximum}
}

func (p *Progressbar) Start(ctx context.Context) {
	p.starttime = time.Now()
	p.ticker = time.NewTicker(time.Second)
	p.updatech = make(chan any, 32)

outerloop:
	for {
		select {
		case <-p.ticker.C:
			p.Draw()
		case <-p.updatech:
			p.Draw()
		case <-ctx.Done():
			p.Clear()
			break outerloop
		}
	}
}

func (p *Progressbar) Add(name string) {
	p.currentname = name
	p.current++

	select {
	case p.updatech <- nil:
		break
	default:
		break
	}
}

func (p *Progressbar) Draw() {
	fmt.Printf(
		"\033[2K\r%s %d/%d %s",
		time.Since(p.starttime).Round(time.Second).String(),
		p.current,
		p.last,
		p.currentname,
	)
}

func (p *Progressbar) Clear() {
	fmt.Printf("\033[2K\r")
}
