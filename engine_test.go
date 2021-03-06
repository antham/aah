// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/aah source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package aah

import (
	"compress/gzip"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aahframework.org/ahttp.v0"
	"aahframework.org/config.v0"
	"aahframework.org/log.v0"
	"aahframework.org/test.v0/assert"
)

type (
	Site struct {
		*Context
	}
)

func (s *Site) GetInvolved() {
	s.Session().Set("test1", "test1value")
	s.Reply().Text("GetInvolved action")
}

func (s *Site) Credits() {
	s.Reply().Header("X-Custom-Header", "custom value").
		DisableGzip().
		JSON(map[string]interface{}{
			"message": "This is credits page",
			"code":    1000001,
		})
}

func (s *Site) ContributeCode() {
	panic("panic flow testing")
}

func (s *Site) Before() {
	log.Info("Before interceptor")
}

func (s *Site) After() {
	log.Info("After interceptor")
}

func (s *Site) BeforeGetInvolved() {
	log.Info("Before GetInvolved interceptor")
}

func (s *Site) AfterGetInvolved() {
	log.Info("After GetInvolved interceptor")
}

func testEngineMiddleware(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("X-Custom-Name", "test engine middleware")
}

func TestEngineNew(t *testing.T) {
	cfgDir := filepath.Join(getTestdataPath(), appConfigDir())
	err := initConfig(cfgDir)
	assert.Nil(t, err)
	assert.NotNil(t, AppConfig())

	AppConfig().SetInt("render.gzip.level", 5)
	AppConfig().SetString("request.id.header", "X-Test-Request-Id")

	e := newEngine(AppConfig())
	assert.Equal(t, "X-Test-Request-Id", e.requestIDHeader)
	assert.True(t, e.isRequestIDEnabled)
	assert.True(t, e.isGzipEnabled)
	assert.NotNil(t, e.ctxPool)
	assert.NotNil(t, e.bufPool)
	assert.NotNil(t, e.reqPool)

	req := e.getRequest()
	ctx := e.getContext()
	ctx.Req = req
	assert.NotNil(t, ctx)
	assert.NotNil(t, req)
	assert.NotNil(t, ctx.Req)
	e.putContext(ctx)

	buf := e.getBuffer()
	assert.NotNil(t, buf)
	e.putBuffer(buf)
}

