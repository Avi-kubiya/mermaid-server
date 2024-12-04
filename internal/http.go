package internal

import (
	"encoding/json"
	"fmt"
	"github.com/tomwright/grace"
	"github.com/tomwright/gracehttpserverrunner"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// NewHTTPRunner returns a grace runner that runs a HTTP server.
func NewHTTPRunner(generator Generator, allowAllOrigins bool) grace.Runner {
	httpHandler := generateHTTPHandler(generator)

	if allowAllOrigins {
		httpHandler = allowAllOriginsMiddleware(httpHandler)
	}

	r := http.NewServeMux()
	r.Handle("/generate", httpHandler)

	return &gracehttpserverrunner.HTTPServerRunner{
		Server: &http.Server{
			Addr:    ":80",
			Handler: r,
		},
		ShutdownTimeout: time.Second * 5,
	}
}

// allowAllOriginsMiddleware sets appropriate CORS headers to allow requests from any origin.
func allowAllOriginsMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		h.ServeHTTP(w, r)
	})
}

func writeJSON(rw http.ResponseWriter, value interface{}, status int) {
	bytes, err := json.Marshal(value)
	if err != nil {
		panic("could not marshal value: " + err.Error())
	}
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(status)
	if _, err := rw.Write(bytes); err != nil {
		panic("could not write bytes to response: " + err.Error())
	}
}

func writeImage(rw http.ResponseWriter, data []byte, status int, imgType string) error {
	switch imgType {
	case "png":
		rw.Header().Set("Content-Type", "image/png")
	case "svg":
		rw.Header().Set("Content-Type", "image/svg+xml")
	default:
		return fmt.Errorf("unhandled image type: %s", imgType)
	}
	rw.WriteHeader(status)
	if _, err := rw.Write(data); err != nil {
		return fmt.Errorf("could not write image bytes: %w", err)
	}
	return nil
}

func writeErr(rw http.ResponseWriter, err error, status int) {
	log.Printf("[%d] %s", status, err)

	writeJSON(rw, map[string]interface{}{
		"error": err,
	}, status)
}

// URLParam is the URL parameter getDiagramFromGET uses to look for data.
const URLParam = "data"

func getDiagramFromGET(r *http.Request, imgType string) (*Diagram, error) {
	if r.Method != http.MethodGet {
		return nil, fmt.Errorf("expected HTTP method GET")
	}

	queryVal := strings.TrimSpace(r.URL.Query().Get(URLParam))
	if queryVal == "" {
		return nil, fmt.Errorf("missing data")
	}
	data, err := url.QueryUnescape(queryVal)
	if err != nil {
		return nil, fmt.Errorf("could not read query param: %s", err)
	}

	// Create a diagram from the description
	d := NewDiagram([]byte(data), imgType)
	return d, nil
}

func getDiagramFromPOST(r *http.Request, imgType string) (*Diagram, error) {
	if r.Method != http.MethodPost {
		return nil, fmt.Errorf("expected HTTP method POST")
	}
	// Get description from request body
	bytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read body: %s", err)
	}

	// Create a diagram from the description
	d := NewDiagram(bytes, imgType)
	return d, nil
}

const URLParamImageType = "type"
const URLParamImageScale = "scale"

// generateHTTPHandler returns a HTTP handler used to generate a diagram.
func generateHTTPHandler(generator Generator) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		var diagram *Diagram

		imgType := r.URL.Query().Get(URLParamImageType)

		switch imgType {
		case "png", "svg":
		case "":
			imgType = "svg"
		default:
			writeErr(rw, fmt.Errorf("unsupported image type (%s) use svg or png", imgType), http.StatusBadRequest)
			return
		}

		var err error
		switch r.Method {
		case http.MethodGet:
			diagram, err = getDiagramFromGET(r, imgType)
		case http.MethodPost:
			diagram, err = getDiagramFromPOST(r, imgType)
		default:
			writeErr(rw, fmt.Errorf("unexpected HTTP method %s", r.Method), http.StatusBadRequest)
			return
		}
		if err != nil {
			writeErr(rw, err, http.StatusBadRequest)
			return
		}
		if diagram == nil {
			writeErr(rw, fmt.Errorf("could not create diagram"), http.StatusInternalServerError)
			return
		}

		diagram.scale = "10"
		scaleStr := r.URL.Query().Get(URLParamImageScale)
		if scaleStr != "" {
			scale, err := strconv.Atoi(scaleStr) // Convert string to integer

			if err != nil || scale < 1 || scale > 100 {
				http.Error(rw, "Invalid scale parameter. It must be a number between 1 and 100.", http.StatusBadRequest)
				return
			}

			diagram.scale = scaleStr
		}

		// Generate the diagram
		if err := generator.Generate(diagram); err != nil {
			writeErr(rw, fmt.Errorf("could not generate diagram: %s", err), http.StatusInternalServerError)
			return
		}

		// Output the diagram as an SVG.
		// We assume generate always generates an SVG at this point in time.
		diagramBytes, err := ioutil.ReadFile(diagram.Output)
		if err != nil {
			writeErr(rw, fmt.Errorf("could not read diagram bytes: %s", err), http.StatusInternalServerError)
			return
		}
		if err := writeImage(rw, diagramBytes, http.StatusOK, imgType); err != nil {
			writeErr(rw, fmt.Errorf("could not write diagram: %w", err), http.StatusInternalServerError)
		}
	})
}
