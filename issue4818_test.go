package gin

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestIssue4818_skippedNodesOverflow_Panic reproduces the panic from issue #4818:
// with HandleMethodNotAllowed enabled, getValue is called once per method tree
// reusing the same c.skippedNodes stack without resetting it, so residue
// accumulates until the reslice exceeds the capacity (engine.maxSections).
// The request must not panic and must fall through to the normal 404 path
// (the path genuinely matches no route, so 405 is not applicable here).
func TestIssue4818_skippedNodesOverflow_Panic(t *testing.T) {
	router := New()
	router.HandleMethodNotAllowed = true

	h := func(c *Context) {}
	router.OPTIONS("/:p0/:p1/a/:p2", h)
	router.GET("/:p0/:p1/a/:p2", h)
	router.PATCH("/b/:p0/:p1/c", h)
	router.DELETE("/b/:p0/:p1/d/:p3", h)
	router.GET("/b/:p0/:p1/e/f", h)
	router.POST("/b/:p0/:p1/g/:p4/h", h)
	router.OPTIONS("/b/:p0/:p1/g/:p4/h", h)
	router.DELETE("/b/cache", h)
	router.GET("/b/clients/:p1/g", h)
	router.POST("/b/clients/:p1/g", h)
	router.PATCH("/b/clients/:p1/g/:p4", h)
	router.OPTIONS("/b/clients/:p1/g/:p4", h)

	req := httptest.NewRequest(http.MethodPost, "/b/clients/42", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Must not panic; path matches no route, so 404 is the correct outcome.
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (a panic would have aborted the test)", w.Code)
	}
}

// TestIssue4818_MethodNotAllowedStillWorks verifies the HandleMethodNotAllowed
// loop still produces a correct 405 with an Allow header after the fix, i.e.
// the reset does not break the loop across multiple method trees.
func TestIssue4818_MethodNotAllowedStillWorks(t *testing.T) {
	router := New()
	router.HandleMethodNotAllowed = true

	h := func(c *Context) {}
	router.OPTIONS("/:p0/:p1/a/:p2", h)
	router.GET("/:p0/:p1/a/:p2", h)
	router.PATCH("/b/:p0/:p1/c", h)
	router.DELETE("/b/:p0/:p1/d/:p3", h)
	router.GET("/b/:p0/:p1/e/f", h)
	router.POST("/b/:p0/:p1/g/:p4/h", h)
	router.OPTIONS("/b/:p0/:p1/g/:p4/h", h)
	router.DELETE("/b/cache", h)
	router.GET("/b/clients/:p1/g", h)
	router.POST("/b/clients/:p1/g", h)
	router.PATCH("/b/clients/:p1/g/:p4", h)
	router.OPTIONS("/b/clients/:p1/g/:p4", h)

	// PATCH /b/clients/42/g is registered under GET but not under PATCH
	// -> HandleMethodNotAllowed should yield 405 with an Allow header.
	req := httptest.NewRequest(http.MethodPatch, "/b/clients/42/g", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
	allow := w.Header().Get("Allow")
	if allow == "" {
		t.Fatal("expected Allow header to be set for 405")
	}
}

// TestIssue4818_MethodNotAllowedManyTrees stresses the loop across many
// method trees to make sure the skipped-nodes accumulator can never overflow.
func TestIssue4818_MethodNotAllowedManyTrees(t *testing.T) {
	router := New()
	router.HandleMethodNotAllowed = true

	h := func(c *Context) {}
	for _, m := range []string{
		http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete,
		http.MethodPatch, http.MethodOptions, http.MethodHead, http.MethodConnect,
		http.MethodTrace,
	} {
		router.Handle(m, "/:p0/:p1/a/:p2", h)
		router.Handle(m, "/s/:p0/:p1/:p2/:p3/:p4/:p5/d", h)
	}
	// GET route so that a POST to a GET-only path yields 405
	router.GET("/only-get", h)

	req := httptest.NewRequest(http.MethodPost, "/only-get", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}
