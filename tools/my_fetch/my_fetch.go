// application/json
// neo-moonchan @ 2025-02-24
// 写坏了，只能用最原始的client。
// gin-pack @ 2024-04-06
// @ 2023-12-21
// azure-go @ 2023-12-21

package myfetch

import (
	"io"
	"net/http"

	"github.com/Hana-ame/api-pack/tools/orderedmap"
)

type fetcher struct {
	count      int
	header     http.Header
	clientPool *clientPool
}

func (f *fetcher) SetDefaultHeader(header http.Header) {
	f.header = header
}

func (f *fetcher) SetClientPool(clientPool *clientPool) {
	f.clientPool = clientPool
	f.count = 0
}

func (f *fetcher) Do(req *http.Request) (*http.Response, error) {
	f.count++

	clientPool := f.clientPool
	defaultHeader := f.header

	// 如果没有被指定过，那么defaultHeader中有的东西就放进去
	for k, vs := range defaultHeader {
		if req.Header.Get(k) == "" {
			for _, v := range vs {
				req.Header.Add(k, v)
			}
		}
	}
	return clientPool.Client().Do(req)
}

func (f *fetcher) Fetch(method, url string, header http.Header, body io.Reader) (*http.Response, error) {

	req, err := NewRequest(method, url, header, body)
	if err != nil {
		return nil, err
	}

	return f.Do(req)
}

func (f *fetcher) FetchJSON(method, url string, header http.Header, body io.Reader) (*http.Response, *orderedmap.OrderedMap, error) {

	req, err := NewRequest(method, url, header, body)
	if err != nil {
		return nil, nil, err
	}

	if req.Header.Get("Accept") == "" {
		req.Header.Add("Accept", "application/json")
		req.Header.Add("Accept", "application/activity+json")
		req.Header.Add("Accept", "application/ld+json")
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Add("User-Agent", "myfetch/1.0.0")
	}
	resp, err := f.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	respObj, err := ResponseToObject(resp)
	return resp, respObj, err
}

func (f *fetcher) FetchJSONArray(method, url string, header http.Header, body io.Reader) ([]*orderedmap.OrderedMap, error) {

	req, err := NewRequest(method, url, header, body)
	if err != nil {
		return nil, err
	}

	resp, err := f.Do(req)
	if err != nil {
		return nil, err
	}

	return ResponseToObjectArray(resp)
}

func (f *fetcher) Count() int {
	return f.count
}
func (f *fetcher) AddCount(n int) {
	f.count += n
}

// defaultHeader是Fetch的时候没指定的话会加上的东西
// clientPool是fetch的时候选用的client集合
func NewFetcher(defaultHeader http.Header, clientPool *clientPool) *fetcher {
	if clientPool == nil {
		clientPool = DefaultClientPool
	}
	return &fetcher{
		header:     defaultHeader,
		clientPool: clientPool,
	}
}

// public methods
var DefaultFetcher *fetcher

func SetDefaultHeader(header http.Header) {
	DefaultFetcher.SetDefaultHeader(header)
}

// 使用默认Fetcher进行http访问
func Do(req *http.Request) (*http.Response, error) {
	return DefaultFetcher.Do(req)
}

// 使用默认Fetcher进行http访问
func Fetch(method, url string, header http.Header, body io.Reader) (*http.Response, error) {
	return DefaultFetcher.Fetch(method, url, header, body)
}

func FetchJSON(method, url string, header http.Header, body io.Reader) (*http.Response, *orderedmap.OrderedMap, error) {
	return DefaultFetcher.FetchJSON(method, url, header, body)
}

func FetchJSONArray(method, url string, header http.Header, body io.Reader) ([]*orderedmap.OrderedMap, error) {
	return DefaultFetcher.FetchJSONArray(method, url, header, body)
}
