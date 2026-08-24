package httpapi

import (
	"net/http"
	"soundledger/internal/application"
)

type Server struct {
	app                          *application.Service
	mux                          *http.ServeMux
	maxJSONBytes, maxUploadBytes int64
}

func New(app *application.Service) *Server {
	s := &Server{app: app, mux: http.NewServeMux(), maxJSONBytes: 1 << 20, maxUploadBytes: 64 << 20}
	s.routes()
	return s
}
func (s *Server) Handler() http.Handler { return requestMiddleware(s.mux) }
func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.Health)
	s.mux.HandleFunc("POST /api/v1/batches", s.CreateBatch)
	s.mux.HandleFunc("GET /api/v1/batches", s.ListBatches)
	s.mux.HandleFunc("GET /api/v1/batches/{batchID}", s.GetBatch)
	s.mux.HandleFunc("POST /api/v1/batches/{batchID}/clips", s.UploadClip)
	s.mux.HandleFunc("POST /api/v1/batches/{batchID}/clips/{clipID}/annotations", s.SubmitAnnotation)
	s.mux.HandleFunc("POST /api/v1/batches/{batchID}/consensus", s.ComputeConsensus)
	s.mux.HandleFunc("GET /api/v1/batches/{batchID}/disputes/{disputeID}", s.GetDisputeEvidence)
	s.mux.HandleFunc("POST /api/v1/batches/{batchID}/disputes/{disputeID}/arbitrate", s.Arbitrate)
	s.mux.HandleFunc("POST /api/v1/batches/{batchID}/freeze", s.Freeze)
	s.mux.HandleFunc("POST /api/v1/batches/{batchID}/publish", s.Publish)
	s.mux.HandleFunc("GET /api/v1/batches/{batchID}/manifest", s.GetManifest)
	s.mux.HandleFunc("GET /api/v1/batches/{batchID}/certificate", s.GetCertificate)
	s.mux.HandleFunc("GET /api/v1/batches/{batchID}/events", s.GetEvents)
}

func requestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		if requestID := r.Header.Get("X-Request-ID"); requestID != "" {
			w.Header().Set("X-Request-ID", requestID)
		}
		next.ServeHTTP(w, r)
	})
}
