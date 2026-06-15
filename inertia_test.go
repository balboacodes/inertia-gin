package inertia

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createIndexFile() {
	err := os.Mkdir("dist", 0755)
	if err != nil {
		panic(err)
	}

	err = os.WriteFile("dist/index.html", []byte("{{ .data }}"), 0644)
	if err != nil {
		panic(err)
	}
}

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)

	createIndexFile()

	code := m.Run()

	os.RemoveAll("dist")

	os.Exit(code)
}

func TestRenderWhenInertiaRequest(t *testing.T) {
	router := gin.New()
	store := cookie.NewStore([]byte("your-secret-key"))
	router.Use(sessions.Sessions("mysession", store))

	Init(router)

	router.GET("/", func(c *gin.Context) {
		Render(c, "Home", gin.H{"test": "testing"})
	})

	res := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Add(X_INERTIA, "true")
	req.Header.Add("X-Inertia-Version", getVersion())
	router.ServeHTTP(res, req)

	headers := res.Result().Header

	var resPage pageObject
	err := json.Unmarshal(res.Body.Bytes(), &resPage)
	require.NoError(t, err)

	page := pageObject{
		Component: "Home",
		Props:     map[string]any{"errors": map[string]any{}, "test": "testing"},
		Url:       "/",
		Version:   getVersion(),
	}

	assert.Equal(t, headers.Get(X_INERTIA), "true")
	assert.Equal(t, headers.Get(VARY), X_INERTIA)
	assert.Equal(t, headers.Get("Content-Type"), "application/json; charset=utf-8")
	assert.Equal(t, res.Result().StatusCode, http.StatusOK)
	assert.Equal(t, resPage, page)
}

func TestRenderWhenNotInertiaRequest(t *testing.T) {
	router := gin.New()
	store := cookie.NewStore([]byte("your-secret-key"))
	router.Use(sessions.Sessions("mysession", store))

	Init(router)

	router.GET("/", func(c *gin.Context) {
		Render(c, "Home", gin.H{"test": "testing"})
	})

	res := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	router.ServeHTTP(res, req)

	headers := res.Result().Header

	assert.Equal(t, headers.Get(X_INERTIA), "")
	assert.Equal(t, res.Result().StatusCode, http.StatusOK)
	assert.Equal(t, headers.Get("Content-Type"), "text/html; charset=utf-8")
}

func TestFlash(t *testing.T) {
	router := gin.New()
	store := cookie.NewStore([]byte("your-secret-key"))
	router.Use(sessions.Sessions("mysession", store))

	router.GET("/", func(c *gin.Context) {
		Flash(c, "test")

		session := sessions.Default(c)
		assert.Equal(t, session.Flashes(), []any{"test"})

		c.JSON(http.StatusOK, "/")
	})

	res := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	router.ServeHTTP(res, req)
}

func TestWriteHeader(t *testing.T) {
	router := gin.New()
	router.Use(convertRedirect())

	router.GET("/redirect", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/")

		assert.IsType(t, c.Writer, &redirectWriter{})
	})

	res := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/redirect", nil)
	router.ServeHTTP(res, req)
}

func TestConvertRedirectMiddleware(t *testing.T) {
	router := gin.New()
	router.Use(convertRedirect())

	router.GET("/redirect", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/")
	})

	res := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/redirect", nil)
	router.ServeHTTP(res, req)

	assert.Equal(t, http.StatusSeeOther, res.Result().StatusCode)
}

func TestDeleteFlashDataMiddleware(t *testing.T) {
	router := gin.New()
	store := cookie.NewStore([]byte("your-secret-key"))
	router.Use(sessions.Sessions("mysession", store))
	router.Use(deleteFlashData())

	var session sessions.Session

	router.GET("/redirect", func(c *gin.Context) {
		session = sessions.Default(c)
		session.AddFlash("test")
		session.Save()

		assert.Equal(t, session.Flashes(), []any{"test"})

		c.Redirect(http.StatusFound, "/")

	})

	res := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/redirect", nil)
	router.ServeHTTP(res, req)

	assert.Equal(t, session.Flashes(), []any([]any(nil)))
}

func TestBindAndSetHeadersMiddleware(t *testing.T) {
	router := gin.New()
	router.Use(bindAndSetHeaders())

	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, "/")

		_, exists := c.Get("inertia.headers")
		assert.True(t, exists)
	})

	res := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	router.ServeHTTP(res, req)
}

func TestGetVersion(t *testing.T) {
	assert.NotPanics(t, func() { getVersion() })
}

func TestGetVersionPanicsWhenNoFile(t *testing.T) {
	os.RemoveAll("dist")

	assert.Panics(t, func() { getVersion() })

	createIndexFile()
}

func TestCheckVersionMiddlewareWhenNotInertiaRequest(t *testing.T) {
	router := gin.New()
	router.Use(bindAndSetHeaders(), checkVersion())

	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, "/")
	})

	res := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	router.ServeHTTP(res, req)

	assert.Equal(t, res.Result().StatusCode, http.StatusOK)
}

func TestCheckVersionMiddlewareWhenVersionIsSame(t *testing.T) {
	router := gin.New()
	router.Use(bindAndSetHeaders(), checkVersion())

	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, "/")
	})

	res := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Add(X_INERTIA, "true")
	req.Header.Add("X-Inertia-Version", getVersion())
	router.ServeHTTP(res, req)

	assert.Equal(t, res.Result().StatusCode, http.StatusOK)
}

func TestCheckVersionMiddlewareWhenVersionIsDifferent(t *testing.T) {
	router := gin.New()
	router.Use(bindAndSetHeaders(), checkVersion())

	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, "/")
	})

	res := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Add(X_INERTIA, "true")
	req.Header.Add("X-Inertia-Version", "123")
	router.ServeHTTP(res, req)

	assert.Equal(t, res.Result().Header.Get(X_INERTIA_LOCATION), "/")
	assert.Equal(t, res.Result().StatusCode, http.StatusConflict)
}
