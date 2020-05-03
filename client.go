package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	http.Client
	websocket.Dialer
	addr string
}

func NewClient(addr string) *Client {
	jar := newJar()
	return &Client{
		Client: http.Client{
			Jar: jar,
		},
		Dialer: websocket.Dialer{
			Jar: jar,
		},
		addr: addr,
	}
}

func (c *Client) Get(key Key) (Object, error) {
	url := c.url("GET")

	buf, err := c.body(key)
	if err != nil {
		return Object{}, err
	}

	req, err := http.NewRequest("POST", url, buf)
	if err != nil {
		return Object{}, err
	}

	resp, err := c.Do(req)
	if err != nil {
		return Object{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
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
	url := c.url("FIND")

	buf, err := c.body(key)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, buf)
	if err != nil {
		return nil, err
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
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

func (c *Client) Set(obj Object) error {
	url := c.url("SET")

	buf, err := c.body(obj)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, buf)
	if err != nil {
		return err
	}

	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("status: %d  message: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func (c *Client) Delete(key Key) error {
	url := c.url("DELETE")

	buf, err := c.body(key)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, buf)
	if err != nil {
		return err
	}

	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("status: %d  message: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func (c *Client) Watch(key Key) error {
	url := c.url("WATCH")

	buf, err := c.body(key)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, buf)
	if err != nil {
		return err
	}

	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("status: %d  message: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func (c *Client) Register() error {
	username := os.Getenv("CLIENT_USERNAME")
	if username == "" {
		return fmt.Errorf("'CLIENT_USERNAME' environment variable is not set")
	}

	password := os.Getenv("CLIENT_PASSWORD")
	if password == "" {
		return fmt.Errorf("'CLIENT_PASSWORD' environment variable is not set")
	}

	return c.register(username, password)
}

func (c *Client) register(username, password string) error {
	url := c.url("REGISTER")

	cred := struct {
		Username, Password string
	}{username, password}

	buf := &bytes.Buffer{}
	err := json.NewEncoder(buf).Encode(cred)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, buf)
	if err != nil {
		return err
	}

	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("status: %d  message: %s", resp.StatusCode, string(bodyBytes))
	}
	return nil
}

func (c *Client) Socket(ch chan Object) error {
	url := c.url("SOCKET")
	go func() {
		for {
			ws, _, err := c.Dial(url, nil)
			if err != nil {
				time.Sleep(time.Millisecond * 500)
				continue
			}
			c.reconcile(ch, ws)
		}
	}()
	return nil
}

func (c *Client) reconcile(ch chan Object, ws *websocket.Conn) {
	var obj Object
	for {
		err := ws.ReadJSON(&obj)
		if websocket.IsCloseError(err) {
			return
		} else if err != nil {
			continue
		}
		ch <- obj
	}
}

func (c *Client) url(action string) string {
	var u url.URL
	switch strings.ToUpper(action) {
	case "GET":
		u = url.URL{
			Scheme: "http",
			Host:   c.addr,
			Path:   "get",
		}
	case "SET":
		u = url.URL{
			Scheme: "http",
			Host:   c.addr,
			Path:   "set",
		}
	case "DELETE":
		u = url.URL{
			Scheme: "http",
			Host:   c.addr,
			Path:   "delete",
		}
	case "FIND":
		u = url.URL{
			Scheme: "http",
			Host:   c.addr,
			Path:   "/find",
		}
	case "WATCH":
		u = url.URL{
			Scheme: "http",
			Host:   c.addr,
			Path:   "/watch",
		}
	case "REGISTER":
		u = url.URL{
			Scheme: "http",
			Host:   c.addr,
			Path:   "/register",
		}
	case "SOCKET":
		u = url.URL{
			Scheme: "ws",
			Host:   c.addr,
			Path:   "/socket",
		}
	default:
		panic("unknown action")
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

	buffer := &bytes.Buffer{}
	err := json.NewEncoder(buffer).Encode(obj)
	if err != nil {
		return nil, err
	}
	return buffer, nil
}

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
	Key   Key
	Value interface{}
}
