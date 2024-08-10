package server

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/khoakmp/judgo/pkg/broker"
	"github.com/khoakmp/judgo/pkg/testcase"
)

type Server struct {
	router   *mux.Router
	testcase testcase.TestcaseManager
	broker   broker.Broker
}

func NewServer() *Server {
	r := mux.NewRouter()
	s := &Server{
		router: r,
	}
	privateRouter := r.PathPrefix("/private").Subrouter()

	privateRouter.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	})
	privateRouter.HandleFunc("/submission", s.handleCreateSubmission).Methods(http.MethodPost)

	return s
}
