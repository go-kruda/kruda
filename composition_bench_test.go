//go:build linux || darwin

package kruda

import "testing"

func BenchmarkCompositionPipeline(b *testing.B) {
	for _, fixture := range compositionFixtures() {
		b.Run(fixture.name, func(b *testing.B) {
			app := compositionApp(b, fixture, nil, compositionTypedValidationExpected)
			req := compositionRequest(b, "GET", "/users/42?"+fixture.query, "")
			response := newMockResponse()
			app.ServeKruda(response, req)
			assertCompositionResponse(b, response, compositionExpectedJSON(fixture.groups))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				response.statusCode = 0
				response.body = response.body[:0]
				clear(response.headers.h)
				app.ServeKruda(response, req)
			}
		})
	}
}

func BenchmarkCompositionJSONPost(b *testing.B) {
	app := compositionPostApp()
	req := compositionRequest(b, "POST", "/items", `{"name":" alice ","count":3}`)
	response := newMockResponse()
	app.ServeKruda(response, req)
	assertCompositionResponse(b, response, `{"name":"alice","count":3}`)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		response.statusCode = 0
		response.body = response.body[:0]
		clear(response.headers.h)
		app.ServeKruda(response, req)
	}
}
