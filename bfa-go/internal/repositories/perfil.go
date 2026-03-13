package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Kauakfm/ai-banking-assistant/bfa-go/internal/core/domain"
	"github.com/Kauakfm/ai-banking-assistant/bfa-go/internal/core/ports"
	"github.com/Kauakfm/ai-banking-assistant/bfa-go/internal/infra/resiliencia"
)

type perfilHTTP struct {
	baseURL     string
	client      *http.Client
	resiliencia *resiliencia.Wrapper
}

func NovoRepositorioPerfil(baseURL string, client *http.Client, res *resiliencia.Wrapper) ports.RepositorioPerfil {
	return &perfilHTTP{
		baseURL:     baseURL,
		client:      client,
		resiliencia: res,
	}
}

func (r *perfilHTTP) BuscarPerfil(ctx context.Context, clienteID string) (*domain.Perfil, error) {
	var perfil domain.Perfil

	err := r.resiliencia.Executar(ctx, "buscar_perfil", func(ctx context.Context) error {
		url := fmt.Sprintf("%s/api/v1/profile/%s", r.baseURL, clienteID)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}

		resp, err := r.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			return domain.NovoErroNaoEncontrado("perfil não encontrado")
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("profile api retornou status %d", resp.StatusCode)
		}

		return json.NewDecoder(resp.Body).Decode(&perfil)
	})

	if err != nil {
		return nil, err
	}

	return &perfil, nil
}
