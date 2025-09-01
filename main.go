package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

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

// TattooStore defines the contract.
// Of the service which stores the tattoos.
type TattooStore interface {
	GetTattoosByID([]string) ([]TattooImagesCollection, error)
}

// VectorStore defines the contract.
// Of the service which stores the embedded
// data about the tattoos.
type VectorStore interface {
	GetIDsByQuery(string) ([]string, error)
}

// SearchEngine peforms searching for tattoos.
type SearchEngine struct {
	tattooStore TattooStore
	vectorStore VectorStore
}

// NewSearchEngine creates a new search engine instance
func NewSearchEngine(ts TattooStore, vs VectorStore) *SearchEngine {
	return &SearchEngine{
		tattooStore: ts,
		vectorStore: vs,
	}
}

// Search returns a list of tattoos by their query match rating.
// And all the images which are related to it.
func (e *SearchEngine) Search(q string) ([]TattooImagesCollection, error) {
	q = normalizeQuery(q)
	ids, err := e.vectorStore.GetIDsByQuery(q)
	if err != nil {
		return nil, err
	}

	imgs, err := e.tattooStore.GetTattoosByID(ids)
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

func NewHandler(se *SearchEngine) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}

		q := normalizeQuery(r.URL.Query().Get("q"))
		if q == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(Response{ImageCollections: nil})

			return
		}

		imgColl, _ := se.Search(q)
		payload := Response{ImageCollections: imgColl}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(payload)
	})

	return mux
}
