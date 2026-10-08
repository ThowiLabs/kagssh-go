package config

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const kaggleSecretEndpoint = "https://www.kaggle.com/requests/GetUserSecretByLabelRequest"

type kaggleSecrets struct {
	endpoint string
	token    string
	iapToken string
	http     *http.Client
}

func newKaggleSecrets(token, iapToken string) *kaggleSecrets {
	return &kaggleSecrets{
		endpoint: kaggleSecretEndpoint,
		token:    token,
		iapToken: iapToken,
		http:     &http.Client{Timeout: 8 * time.Second},
	}
}

// Get reproduce la consulta por etiqueta de UserSecretsClient sin requerir Python.
// El JSON es el transporte interno de la API, NO un fichero de configuración.
func (s *kaggleSecrets) Get(ctx context.Context, label string) (string, bool, error) {
	payload, err := json.Marshal(struct {
		Label string `json:"Label"`
	}{label})
	if err != nil {
		return "", false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", false, errors.New("solicitud inválida al servicio de Secrets")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Kaggle-Authorization", "Bearer "+s.token)
	if s.iapToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.iapToken)
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return "", false, errors.New("no se pudo consultar el servicio Kaggle Secrets (red o timeout)")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("servicio Kaggle Secrets: HTTP %d", resp.StatusCode)
	}
	var out struct {
		WasSuccessful bool `json:"wasSuccessful"`
		Result        struct {
			Secret string `json:"secret"`
		} `json:"result"`
		Error struct {
			Code int `json:"code"`
		} `json:"error"`
		Errors []string `json:"errors"`
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, 16384))
	if err := dec.Decode(&out); err != nil {
		return "", false, errors.New("respuesta inválida de Kaggle Secrets")
	}
	if out.WasSuccessful {
		if out.Result.Secret == "" {
			return "", false, nil
		}
		return out.Result.Secret, true, nil
	}
	// La API devuelve code=5 cuando no hay un Secret con esta etiqueta
	// autorizado para el notebook. Otros errores jamás se ignoran.
	if out.Error.Code == 5 {
		for _, message := range out.Errors {
			if strings.Contains(message, "No user secrets exist") {
				return "", false, nil
			}
		}
	}
	return "", false, errors.New("el servicio Kaggle Secrets rechazó la consulta")
}