func TestEngineServeHTTP(t *testing.T) {
	// App Config
	cfgDir := filepath.Join(getTestdataPath(), appConfigDir())
	err := initConfig(cfgDir)
	assert.Nil(t, err)
	assert.NotNil(t, AppConfig())

	AppConfig().SetString("server.port", "8080")

	// Router
	err = initRoutes(cfgDir, AppConfig())
	assert.Nil(t, err)
	assert.NotNil(t, AppRouter())

	// Security
	err = initSecurity(cfgDir, AppConfig())
	assert.Nil(t, err)
	assert.True(t, AppSessionManager().IsStateful())

	// Controllers
	cRegistry = controllerRegistry{}

	AddController((*Site)(nil), []*MethodInfo{
		{
			Name:       "GetInvolved",
			Parameters: []*ParameterInfo{},
		},
		{
			Name:       "ContributeCode",
			Parameters: []*ParameterInfo{},
		},
		{
			Name:       "Credits",
			Parameters: []*ParameterInfo{},
		},
	})

	// Middlewares
	Middlewares(ToMiddleware(testEngineMiddleware))

	// Engine
	e := newEngine(AppConfig())

	// Request 1
	r1 := httptest.NewRequest("GET", "http://localhost:8080/doc/v0.3/mydoc.html", nil)
	w1 := httptest.NewRecorder()
	e.ServeHTTP(w1, r1)

	resp1 := w1.Result()
	assert.Equal(t, 404, resp1.StatusCode)
	assert.True(t, strings.Contains(resp1.Status, "Not Found"))
	assert.Equal(t, "aah-go-server", resp1.Header.Get(ahttp.HeaderServer))

	// Request 2
	r2 := httptest.NewRequest("GET", "http://localhost:8080/get-involved.html", nil)
	r2.Header.Add(ahttp.HeaderAcceptEncoding, "gzip, deflate, sdch, br")
	w2 := httptest.NewRecorder()
	e.ServeHTTP(w2, r2)

	resp2 := w2.Result()
	gr2, _ := gzip.NewReader(resp2.Body)
	body2, _ := ioutil.ReadAll(gr2)
	assert.Equal(t, 200, resp2.StatusCode)
	assert.True(t, strings.Contains(resp2.Status, "OK"))
	assert.Equal(t, "GetInvolved action", string(body2))
	assert.Equal(t, "test engine middleware", resp2.Header.Get("X-Custom-Name"))

	// Request 3
	r3 := httptest.NewRequest("GET", "http://localhost:8080/contribute-to-code.html", nil)
	w3 := httptest.NewRecorder()
	e.ServeHTTP(w3, r3)

	resp3 := w3.Result()
	body3, _ := ioutil.ReadAll(resp3.Body)
	assert.Equal(t, 500, resp3.StatusCode)
	assert.True(t, strings.Contains(resp3.Status, "Internal Server Error"))
	assert.True(t, strings.Contains(string(body3), "Internal Server Error"))

	// Request 4 static
	r4 := httptest.NewRequest("GET", "http://localhost:8080/assets/logo.png", nil)
	w4 := httptest.NewRecorder()
	e.ServeHTTP(w4, r4)

	resp4 := w4.Result()
	assert.Equal(t, 404, resp4.StatusCode)
	assert.True(t, strings.Contains(resp4.Status, "Not Found"))

	// Request 5 RedirectTrailingSlash - 302 status
	wd, _ := os.Getwd()
	appBaseDir = wd
	r5 := httptest.NewRequest("GET", "http://localhost:8080/testdata", nil)
	w5 := httptest.NewRecorder()
	e.ServeHTTP(w5, r5)

	resp5 := w5.Result()
	assert.Equal(t, 302, resp5.StatusCode)
	assert.True(t, strings.Contains(resp5.Status, "Found"))
	assert.Equal(t, "http://localhost:8080/testdata/", resp5.Header.Get(ahttp.HeaderLocation))

	// Directory Listing
	r6 := httptest.NewRequest("GET", "http://localhost:8080/testdata/", nil)
	r6.Header.Add(e.requestIDHeader, "D9391509-595B-4B92-BED7-F6A9BE0DFCF2")
	r6.Header.Add(ahttp.HeaderAcceptEncoding, "gzip, deflate, sdch, br")
	w6 := httptest.NewRecorder()
	e.ServeHTTP(w6, r6)

	resp6 := w6.Result()
	body6, _ := ioutil.ReadAll(resp6.Body)
	body6Str := string(body6)
	assert.True(t, strings.Contains(body6Str, "Listing of /testdata/"))
	assert.True(t, strings.Contains(body6Str, "config/"))

	// Custom Headers
	r7 := httptest.NewRequest("GET", "http://localhost:8080/credits", nil)
	r7.Header.Add(ahttp.HeaderAcceptEncoding, "gzip, deflate, sdch, br")
	w7 := httptest.NewRecorder()
	e.ServeHTTP(w7, r7)

	resp7 := w7.Result()
	body7, _ := ioutil.ReadAll(resp7.Body)
	body7Str := string(body7)
	assert.Equal(t, `{"code":1000001,"message":"This is credits page"}`, body7Str)
	assert.Equal(t, "custom value", resp7.Header.Get("X-Custom-Header"))

	r8 := httptest.NewRequest("POST", "http://localhost:8080/credits", nil)
	r8.Header.Add(ahttp.HeaderAcceptEncoding, "gzip, deflate, sdch, br")
	w8 := httptest.NewRecorder()
	e.ServeHTTP(w8, r8)

	// Method Not Allowed 405 response
	resp8 := w8.Result()
	reader8, _ := gzip.NewReader(resp8.Body)
	body8, _ := ioutil.ReadAll(reader8)
	assert.Equal(t, "405 Method Not Allowed", string(body8))
	assert.Equal(t, "GET, OPTIONS", resp8.Header.Get("Allow"))

	appBaseDir = ""
}

func TestEngineGzipHeaders(t *testing.T) {
	cfg, _ := config.ParseString("")
	e := newEngine(cfg)

	req := httptest.NewRequest("GET", "http://localhost:8080/doc/v0.3/mydoc.html", nil)
	req.Header.Add(ahttp.HeaderAcceptEncoding, "gzip")
	ctx := e.prepareContext(httptest.NewRecorder(), req)
	e.wrapGzipWriter(ctx)

	assert.True(t, ctx.Req.IsGzipAccepted)
	assert.Equal(t, "gzip", ctx.Res.Header().Get(ahttp.HeaderContentEncoding))
	assert.Equal(t, "Accept-Encoding", ctx.Res.Header().Get(ahttp.HeaderVary))
}
