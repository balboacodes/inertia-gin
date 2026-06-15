# inertia-gin

inertia-gin is a basic [Inertia.js](https://github.com/inertiajs) adapter for [Gin](https://github.com/gin-gonic/gin).

## Features

This package currently supports rendering Inertia pages, with automatic asset version handling, and flash data. In short, it doesn't support every Inertia feature, but if you want a simple way to render Inertia pages with Gin, this will work.

## Installation

### Install inertia-gin

```
go get github.com/balboacodes/inertia-gin
```

### Install Vite

Installation instructions for Vite can be found in their [docs](https://vite.dev/guide).

### Install inertia.js

Installation instructions for Inertia can be found in their [docs](https://inertiajs.com/docs/v3/installation/client-side-setup).

### Update `index.html`

This uses the file created when setting up a new Vite project.

```html
<!doctype html>
<html lang="en">
    <head>
        <meta charset="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <link rel="icon" type="image/svg+xml" href="/favicon.svg" />
        <title>App</title>
    </head>
    <body>
        <div id="app"></div>
        <script type="module" src="/src/main.tsx"></script>
        <script data-page="app" type="application/json">
            {{ .data }}
        </script>
    </body>
</html>
```

## Usage

### Gin

```go
package main

import (
	"github.com/balboacodes/inertia-gin"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func main() {
	router := gin.Default()
	store := cookie.NewStore([]byte("your-secret-key"))
	router.Use(sessions.Sessions("mysession", store))

	inertia.Init(router)

	router.GET("/", func(context *gin.Context) {
		inertia.Render(context, "Home", gin.H{"title": "Home"})
	})

	router.POST("/", func(context *gin.Context) {
		// perform some task...

		inertia.Flash(context, "Success!")
		context.Redirect(302, "/")
	})

	router.Run()
}
```

### Inertia

This setup uses React for the frontend.

```tsx
// main.tsx
import { createInertiaApp } from "@inertiajs/react";

createInertiaApp({
  strictMode: true,
});
```

Create a `pages` directory in the `src` directory that was created during Vite setup.

```tsx
// src/pages/Home.tsx
import { Head } from "@inertiajs/react";

export default function Home({ title }: { title: string }) {
  return (
    <>
      <Head title={title} />
      <h1>{title}</h1>
    </>
  );
}
```

Run `npm run build` to build your frontend assets. Then you can run `go run .` to start your app as usual. Your app will now be running on Inertia!
