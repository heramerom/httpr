package httpr

import (
	"context"
	"sync"
)

type Group struct {
	requests []*Request
	sync     chan *ResponseWrapper
	async    chan *ResponseWrapper
	next     *context.CancelFunc
	stop     *context.CancelFunc
}

func NewGroup(req ...*Request) *Group {
	return &Group{
		requests: req,
	}
}

func (g *Group) Continue() {
	if g.next != nil {
		(*g.next)()
	}
}

func (g *Group) Stop() {
	if g.stop != nil {
		(*g.stop)()
	}
}

type ResponseWrapper struct {
	Response *Response
	Err      error
}

func (g *Group) Sync() <-chan *ResponseWrapper {
	if g.sync != nil {
		return g.sync
	}
	g.sync = make(chan *ResponseWrapper)
	go func() {
		defer func() {
			close(g.sync)
			g.sync = nil
		}()
		for _, req := range g.requests {
			rsp, err := req.Response()
			next, nextFunc := context.WithCancel(context.Background())
			g.next = &nextFunc
			stop, stopFunc := context.WithCancel(context.Background())
			g.stop = &stopFunc
			g.sync <- &ResponseWrapper{
				Response: rsp,
				Err:      err,
			}
			select {
			case <-next.Done():
			case <-stop.Done():
				return
			}
		}
	}()
	return g.sync
}

func (g *Group) Async() <-chan *ResponseWrapper {
	if g.async != nil {
		return g.async
	}
	g.async = make(chan *ResponseWrapper, len(g.requests))
	go func() {
		var wg sync.WaitGroup
		for _, req := range g.requests {
			wg.Add(1)
			go func() {
				defer wg.Done()
				rsp, err := req.Response()
				g.async <- &ResponseWrapper{
					Response: rsp,
					Err:      err,
				}
			}()
		}
		wg.Wait()
		close(g.async)
		g.async = nil
	}()
	return g.async
}
