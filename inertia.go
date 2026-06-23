// Package inertia provides the functionality to use Inertia.js with Gin.
// https://inertiajs.com
package inertia

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"maps"
	"net/http"
	"os"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// requestHeaders represent the headers included in an Inertia request.
type requestHeaders struct {
	// Set to true to indicate this is an Inertia request.
	XInertia bool `header:"X-Inertia"`
	// The current asset version to check for asset mismatches.
	XInertiaVersion string `header:"X-Inertia-Version"`
}

// Represents the Inertia response headers.
const (
	// Set to true to indicate this is an Inertia response.
	X_INERTIA string = "X-Inertia"
	// Used for external redirects when a 409 Conflict response is returned due to asset version mismatches. Triggers a
	// full window.location visit.
	X_INERTIA_LOCATION string = "X-Inertia-Location"
	// Set to X-Inertia to help browsers correctly differentiate between HTML and JSON responses.
	// Set to Precognition on all responses when the Precognition middleware is applied.
	VARY string = "Vary"
)

// pageObject represents an Inertia page object.
type pageObject struct {
	// The name of the JavaScript page component.
	Component string `json:"component"`
	// The page props. Contains all of the page data along with an errors object (defaults to {} if there are no errors).
	Props gin.H `json:"props"`
	// The page URL.
	Url string `json:"url"`
	// The current asset version.
	// https://inertiajs.com/docs/v3/advanced/asset-versioning
	Version string `json:"version"`
}

// Init initializes Inertia by attaching its global middleware, serving its static assets (serves dist/assets from /assets), and loading its dist/index.html file.
func Init(router *gin.Engine) {
	router.Use(convertRedirect(), deleteFlashData(), bindAndSetHeaders(), checkVersion())
	router.Static("/assets", "dist/assets")
	router.LoadHTMLFiles("dist/index.html")
}

// Render renders an Inertia page.
// Returns error if the version was not retrieved or if the page was not JSON encoded.
func Render(context *gin.Context, component string, props gin.H) error {
	allProps := gin.H{"errors": gin.H{}}
	maps.Copy(allProps, props)

	version, error := getVersion()
	if error != nil {
		return error
	}

	page := pageObject{
		Component: component,
		Props:     allProps,
		Url:       context.Request.URL.RequestURI(),
		Version:   version,
	}

	headers := context.MustGet("inertia.headers").(requestHeaders)

	if headers.XInertia {
		context.Header(X_INERTIA, "true")
		context.Header(VARY, X_INERTIA)
		context.JSON(http.StatusOK, page)

		return nil
	}

	data, error := json.Marshal(page)
	if error != nil {
		return error
	}

	context.HTML(http.StatusOK, "index.html", gin.H{"data": template.JS(data)})
	return nil
}

// Flash flashes a value to the current session with the default "_flash" key.
// Returns error if session was not saved.
func Flash(context *gin.Context, value any) error {
	session := sessions.Default(context)
	session.AddFlash(value)
	return session.Save()
}

// redirectWriter represents the ResponseWriter for redirects.
type redirectWriter struct {
	gin.ResponseWriter
}

// WriteHeader converts 302 redirects to 303.
func (writer *redirectWriter) WriteHeader(code int) {
	if code == http.StatusFound {
		code = http.StatusSeeOther
	}

	writer.ResponseWriter.WriteHeader(code)
}

// convertRedirect returns a middleware handler that converts 302 redirects to 303.
func convertRedirect() gin.HandlerFunc {
	return func(context *gin.Context) {
		context.Writer = &redirectWriter{context.Writer}
		context.Next()
	}
}

// deleteFlashData returns a middleware handler that deletes flash data after non-303 and 409 responses.
// Aborts if session was not saved.
func deleteFlashData() gin.HandlerFunc {
	return func(context *gin.Context) {
		context.Next()

		status := context.Writer.Status()

		if status != http.StatusSeeOther && status != http.StatusConflict {
			session := sessions.Default(context)
			session.Delete("_flash")
			error := session.Save()
			if error != nil {
				context.AbortWithError(http.StatusInternalServerError, error)
			}
		}
	}
}

// bindAndSetHeaders returns a middleware handler to bind the request headers and set them to the context.
// Aborts if headers were not bound.
func bindAndSetHeaders() gin.HandlerFunc {
	return func(context *gin.Context) {
		headers := requestHeaders{}

		if error := context.ShouldBindHeader(&headers); error != nil {
			context.AbortWithError(http.StatusBadRequest, error)
		}

		context.Set("inertia.headers", headers)
		context.Next()
	}
}

// getVersion gets the assets version from the hashed dist/index.html file. This mimics Laravel's adapter.
// Returns error if the file cannot be opened or hashed.
func getVersion() (string, error) {
	file, error := os.Open("dist/index.html")
	if error != nil {
		return "", error
	}
	defer file.Close()

	hash := sha1.New()
	if _, error := io.Copy(hash, file); error != nil {
		return "", error
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// checkVersion returns a middleware handler that compares the assets version of the request with the current version,
// based on the hash of the dist/index.html file.
// Aborts if the versions do not match.
func checkVersion() gin.HandlerFunc {
	return func(context *gin.Context) {
		headers := context.MustGet("inertia.headers").(requestHeaders)

		if !headers.XInertia || context.Request.Method != "GET" {
			context.Next()
		} else {
			version, error := getVersion()
			if error != nil {
				context.AbortWithError(http.StatusInternalServerError, error)
			}

			if headers.XInertiaVersion != version {
				context.Header(X_INERTIA_LOCATION, context.Request.URL.RequestURI())
				context.AbortWithStatus(http.StatusConflict)
			}

			context.Next()
		}
	}
}
