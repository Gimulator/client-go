package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	id string
	http.Client
	websocket.Dialer
	addr string
	ch   chan Object
}

func NewClient(ch chan Object) (*Client, error) {
	addr := os.Getenv("GIMULATOR_HOST")
	if addr == "" {
		return nil, fmt.Errorf("'GIMULATOR_HOST' environment variable is not set")
	}

	id := os.Getenv("CLIENT_ID")
	if id == "" {
		return nil, fmt.Errorf("'CLIENT_ID' environment variable is not set")
	}

	jar := newJar()
	cli := &Client{
		Client: http.Client{
			Jar: jar,
		},
		Dialer: websocket.Dialer{
			Jar: jar,
		},
		addr: addr,
		id:   id,
		ch:   ch,
	}

	if err := cli.register(); err != nil {
		return nil, err
	}

	if err := cli.socket(); err != nil {
		return nil, err
	}

	return cli, nil
}

func (c *Client) Get(key Key) (Object, error) {
	url := c.url(urlPathGet)

	buf, err := c.body(key)
	if err != nil {
		return Object{}, err
	}

	req, err := http.NewRequest(http.MethodPost, url, buf)
	if err != nil {
		return Object{}, err
	}

	resp, err := c.Do(req)
	if err != nil {
		return Object{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return Object{}, err
		}
		return Object{}, fmt.Errorf("status: %d  message: %s", resp.StatusCode, string(bodyBytes))
	}

	var obj Object
	if err := json.NewDecoder(resp.Body).Decode(&obj); err != nil {
		return Object{}, err
	}

	return obj, nil
}

func (c *Client) Find(key Key) ([]Object, error) {
	url := c.url(urlPathFind)

	buf, err := c.body(key)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, url, buf)
	if err != nil {
		return nil, err
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("status: %d  message: %s", resp.StatusCode, string(bodyBytes))
	}

	var objectList []Object
	if err := json.NewDecoder(resp.Body).Decode(&objectList); err != nil {
		return nil, err
	}

	return objectList, nil
}

func (c *Client) Set(key Key, val interface{}) error {
	obj := Object{Key: key, Value: val}
	url := c.url(urlPathSet)

	buf, err := c.body(obj)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, url, buf)
	if err != nil {
		return err
	}

	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("status: %d  message: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func (c *Client) Delete(key Key) error {
	url := c.url(urlPathDelete)

	buf, err := c.body(key)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, url, buf)
	if err != nil {
		return err
	}

	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("status: %d  message: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func (c *Client) Watch(key Key) error {
	url := c.url(urlPathWatch)

	buf, err := c.body(key)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, url, buf)
	if err != nil {
		return err
	}

	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("status: %d  message: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func (c *Client) register() error {
	url := c.url(urlPathRegister)

	cred := struct {
		ID string
	}{c.id}

	buf := &bytes.Buffer{}
	err := json.NewEncoder(buf).Encode(cred)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, url, buf)
	if err != nil {
		return err
	}

	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("status: %d  message: %s", resp.StatusCode, string(bodyBytes))
	}
	return nil
}

func (c *Client) socket() error {
	url := c.url(urlPathSocket)
	go func() {
		for {
			ws, _, err := c.Dial(url, nil)
			if err != nil {
				time.Sleep(time.Millisecond * 500)
				continue
			}
			c.reconcile(ws)
		}
	}()
	return nil
}

func (c *Client) reconcile(ws *websocket.Conn) {
	var obj Object
	for {
		err := ws.ReadJSON(&obj)
		if websocket.IsCloseError(err) {
			return
		} else if err != nil {
			continue
		}
		c.ch <- obj
	}
}

type urlPath string

const (
	urlPathGet      urlPath = "get"
	urlPathSet      urlPath = "set"
	urlPathFind     urlPath = "find"
	urlPathDelete   urlPath = "delete"
	urlPathWatch    urlPath = "watch"
	urlPathRegister urlPath = "register"
	urlPathSocket   urlPath = "socket"
)

func (c *Client) url(path urlPath) string {
	u := url.URL{
		Scheme: "http",
		Host:   c.addr,
		Path:   string(path),
	}

	if path == urlPathSocket {
		u.Scheme = "ws"
	}

	return u.String()
}

func (c *Client) body(i interface{}) (*bytes.Buffer, error) {
	var obj Object
	switch i.(type) {
	case Object:
		obj = i.(Object)
	case Key:
		obj = Object{
			Key:   i.(Key),
			Value: "",
		}
	default:
		return nil, fmt.Errorf("invalid type. type must be 'object' or 'key'")
	}
	obj.Owner = c.id

	buffer := &bytes.Buffer{}
	err := json.NewEncoder(buffer).Encode(obj)
	if err != nil {
		return nil, err
	}
	return buffer, nil
}

// **************************** Jar ****************************
// It is here to handle cookies

type Jar struct {
	sync.Mutex
	cookies map[string][]*http.Cookie
}

func newJar() *Jar {
	jar := new(Jar)
	jar.cookies = make(map[string][]*http.Cookie)
	return jar
}

// SetCookies handles the receipt of the cookies in a reply for the
// given URL.  It may or may not choose to save the cookies, depending
// on the jar's policy and implementation.
func (jar *Jar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	jar.Lock()
	jar.cookies[u.Host] = cookies
	jar.Unlock()
}

// Cookies returns the cookies to send in a request for the given URL.
// It is up to the implementation to honor the standard cookie use
// restrictions such as in RFC 6265.
func (jar *Jar) Cookies(u *url.URL) []*http.Cookie {
	jar.Lock()
	defer jar.Unlock()
	return jar.cookies[u.Host]
}

// **************************** Object ****************************
// It is what Gimulator uses in communications

type Key struct {
	Type      string
	Namespace string
	Name      string
}

func (k Key) Equal(key Key) bool {
	if k.Type != key.Type {
		return false
	} else if k.Namespace != key.Namespace {
		return false
	} else if k.Name != key.Name {
		return false
	}
	return true
}

func (k Key) Match(key Key) bool {
	if k.Type != "" && k.Type != key.Type {
		return false
	} else if k.Namespace != "" && k.Namespace != key.Namespace {
		return false
	} else if k.Name != "" && k.Name != key.Name {
		return false
	}
	return true
}

type Object struct {
	Owner string
	Key   Key
	Value interface{}
}
