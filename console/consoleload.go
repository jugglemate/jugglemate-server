package console

import (
	"embed"
	"errors"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed web/dist
var consoleWebFS embed.FS

const ConsoleBasePath = "/jmateconsole"

func LoadConsoleWeb(eng *gin.Engine) {
	distFS, err := fs.Sub(consoleWebFS, "web/dist")
	if err != nil {
		panic(err)
	}
	httpFS := http.FS(distFS)

	serveIndex := func(ctx *gin.Context) {
		index, err := fs.ReadFile(distFS, "index.html")
		if err != nil {
			ctx.Status(http.StatusInternalServerError)
			return
		}
		ctx.Data(http.StatusOK, "text/html; charset=utf-8", index)
	}

	serveConsole := func(ctx *gin.Context) {
		filePath := strings.TrimPrefix(ctx.Param("filepath"), "/")
		if filePath == "" {
			serveIndex(ctx)
			return
		}

		file, err := distFS.Open(filePath)
		if err == nil {
			defer file.Close()
			if stat, statErr := file.Stat(); statErr == nil && !stat.IsDir() {
				ctx.FileFromFS(filePath, httpFS)
				return
			}
		} else if !errors.Is(err, fs.ErrNotExist) {
			ctx.Status(http.StatusInternalServerError)
			return
		}

		serveIndex(ctx)
	}

	eng.GET(ConsoleBasePath, serveIndex)
	eng.GET(ConsoleBasePath+"/", serveIndex)
	eng.NoRoute(func(ctx *gin.Context) {
		if ctx.Request.Method != http.MethodGet || !strings.HasPrefix(ctx.Request.URL.Path, ConsoleBasePath+"/") {
			ctx.Status(http.StatusNotFound)
			return
		}
		ctx.Params = append(ctx.Params, gin.Param{
			Key:   "filepath",
			Value: strings.TrimPrefix(ctx.Request.URL.Path, ConsoleBasePath),
		})
		serveConsole(ctx)
	})
}
