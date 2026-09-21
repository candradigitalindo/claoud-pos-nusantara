package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"
)

type apiResp struct {
	OK        bool        `json:"ok"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	Permanent bool        `json:"permanent,omitempty"`
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, err error) {
	var pe permanentError
	writeJSON(w, code, apiResp{OK: false, Error: err.Error(), Permanent: errors.As(err, &pe)})
}

func auth(token string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			writeJSON(w, http.StatusUnauthorized, apiResp{OK: false, Error: "token gateway salah"})
			return
		}
		next(w, r)
	}
}

func newRouter(g *gateway, token string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, apiResp{OK: true, Data: map[string]string{"state": g.snapshot().State}})
	})

	mux.HandleFunc("GET /status", auth(token, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, apiResp{OK: true, Data: g.snapshot()})
	}))

	mux.HandleFunc("POST /login", auth(token, func(w http.ResponseWriter, r *http.Request) {
		if err := g.startLogin(); err != nil {
			writeErr(w, 500, err)
			return
		}
		// Beri waktu server mengirim kode pertama supaya klien tidak perlu
		// polling kosong.
		deadline := time.Now().Add(6 * time.Second)
		for time.Now().Before(deadline) {
			s := g.snapshot()
			if s.QR != "" || s.LoggedIn || s.State == stateError {
				break
			}
			time.Sleep(300 * time.Millisecond)
		}
		writeJSON(w, 200, apiResp{OK: true, Data: g.snapshot()})
	}))

	mux.HandleFunc("POST /logout", auth(token, func(w http.ResponseWriter, r *http.Request) {
		if err := g.logout(); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 200, apiResp{OK: true, Data: g.snapshot()})
	}))

	mux.HandleFunc("GET /groups", auth(token, func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		list, err := g.groups(ctx)
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 200, apiResp{OK: true, Data: list})
	}))

	mux.HandleFunc("GET /check", auth(token, func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		ok, jid, err := g.check(ctx, r.URL.Query().Get("phone"))
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 200, apiResp{OK: true, Data: map[string]interface{}{"registered": ok, "jid": jid}})
	}))

	mux.HandleFunc("POST /send", auth(token, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			To       string `json:"to"`
			Message  string `json:"message"`
			ImageURL string `json:"image_url"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256*1024)).Decode(&body); err != nil {
			writeErr(w, 400, permanentError{"body JSON tidak valid"})
			return
		}
		body.Message = strings.TrimSpace(body.Message)
		if body.To == "" || (body.Message == "" && body.ImageURL == "") {
			writeErr(w, 400, permanentError{"'to' dan 'message' wajib diisi"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
		defer cancel()
		id, err := g.send(ctx, body.To, body.Message, body.ImageURL)
		if err != nil {
			var pe permanentError
			code := 502
			if errors.As(err, &pe) {
				code = 422
			}
			log.Printf("Kirim ke %s gagal: %v", body.To, err)
			writeErr(w, code, err)
			return
		}
		writeJSON(w, 200, apiResp{OK: true, Data: map[string]string{"id": id}})
	}))

	return mux
}
