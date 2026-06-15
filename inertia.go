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
	// Set to XMLHttpRequest on all Inertia requests.
	XRequestedWith string `header:"X-Requested-With"`
	// Set to text/html, application/xhtml+xml to indicate acceptable response types.
	Accept string `header:"Accept"`
	// The current asset version to check for asset mismatches.
	XInertiaVersion string `header:"X-Inertia-Version"`
	// Set to prefetch when making prefetch requests.
	// https://inertiajs.com/docs/v3/data-props/prefetching
	Purpose string `header:"Purpose"`
	// The component name for partial reloads.
	// https://inertiajs.com/docs/v3/data-props/partial-reloads
	XInertiaPartialComponent string `header:"X-Inertia-Partial-Component"`
	// Comma-separated list of props to include in partial reloads.
	XInertiaPartialData string `header:"X-Inertia-Partial-Data"`
	// Comma-separated list of props to exclude from partial reloads.
	XInertiaPartialExcept string `header:"X-Inertia-Partial-Except"`
	// Comma-separated list of props to reset on navigation.
	XInertiaReset string `header:"X-Inertia-Reset"`
	// Set to no-cache for reload requests to prevent serving stale content.
	CacheControl string `header:"Cache-Control"`
	// Specifies which error bag to use for validation errors.
	// https://inertiajs.com/docs/v3/the-basics/validation
	XInertiaErrorBag string `header:"X-Inertia-Error-Bag"`
	// Indicates whether the requested data should be appended or prepended when using Infinite scroll.
	// https://inertiajs.com/docs/v3/data-props/infinite-scroll
	XInertiaInfiniteScrollMergeIntent string `header:"X-Inertia-Infinite-Scroll-Merge-Intent"`
	// Comma-separated list of non-expired once prop keys already loaded on the client. The server will skip resolving
	// these props unless explicitly requested via a partial reload or force refreshed server-side.
	// https://inertiajs.com/docs/v3/data-props/once-props
	XInertiaExceptOnceProps string `header:"X-Inertia-Except-Once-Props"`
	// Set to true to indicate this is a Precognition validation request.
	// https://inertiajs.com/docs/v3/the-basics/forms#precognition
	Precognition string `header:"Precognition"`
	// Comma-separated list of field names to validate.
	// https://inertiajs.com/docs/v3/the-basics/forms#precognition
	PrecognitionValidateOnly string `header:"Precognition-Validate-Only"`
}

// Represent the Inertia response headers.
const (
	// Set to true to indicate this is an Inertia response.
	X_INERTIA string = "X-Inertia"
	// Used for external redirects when a 409 Conflict response is returned due to asset version mismatches. Triggers a
	// full window.location visit.
	X_INERTIA_LOCATION string = "X-Inertia-Location"
	// Used for redirects containing URL fragments when a 409 Conflict response is returned. Contains the full redirect
	// URL including the fragment. Triggers a standard Inertia visit instead of a full page reload.
	X_INERTIA_REDIRECT string = "X-Inertia-Redirect"
	// Set to X-Inertia to help browsers correctly differentiate between HTML and JSON responses.
	// Set to Precognition on all responses when the Precognition middleware is applied.
	VARY string = "Vary"
	// Set to true to indicate this is a Precognition validation response.
	// https://inertiajs.com/docs/v3/the-basics/forms#precognition
	PRECOGNITION string = "Precognition"
	// Set to true when validation passes with no errors, combined with a 204 No Content status code.
	// https://inertiajs.com/docs/v3/the-basics/forms#precognition
	PRECOGNITION_SUCCESS string = "Precognition-Success"
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
	// Whether or not to encrypt the current page’s history state. Only included when true.
	// https://inertiajs.com/docs/v3/security/history-encryption
	EncryptHistory bool
	// Whether or not to clear any encrypted history state. Only included when true.
	// https://inertiajs.com/docs/v3/security/history-encryption#clearing-history
	ClearHistory bool
	// Whether to preserve the URL fragment from the original request across a redirect.
	// https://inertiajs.com/docs/v3/the-basics/redirects#preserving-fragments
	PreserveFragment bool
	// Array of prop keys that should be merged (appended) during navigation.
	// https://inertiajs.com/docs/v3/data-props/merging-props
	MergeProps []string
	// Array of prop keys that should be prepended during navigation.
	// https://inertiajs.com/docs/v3/data-props/merging-props
	PrependProps []string
	// Array of prop keys that should be deep merged during navigation.
	// https://inertiajs.com/docs/v3/data-props/merging-props#deep-merge
	DeepMergeProps []string
	// Array of prop keys to use for matching when merging props.
	// https://inertiajs.com/docs/v3/data-props/merging-props#matching-items
	MatchPropsOn []string
	// Configuration for infinite scroll prop merging behavior.
	// https://inertiajs.com/docs/v3/data-props/infinite-scroll
	ScrollProps gin.H
	// Configuration for client-side lazy loading of props.
	// https://inertiajs.com/docs/v3/data-props/deferred-props
	DeferredProps gin.H
	// Array of deferred prop keys that failed to resolve and were rescued server-side. Used by the client to render the
	// rescue slot on the <Deferred> component.
	// https://inertiajs.com/docs/v3/data-props/deferred-props#error-handling
	RescuedProps []string
	// Array of top-level prop keys registered via Inertia::share(). Used by the client to carry shared props over during
	// instant visits.
	// https://inertiajs.com/docs/v3/the-basics/instant-visits
	SharedProps []string
	// Configuration for once props that should only be resolved once and reused on subsequent pages. Each entry maps a
	// key to an object containing the prop name and optional expiresAt timestamp (in milliseconds).
	// https://inertiajs.com/docs/v3/data-props/once-props
	OnceProps gin.H
}

