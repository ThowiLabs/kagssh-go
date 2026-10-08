package config

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestKaggleSecrets_ProtocoloPorEtiqueta(t *testing.T) {
	tests := []struct {
		name, body  string
		status      int
		want        string
		found, fail bool
	}{
		{"encontrado", `{"wasSuccessful":true,"result":{"secret":"mi-valor"}}`, 200, "mi-valor", true, false},
		{"no existente", `{"wasSuccessful":false,"errors":["No user secrets exist for kernel id 1 and label SSH_PASSWORD."],"error":{"code":5}}`, 200, "", false, false},
		{"sin permisos", `{"wasSuccessful":false,"errors":["Access denied"],"error":{"code":7}}`, 200, "", false, true},
		{"token inválido", `{"message":"prohibido"}`, 403, "", false, true},
		{"respuesta malformada", `{"wasSuccessful":`, 200, "", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" {
					t.Error("método")
				}
				if r.Header.Get("X-Kaggle-Authorization") != "Bearer token-de-prueba" {
					t.Error("autorización")
				}
				if r.Header.Get("Authorization") != "Bearer iap-de-prueba" {
					t.Error("IAP")
				}
				if r.Header.Get("Content-Type") != "application/json" {
					t.Error("content type")
				}
				data, _ := io.ReadAll(r.Body)
				var label struct {
					Label string `json:"Label"`
				}
				if json.Unmarshal(data, &label) != nil || label.Label != "SSH_PASSWORD" {
					t.Errorf("etiqueta=%q", data)
				}
				w.WriteHeader(tt.status)
				_, _ = fmt.Fprint(w, tt.body)
			}))
			defer ts.Close()
			c := newKaggleSecrets("token-de-prueba", "iap-de-prueba")
			c.endpoint = ts.URL
			got, found, err := c.Get(context.Background(), "SSH_PASSWORD")
			if tt.fail && err == nil {
				t.Fatal("se esperaba error")
			}
			if !tt.fail && err != nil {
				t.Fatal(err)
			}
			if !tt.fail && (got != tt.want || found != tt.found) {
				t.Fatalf("valor=%q encontrado=%v", got, found)
			}
			if err != nil && (strings.Contains(err.Error(), "token-de-prueba") || strings.Contains(err.Error(), "mi-valor")) {
				t.Fatalf("se filtró información: %v", err)
			}
		})
	}
}
