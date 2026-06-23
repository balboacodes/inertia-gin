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
	error := os.Mkdir("dist", 0755)
	if error != nil {
		panic(error)
	}

	error = os.WriteFile("dist/index.html", []byte("{{ .data }}"), 0644)
	if error != nil {
		panic(error)
	}
}

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)

	createIndexFile()

	code := m.Run()

	if error := os.RemoveAll("dist"); error != nil {
		panic(error)
	}

	os.Exit(code)
}

func TestRenderWhenInertiaRequest(t *testing.T) {
	router := gin.New()
	store := cookie.NewStore([]byte("your-secret-key"))
	router.Use(sessions.Sessions("mysession", store))

	Init(router)

	router.GET("/", func(c *gin.Context) {
		if error := Render(c, "Home", gin.H{"test": "testing"}); error != nil {
			panic(error)
		}
	})

	response := httptest.NewRecorder()
	request, error := http.NewRequest("GET", "/", nil)
	if error != nil {
		panic(error)
	}

	request.Header.Add(X_INERTIA, "true")
	version, error := getVersion()
	if error != nil {
		panic(error)
	}

	request.Header.Add("X-Inertia-Version", version)
	router.ServeHTTP(response, request)

	headers := response.Result().Header

	var resPage pageObject
	error = json.Unmarshal(response.Body.Bytes(), &resPage)
	require.NoError(t, error)

	version, error = getVersion()
	if error != nil {
		panic(error)
	}

	page := pageObject{
		Component: "Home",
		Props:     map[string]any{"errors": map[string]any{}, "test": "testing"},
		Url:       "/",
		Version:   version,
	}

	assert.Equal(t, headers.Get(X_INERTIA), "true")
	assert.Equal(t, headers.Get(VARY), X_INERTIA)
	assert.Equal(t, headers.Get("Content-Type"), "application/json; charset=utf-8")
	assert.Equal(t, response.Result().StatusCode, http.StatusOK)
	assert.Equal(t, resPage, page)
}

func TestRenderWhenNotInertiaRequest(t *testing.T) {
	router := gin.New()
	store := cookie.NewStore([]byte("your-secret-key"))
	router.Use(sessions.Sessions("mysession", store))

	Init(router)

	router.GET("/", func(c *gin.Context) {
		if error := Render(c, "Home", gin.H{"test": "testing"}); error != nil {
			panic(error)
		}
	})

	response := httptest.NewRecorder()
	request, error := http.NewRequest("GET", "/", nil)
	if error != nil {
		panic(error)
	}

	router.ServeHTTP(response, request)

	headers := response.Result().Header

	assert.Equal(t, headers.Get(X_INERTIA), "")
	assert.Equal(t, response.Result().StatusCode, http.StatusOK)
	assert.Equal(t, headers.Get("Content-Type"), "text/html; charset=utf-8")
}

func TestFlash(t *testing.T) {
	router := gin.New()
	store := cookie.NewStore([]byte("your-secret-key"))
	router.Use(sessions.Sessions("mysession", store))

	router.GET("/", func(c *gin.Context) {
		if error := Flash(c, "test"); error != nil {
			panic(error)
		}

		session := sessions.Default(c)
		assert.Equal(t, session.Flashes(), []any{"test"})

		c.JSON(http.StatusOK, "/")
	})

	response := httptest.NewRecorder()
	request, error := http.NewRequest("GET", "/", nil)
	if error != nil {
		panic(error)
	}

	router.ServeHTTP(response, request)
}

func TestWriteHeader(t *testing.T) {
	router := gin.New()
	router.Use(convertRedirect())

	router.GET("/redirect", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/")

		assert.IsType(t, c.Writer, &redirectWriter{})
	})

	response := httptest.NewRecorder()
	request, error := http.NewRequest("GET", "/redirect", nil)
	if error != nil {
		panic(error)
	}

	router.ServeHTTP(response, request)
}

func TestConvertRedirectMiddleware(t *testing.T) {
	router := gin.New()
	router.Use(convertRedirect())

	router.GET("/redirect", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/")
	})

	response := httptest.NewRecorder()
	request, error := http.NewRequest("GET", "/redirect", nil)
	if error != nil {
		panic(error)
	}

	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusSeeOther, response.Result().StatusCode)
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
		if error := session.Save(); error != nil {
			panic(error)
		}

		assert.Equal(t, session.Flashes(), []any{"test"})

		c.Redirect(http.StatusFound, "/")

	})

	response := httptest.NewRecorder()
	request, error := http.NewRequest("GET", "/redirect", nil)
	if error != nil {
		panic(error)
	}

	router.ServeHTTP(response, request)

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

	response := httptest.NewRecorder()
	request, error := http.NewRequest("GET", "/", nil)
	if error != nil {
		panic(error)
	}

	router.ServeHTTP(response, request)
}

func TestGetVersion(t *testing.T) {
	assert.NotPanics(t, func() { getVersion() })
}

func TestGetVersionReturnsErrorWhenNoFile(t *testing.T) {
	if error := os.RemoveAll("dist"); error != nil {
		panic(error)
	}

	_, error := getVersion()
	assert.Error(t, error)

	createIndexFile()
}

func TestCheckVersionMiddlewareWhenNotInertiaRequest(t *testing.T) {
	router := gin.New()
	router.Use(bindAndSetHeaders(), checkVersion())

	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, "/")
	})

	response := httptest.NewRecorder()
	request, error := http.NewRequest("GET", "/", nil)
	if error != nil {
		panic(error)
	}

	router.ServeHTTP(response, request)

	assert.Equal(t, response.Result().StatusCode, http.StatusOK)
}

func TestCheckVersionMiddlewareWhenVersionIsSame(t *testing.T) {
	router := gin.New()
	router.Use(bindAndSetHeaders(), checkVersion())

	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, "/")
	})

	response := httptest.NewRecorder()
	request, error := http.NewRequest("GET", "/", nil)
	if error != nil {
		panic(error)
	}

	request.Header.Add(X_INERTIA, "true")
	version, error := getVersion()
	if error != nil {
		panic(error)
	}

	request.Header.Add("X-Inertia-Version", version)
	router.ServeHTTP(response, request)

	assert.Equal(t, response.Result().StatusCode, http.StatusOK)
}

func TestCheckVersionMiddlewareWhenVersionIsDifferent(t *testing.T) {
	router := gin.New()
	router.Use(bindAndSetHeaders(), checkVersion())

	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, "/")
	})

	response := httptest.NewRecorder()
	request, error := http.NewRequest("GET", "/", nil)
	if error != nil {
		panic(error)
	}

	request.Header.Add(X_INERTIA, "true")
	request.Header.Add("X-Inertia-Version", "123")
	router.ServeHTTP(response, request)

	assert.Equal(t, response.Result().Header.Get(X_INERTIA_LOCATION), "/")
	assert.Equal(t, response.Result().StatusCode, http.StatusConflict)
}