// Init initializes Inertia by:
// - attaching its global middleware
// - serving its static assets (serves dist/assets from /assets)
// - loading its dist/index.html file
func Init(router *gin.Engine) {
	router.Use(convertRedirect(), deleteFlashData(), bindAndSetHeaders(), checkVersion())
	router.Static("/assets", "dist/assets")
	router.LoadHTMLFiles("dist/index.html")
}

// Render renders an Inertia page.
// Panics if a page cannot be JSON encoded.
func Render(context *gin.Context, component string, props gin.H) {
	allProps := gin.H{"errors": gin.H{}}
	maps.Copy(allProps, props)
	page := pageObject{
		Component: component,
		Props:     allProps,
		Url:       context.Request.URL.RequestURI(),
		Version:   getVersion(),
	}

	headers := context.MustGet("inertia.headers").(requestHeaders)

	if headers.XInertia {
		context.Header(X_INERTIA, "true")
		context.Header(VARY, X_INERTIA)
		context.JSON(http.StatusOK, page)

		return
	}

	data, error := json.Marshal(page)
	if error != nil {
		panic(error)
	}

	context.HTML(http.StatusOK, "index.html", gin.H{"data": template.JS(data)})
}

// Flash flashes a value to the current session with the default "_flash" key.
func Flash(context *gin.Context, value any) {
	session := sessions.Default(context)
	session.AddFlash(value)
	session.Save()
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
func deleteFlashData() gin.HandlerFunc {
	return func(context *gin.Context) {
		context.Next()

		status := context.Writer.Status()

		if status != http.StatusSeeOther && status != http.StatusConflict {
			session := sessions.Default(context)
			session.Delete("_flash")
			session.Save()
		}
	}
}

// bindAndSetHeaders returns a middleware handler to bind the request headers and set them to the context.
// Aborts if headers cannot be bound.
func bindAndSetHeaders() gin.HandlerFunc {
	return func(context *gin.Context) {
		headers := requestHeaders{}

		if error := context.ShouldBindHeader(&headers); error != nil {
			context.AbortWithStatus(http.StatusBadRequest)
		}

		context.Set("inertia.headers", headers)
		context.Next()
	}
}

// getVersion gets the assets version from the hashed dist/index.html file. This mimics Laravel's adapter.
// Panics if the file cannot be opened or hashed.
func getVersion() string {
	file, error := os.Open("dist/index.html")

	if error != nil {
		panic(error)
	}

	defer file.Close()

	hash := sha1.New()

	if _, error := io.Copy(hash, file); error != nil {
		panic(error)
	}

	return fmt.Sprintf("%x", hash.Sum(nil))
}

// checkVersion returns a middleware handler that compares the assets version of the request with the current version,
// based on the hash of the dist/index.html file.
// Aborts if the versions don't match.
func checkVersion() gin.HandlerFunc {
	return func(context *gin.Context) {
		headers := context.MustGet("inertia.headers").(requestHeaders)

		if !headers.XInertia || context.Request.Method != "GET" {
			context.Next()
		} else {
			version := getVersion()

			if headers.XInertiaVersion != version {
				context.Header(X_INERTIA_LOCATION, context.Request.URL.RequestURI())
				context.AbortWithStatus(http.StatusConflict)
			}

			context.Next()
		}
	}
}
