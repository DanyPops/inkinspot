package main_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"

	searchAPI "github.com/DanyPops/inkinspot"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type HTTPResult struct {
	Status int
	Body   []byte
	JSON   searchAPI.Response
}

func doQuery(se *httptest.Server, query string) HTTPResult {
	GinkgoHelper()
	resp, err := http.Get(se.URL + "/search?q=" + url.QueryEscape(query))
	Expect(err).NotTo(HaveOccurred())
	defer resp.Body.Close()

	var res HTTPResult
	res.Status = resp.StatusCode
	b, err := io.ReadAll(resp.Body)
	Expect(err).NotTo(HaveOccurred())
	res.Body = b
	_ = json.Unmarshal(b, &res.JSON)

	return res
}

func doRequest(se *httptest.Server, method, query string, body io.Reader) *http.Response {
	GinkgoHelper()
	req, err := http.NewRequest(method, se.URL+"/search?q="+url.QueryEscape(query), body)
	Expect(err).NotTo(HaveOccurred())
	resp, err := se.Client().Do(req)
	Expect(err).NotTo(HaveOccurred())

	return resp
}


func initSearchEngineHttpServer(ts searchAPI.TattooStore, vs searchAPI.VectorStore) *httptest.Server {
	GinkgoHelper()
	eng := searchAPI.NewSearchEngine(ts, vs)
	srv := httptest.NewServer(searchAPI.NewHandler(eng))

	return srv
}

type fakeTattooStore []searchAPI.TattooImagesCollection

func (ts fakeTattooStore) GetTattoosByID(ids []string) ([]searchAPI.TattooImagesCollection, error) {
	return ts, nil
}

func (ts *fakeTattooStore) AddCollection(c searchAPI.TattooImagesCollection) error {
	*ts = append(*ts, c)
	return nil
}

type fakeVectorStore struct{}

func (vs *fakeVectorStore) GetIDsByQuery(q string) ([]string, error) {
	return []string{"id"}, nil
}

func (vs *fakeVectorStore) AddVector(v searchAPI.TattooImagesVector) error {
	return nil
}

type testCaseTattoos struct {
	collection searchAPI.TattooImagesCollection
	vector     searchAPI.TattooImagesVector
}

var testCases = []testCaseTattoos{
	{
		searchAPI.TattooImagesCollection{
			ID:   "X",
			URLs: []string{"lion_realistic_bw_chest.jpg"},
		},
		searchAPI.TattooImagesVector{
			ID:      "X",
			Style:   searchAPI.LabelSet{"realistic": 100, "bw": 100},
			Subject: searchAPI.LabelSet{"lion": 100},
			Area:    searchAPI.LabelSet{"chest": 100},
		},
	},
	{
		searchAPI.TattooImagesCollection{
			ID:   "Y",
			URLs: []string{"lion_neotrad_color_arm.jpg"},
		},
		searchAPI.TattooImagesVector{
			ID:      "Y",
			Style:   searchAPI.LabelSet{"neotrad": 100, "color": 100},
			Subject: searchAPI.LabelSet{"lion": 100},
			Area:    searchAPI.LabelSet{"arm": 100},
		},
	},
	{
		searchAPI.TattooImagesCollection{
			ID:   "Z",
			URLs: []string{"tiger_abstract_bw_chest.jpg"},
		},
		searchAPI.TattooImagesVector{
			ID:      "Z",
			Style:   searchAPI.LabelSet{"abstract": 100, "bw": 100},
			Subject: searchAPI.LabelSet{"tiger": 100},
			Area:    searchAPI.LabelSet{"chest": 100},
		},
	},
}

var _ = Describe("Search API", func() {

	Describe("Making a search query for tattoo images", func() {
		var se *httptest.Server

		BeforeEach(func() {
			se = initSearchEngineHttpServer(&fakeTattooStore{}, &fakeVectorStore{})
		})

		AfterEach(func() {
			se.Close()
		})

		Context("Client Side Issues", func() {
			When("Query is empty", func() {
				It("returns a 400 Bad Request", func() {
					res := doQuery(se, "")
					Expect(res.Status).To(Equal(http.StatusBadRequest))
					ic := res.JSON.ImageCollections
					Expect(ic).To(HaveLen(0),
						"must have zero image collections, got: %s", ic)
				})
			})

			When("Query is is not using GET method", func() {
				It("returns a 405 Method Not Allowed", func() {
					resp := doRequest(se, http.MethodPost, "NOTGOT", nil)
					defer resp.Body.Close()
					statusCode := resp.StatusCode
					Expect(statusCode).Should(Equal(http.StatusMethodNotAllowed))
					allowHeader := resp.Header.Get("Allow")
					Expect(allowHeader).To(ContainSubstring(http.MethodGet))
				})
			})
		})

		Context("Image Store Issues", func() {
			When("Image store is empty", func() {
				q := "empty tattoo store"

				It("returns a 500 Internal Server Error", func() {
					resp := doQuery(se, q)
					ic := resp.JSON.ImageCollections
					Expect(ic).To(HaveLen(0),
						"must have zero image collections, got: %s", ic)
				})
			})

			When("Image store is timing out", func() {
				It("returns a 408 Request Timeout", func() {

				})
			})
		})

		Context("Image Store loaded with big cat tattoos", func() {
			When("query is realistic black & white lion", func() {
				queryVerbose := "realistic black and white lion on chest"
				queryTerse := "realistic black white lion chest"
				queryCAPS := "REALISTIC BLACK WHITE LION CHEST"

				It("returns a ranked list with the same results for all quries", func() {
					resultVerbose := doQuery(se, queryVerbose)
					resultTerse := doQuery(se, queryTerse)
					respultCaps := doQuery(se, queryCAPS)
					for index, verboseCollection := range resultVerbose.JSON.ImageCollections {
						terseCollection := resultTerse.JSON.ImageCollections[index]
						Expect(verboseCollection).To(Equal(terseCollection), "Expected verbose and terse query must have the same results")

						capsCollection := respultCaps.JSON.ImageCollections[index]
						Expect(verboseCollection).To(Equal(capsCollection), "Verbose and caps query must have the same results")
					}
				})

			})
		})

	})
})
