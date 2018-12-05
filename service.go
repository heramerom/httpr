package httpr

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

type Conf struct {
	Timeout time.Duration
	Debug   bool
}

type BeforeFunc func(r *Request) (stop bool)
type BeforeRequestHook func(r *http.Request)
type AfterFunc func(r *Request, rsp *Response) (stop bool)

type Service struct {
	host          string
	hosts         []string
	paths         map[string]string
	header        http.Header
	conf          Conf
	client        *http.Client
	beforeRequest []BeforeRequestHook
	afterHooks    []AfterFunc
}

func NewService(conf *Conf) *Service {
	c := Conf{
		Timeout: 20 * time.Second,
	}
	if conf != nil {
		c = *conf
	}
	return &Service{
		conf: c,
		client: &http.Client{
			Timeout: c.Timeout,
		},
	}
}

func (s *Service) Paths(methodAndPath ...string) *Service {
	if len(methodAndPath)%2 != 0 {
		panic("method and path are not pairs")
	}
	if s.paths == nil {
		s.paths = map[string]string{}
	}
	for i := 0; i < len(methodAndPath); i += 2 {
		s.paths[methodAndPath[i]] = methodAndPath[i+1]
	}
	return s
}

func (s *Service) RawHeader(key, value string) *Service {
	s.header[key] = []string{value}
	return s
}

func (s *Service) Header(key, value string) *Service {
	s.header.Add(key, value)
	return s
}

func (s *Service) Method(method string, uriKey string) *Request {
	uri, ok := s.paths[uriKey]
	if !ok {
		panic("not found" + uriKey)
	}
	return s.Request(method, uri)
}

func (s *Service) Request(method, uri string) *Request {
	return &Request{
		method:  method,
		header:  s.header,
		conf:    s.conf,
		uri:     s.host + uri,
		service: s,
	}
}

func (s *Service) Get(uri string) *Request {
	return s.Request(http.MethodGet, uri)
}

func (s *Service) Post(uri string) *Request {
	return s.Request(http.MethodPost, uri)
}

func (s *Service) Rest(method string, params ...string) *Request {
	return s.Request(method, strings.Join(params, "/"))
}

type Request struct {
	uri           string
	conf          Conf
	method        string
	retries       []time.Duration
	header        http.Header
	service       *Service
	startAt       time.Time
	endAt         time.Time
	params        url.Values
	req           *http.Request
	beforeRequest []BeforeRequestHook
	afterHooks    []AfterFunc
}

func NewRequest(method string, uri string) *Request {
	return &Request{
		method: method,
		uri:    uri,
		conf: Conf{
			Timeout: 20 * time.Second,
		},
	}
}

func (req *Request) RetryDelay(retires ...time.Duration) *Request {
	req.retries = retires
	return req
}

func (req *Request) Params(params ...string) *Request {
	if len(params)%2 != 0 {
		panic("params error")
	}
	if req.params == nil {
		req.params = make(url.Values)
	}
	for i := 0; i < len(params); i += 2 {
		req.params.Add(params[i], params[i+1])
	}
	return req
}

func (req *Request) RawHeader(key, value string) *Request {
	if req.header == nil {
		req.header = map[string][]string{}
	}
	req.header[key] = []string{value}
	return req
}

func (req *Request) Header(key, value string) *Request {
	if req.header == nil {
		req.header = map[string][]string{}
	}
	req.header.Add(key, value)
	return req
}

func (req *Request) BeforeRequest(hooks ...BeforeRequestHook) *Request {
	req.beforeRequest = append(req.beforeRequest, hooks...)
	return req
}

func (req *Request) AfterExec(hooks ...AfterFunc) *Request {
	req.afterHooks = append(req.afterHooks, hooks...)
	return req
}

func (req *Request) Request() (r *http.Request, err error) {
	if req.req != nil {
		r = req.req
		return
	}
	if req.method == "" {
		req.method = http.MethodGet
	}
	r, err = http.NewRequest(req.method, req.uri, nil)
	if err != nil {
		return
	}
	req.req = r
	return
}

func (req *Request) _do(r *http.Request) (rsp *Response, err error) {
	resp, err := req.service.client.Do(r)
	if err != nil {
		return
	}
	rsp = &Response{
		req: req,
		rsp: resp,
	}
	return
}

func (req *Request) client() *http.Client {
	if req.service != nil {
		return req.service.client
	}
	return &http.Client{
		Timeout: req.conf.Timeout,
	}
}

func (req *Request) do() (rsp *Response, err error) {
	r, err := req.Request()
	if err != nil {
		return
	}
	req.doBeforeRequestHooks(r)
	req.startAt = time.Now()
	rsp, err = req._do(r)
	if err == nil {
		return
	}
	for _, wait := range req.retries {
		time.Sleep(wait)
		rsp, err = req._do(r)
		if err != nil {
			return
		}
	}
	return
}

func (req *Request) doBeforeRequestHooks(r *http.Request) {
	if req.service != nil {
		for _, hook := range req.service.beforeRequest {
			hook(r)
		}
	}
	for _, hook := range req.beforeRequest {
		hook(r)
	}
}

func (req *Request) doAfterHooks(rsp *Response) (stop bool) {
	if req.service != nil {
		for _, hook := range req.service.afterHooks {
			stop = hook(req, rsp)
			if stop {
				return
			}
		}
	}
	for _, hook := range req.afterHooks {
		stop = hook(req, rsp)
		if stop {
			return
		}
	}
	return
}

func (req *Request) Response() (rsp *Response, err error) {
	req.startAt = time.Now()
	rsp, err = req.do()
	req.endAt = time.Now()
	req.doAfterHooks(rsp)
	return
}

type Response struct {
	rsp  *http.Response
	req  *Request
	body []byte
	err  error
	dump bool
}

func (rsp *Response) StatusCode() int {
	return rsp.rsp.StatusCode
}

func (rsp *Response) Bytes() (bs []byte, err error) {
	if rsp.body != nil || rsp.err != nil {
		bs = rsp.body
		err = rsp.err
		return
	}
	bs, err = ioutil.ReadAll(rsp.rsp.Body)
	if err != nil {
		return
	}
	defer rsp.rsp.Body.Close()
	return
}

func (rsp *Response) ToJson(obj interface{}) (err error) {
	bs, err := rsp.Bytes()
	if err != nil {
		return
	}
	err = json.Unmarshal(bs, obj)
	return
}

func (rsp *Response) ToXML(obj interface{}) (err error) {
	bs, err := rsp.Bytes()
	if err != nil {
		return
	}
	err = xml.Unmarshal(bs, obj)
	return
}

func (rsp *Response) Dump() []byte {
	requestBytes, err := httputil.DumpRequest(rsp.req.req, true)
	if err != nil {
		return nil
	}
	responseBytes, err := httputil.DumpResponse(rsp.rsp, true)
	if err != nil {
		return nil
	}
	summary := []byte(fmt.Sprintf("\nSummary: start at %s, end at %s, cost %v\n", rsp.req.startAt, rsp.req.endAt, rsp.req.endAt.Sub(rsp.req.endAt)))
	bs := append(requestBytes, responseBytes...)
	bs = append(bs, summary...)
	return bs
}
