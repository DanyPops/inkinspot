package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

var (
	ErrImageStoreEmpty   = errors.New("image store empty")
	ErrImageStoreTimeout = errors.New("image store timeout")
	ErrSearchEmptyQuery  = errors.New("search empty query")
)

// WithTightTimeout returns a child context that expires at the earlier of (now + d) and the parent's deadline.
func WithTightTimeout(parent context.Context, duration time.Duration) (context.Context, context.CancelFunc) {
	if parentDeadline, ok := parent.Deadline(); ok {
		internalDeadline := time.Now().Add(duration)
		// if the parent deadline expires earlier use it instead.
		if internalDeadline.After(parentDeadline) {
			return context.WithCancel(parent)
		}

		return context.WithDeadline(parent, internalDeadline)
	}

	return context.WithTimeout(parent, duration)
}

// TimeoutPolicy holds all the timeout policies for the search engine components
type TimeoutPolicy struct {
	ImageStoreTimeout  time.Duration
	VectorStoreTimeout time.Duration
}

// Configuration holds all the top-level policies for the search engine
type Configuration struct {
	TimeoutPolicy TimeoutPolicy
}

// LabelSet is a set of string & value pairs.
// The value represents the proximity rating.
// To the string's defintion.
type LabelSet map[string]float64

// TattooImagesVector holds the embedded characteristics.
// Style, Subject & Area are label sets which represent.
// The tattoo styles (black & white, realistic, etc).
// The tattoo subjects (lion, sword, etc).
// The tattoo anatomical area (arm, chest, etc)
type TattooImagesVector struct {
	ID      string
	Style   LabelSet
	Subject LabelSet
	Area    LabelSet
}

// TattooImagesCollection URLs are links to the photos of the tattoo.
type TattooImagesCollection struct {
	ID   string
	URLs []string
}

// ImageStore defines the contract.
// Of the service which stores the tattoo images.
type ImageStore interface {
	GetTattoosByID(ctx context.Context, ids []string) ([]TattooImagesCollection, error)
}

// VectorStore defines the contract.
// Of the service which stores the embedded
// data about the tattoos.
type VectorStore interface {
	GetIDsByQuery(ctx context.Context, query string) ([]string, error)
}

// SearchEngine peforms searching for tattoos.
type SearchEngine struct {
	configuration Configuration
	imageStore    ImageStore
	vectorStore   VectorStore
}

// NewSearchEngine creates a new search engine instance.
func NewSearchEngine(cfg Configuration, ts ImageStore, vs VectorStore) *SearchEngine {
	return &SearchEngine{
		configuration: cfg,
		imageStore:    ts,
		vectorStore:   vs,
	}
}

// Search returns a list of tattoo images by their query match rating.
// And all the images which are related to it.
func (e *SearchEngine) Search(ctx context.Context, query string) ([]TattooImagesCollection, error) {
	query = normalizeQuery(query)
	if query == "" {
		return nil, ErrSearchEmptyQuery
	}

	vqCtx, vqCancel := WithTightTimeout(ctx, e.configuration.TimeoutPolicy.VectorStoreTimeout)
	defer vqCancel()

	ids, err := e.vectorStore.GetIDsByQuery(vqCtx, query)
	if err != nil {
		return nil, err
	}

	isCtx, isCancel := WithTightTimeout(ctx, e.configuration.TimeoutPolicy.ImageStoreTimeout)
	defer isCancel()

	imgs, err := e.imageStore.GetTattoosByID(isCtx, ids)
	if err != nil {
		return nil, err
	}

	return imgs, nil
}

func normalizeQuery(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

type Response struct {
	ImageCollections []TattooImagesCollection `json:"image_collections"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func SearchReponseErrorHelper(w http.ResponseWriter, r Response, responseCode uint) {
	w.Header().Set("Allow", http.MethodGet)
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	_ = json.NewEncoder(w).Encode(Response{ImageCollections: nil})
	return
}

func NewHandler(se *SearchEngine) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		// search is GET method only
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writeJSON(w, http.StatusMethodNotAllowed, Response{ImageCollections: nil})
			return
		}

		ctx := r.Context()

		var cancelCtx context.CancelFunc
		ctx, cancelCtx = context.WithTimeout(ctx, 300*time.Millisecond)
		defer cancelCtx()

		q := normalizeQuery(r.URL.Query().Get("q"))

		imgColl, err := se.Search(ctx, q)
		if err != nil {
			switch {
			case errors.Is(err, ErrSearchEmptyQuery):
				writeJSON(w, http.StatusBadRequest, Response{ImageCollections: nil})
				return
			case errors.Is(err, ErrImageStoreTimeout):
				writeJSON(w, http.StatusGatewayTimeout, Response{ImageCollections: nil})
				return
			case errors.Is(err, ErrImageStoreEmpty):
				writeJSON(w, http.StatusInternalServerError, Response{ImageCollections: nil})
				return
			default:
				writeJSON(w, http.StatusInternalServerError, Response{ImageCollections: nil})
				return
			}
		}

		writeJSON(w, http.StatusOK, Response{ImageCollections: imgColl})
	})

	return mux
}
