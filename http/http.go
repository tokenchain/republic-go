package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/republicprotocol/republic-go/http/adapter"
)

const reset = "\x1b[0m"

// OpenOrderRequest is an JSON object sent to the HTTP handlers to request the
// opening of an order.
type OpenOrderRequest struct {
	Signature            string                       `json:"signature"`
	OrderFragmentMapping adapter.OrderFragmentMapping `json:"orderFragmentMapping"`
}

// RecoveryHandler handles errors while processing the requests and populates the errors in the response
func RecoveryHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if r := recover(); r != nil {
				writeError(w, http.StatusInternalServerError, fmt.Sprintf("%v", r))
			}
		}()
		h.ServeHTTP(w, r)
	})
}

func ListenAndServe(bind, port string, openOrderAdapter adapter.OpenOrderAdapter, cancelOrderAdapter adapter.CancelOrderAdapter) error {
	r := mux.NewRouter().StrictSlash(true)
	r.HandleFunc("/orders", OpenOrderHandler(openOrderAdapter)).Methods("POST")
	r.HandleFunc("/orders/{id}", CancelOrderHandler(cancelOrderAdapter)).Methods("DELETE")
	r.Use(RecoveryHandler)
	return http.ListenAndServe(fmt.Sprintf("%v:%v", bind, port), r)
}

// OpenOrderHandler handles all HTTP open order requests
func OpenOrderHandler(adapter adapter.OpenOrderAdapter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		openOrderRequest := OpenOrderRequest{}
		if err := json.NewDecoder(r.Body).Decode(&openOrderRequest); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("cannot decode json into an order or a list of order fragments: %v", err))
			return
		}
		if err := adapter.OpenOrder(openOrderRequest.Signature, openOrderRequest.OrderFragmentMapping); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("cannot open order: %v", err))
			return
		}
		w.WriteHeader(http.StatusCreated)
	}
}

// CancelOrderHandler handles HTTP Delete Requests
func CancelOrderHandler(adapter adapter.CancelOrderAdapter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		paths := strings.Split(r.URL.Path, "/")
		if len(paths) < 2 || paths[len(paths)-2] != "orders" || paths[len(paths)-1] == "" {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("cannot cancel order: nil id"))
			return
		}
		orderID := paths[len(paths)-1]
		signature := r.URL.Query().Get("signature")
		if signature == "" {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("cannot cancel order: nil signature"))
			return
		}
		if err := adapter.CancelOrder(signature, orderID); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("cannot cancel order: %v", err))
			return
		}
		w.WriteHeader(http.StatusGone)
	}
}

func writeError(w http.ResponseWriter, statusCode int, err string) {
	w.WriteHeader(statusCode)
	w.Write([]byte(err))
	return
}
